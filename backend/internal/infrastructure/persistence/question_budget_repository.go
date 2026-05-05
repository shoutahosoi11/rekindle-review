package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type questionBudgetRepository struct {
	db *sql.DB
}

func NewQuestionBudgetRepository(db *sql.DB) domain.QuestionBudgetRepository {
	return &questionBudgetRepository{db: db}
}

func (r *questionBudgetRepository) GetBalance(ctx context.Context, userID uuid.UUID, plan string, now time.Time) (*domain.QuestionTokenBalance, error) {
	if err := r.ensureDailyBudget(ctx, userID, now); err != nil {
		return nil, err
	}
	return r.readBalance(ctx, userID, plan, now)
}

func (r *questionBudgetRepository) AwardAdTokens(ctx context.Context, userID uuid.UUID, now time.Time) (*domain.QuestionTokenBalance, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("question budget repo: begin award: %w", err)
	}
	defer tx.Rollback()

	budgetDate := questionBudgetDate(now)
	result, err := tx.ExecContext(ctx, `
INSERT INTO question_daily_budgets (user_id, budget_date, ad_views_today)
VALUES ($1, $2, 1)
ON CONFLICT (user_id, budget_date) DO UPDATE
SET ad_views_today = question_daily_budgets.ad_views_today + 1
WHERE question_daily_budgets.ad_views_today < $3
`, userID, budgetDate, domain.MaxAdViewsPerDay)
	if err != nil {
		return nil, fmt.Errorf("question budget repo: increment ad view: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("question budget repo: award rows affected: %w", err)
	}
	if affected == 0 {
		return nil, domain.ErrQuestionBudgetExceeded
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_ad_tokens (user_id, token_count)
VALUES ($1, $2)
`, userID, domain.AdTokensPerView); err != nil {
		return nil, fmt.Errorf("question budget repo: insert ad token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("question budget repo: commit award: %w", err)
	}
	return r.readBalance(ctx, userID, "free", now)
}

func (r *questionBudgetRepository) ReserveQuestions(ctx context.Context, userID uuid.UUID, plan string, questionCount int, now time.Time) (*domain.QuestionTokenBalance, error) {
	if questionCount <= 0 {
		return r.GetBalance(ctx, userID, plan, now)
	}
	if plan == "premium" {
		return r.GetBalance(ctx, userID, plan, now)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("question budget repo: begin reserve: %w", err)
	}
	defer tx.Rollback()

	budgetDate := questionBudgetDate(now)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO question_daily_budgets (user_id, budget_date)
VALUES ($1, $2)
ON CONFLICT (user_id, budget_date) DO NOTHING
`, userID, budgetDate); err != nil {
		return nil, fmt.Errorf("question budget repo: ensure reserve budget: %w", err)
	}

	var freeUsed int
	if err := tx.QueryRowContext(ctx, `
SELECT free_used
FROM question_daily_budgets
WHERE user_id = $1 AND budget_date = $2
FOR UPDATE
`, userID, budgetDate).Scan(&freeUsed); err != nil {
		return nil, fmt.Errorf("question budget repo: lock reserve budget: %w", err)
	}

	freeRemaining := domain.FreeDailyQuestionLimit - freeUsed
	if freeRemaining < 0 {
		freeRemaining = 0
	}
	freeToUse := questionCount
	if freeToUse > freeRemaining {
		freeToUse = freeRemaining
	}
	tokenNeeded := questionCount - freeToUse

	if tokenNeeded > 0 {
		available, err := r.availableTokensTx(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		if available < tokenNeeded {
			return nil, domain.ErrQuestionBudgetExceeded
		}
		if err := r.consumeTokensTx(ctx, tx, userID, tokenNeeded); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE question_daily_budgets
SET free_used = free_used + $3,
    token_used = token_used + $4
WHERE user_id = $1 AND budget_date = $2
`, userID, budgetDate, freeToUse, tokenNeeded); err != nil {
		return nil, fmt.Errorf("question budget repo: update reserve budget: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("question budget repo: commit reserve: %w", err)
	}
	return r.readBalance(ctx, userID, plan, now)
}

func (r *questionBudgetRepository) ensureDailyBudget(ctx context.Context, userID uuid.UUID, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO question_daily_budgets (user_id, budget_date)
VALUES ($1, $2)
ON CONFLICT (user_id, budget_date) DO NOTHING
`, userID, questionBudgetDate(now))
	if err != nil {
		return fmt.Errorf("question budget repo: ensure daily budget: %w", err)
	}
	return nil
}

func (r *questionBudgetRepository) readBalance(ctx context.Context, userID uuid.UUID, plan string, now time.Time) (*domain.QuestionTokenBalance, error) {
	var balance domain.QuestionTokenBalance
	err := r.db.QueryRowContext(ctx, `
WITH token_balance AS (
    SELECT COALESCE(SUM(token_count), 0)::int AS available_tokens
    FROM user_ad_tokens
    WHERE user_id = $1 AND used_at IS NULL
)
SELECT
    tb.available_tokens,
    b.free_used,
    b.ad_views_today
FROM question_daily_budgets b
CROSS JOIN token_balance tb
WHERE b.user_id = $1
  AND b.budget_date = $2
`, userID, questionBudgetDate(now)).Scan(&balance.AvailableTokens, &balance.FreeUsedToday, &balance.AdViewsToday)
	if err != nil {
		return nil, fmt.Errorf("question budget repo: read balance: %w", err)
	}
	balance.FreeLimit = domain.FreeDailyQuestionLimit
	balance.AdViewsLimit = domain.MaxAdViewsPerDay
	balance.Plan = plan
	return &balance, nil
}

func (r *questionBudgetRepository) availableTokensTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (int, error) {
	var available int
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(token_count), 0)::int
FROM user_ad_tokens
WHERE user_id = $1 AND used_at IS NULL
`, userID).Scan(&available); err != nil {
		return 0, fmt.Errorf("question budget repo: available tokens: %w", err)
	}
	return available, nil
}

func (r *questionBudgetRepository) consumeTokensTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID, needed int) error {
	rows, err := tx.QueryContext(ctx, `
SELECT id, token_count
FROM user_ad_tokens
WHERE user_id = $1 AND used_at IS NULL
ORDER BY earned_at ASC
FOR UPDATE
`, userID)
	if err != nil {
		return fmt.Errorf("question budget repo: list tokens: %w", err)
	}
	defer rows.Close()

	remaining := needed
	for rows.Next() && remaining > 0 {
		var id uuid.UUID
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return fmt.Errorf("question budget repo: scan token: %w", err)
		}
		if count <= remaining {
			if _, err := tx.ExecContext(ctx, `UPDATE user_ad_tokens SET used_at = NOW() WHERE id = $1`, id); err != nil {
				return fmt.Errorf("question budget repo: mark token used: %w", err)
			}
			remaining -= count
			continue
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE user_ad_tokens SET token_count = token_count - $2 WHERE id = $1
`, id, remaining); err != nil {
			return fmt.Errorf("question budget repo: split token: %w", err)
		}
		remaining = 0
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("question budget repo: rows tokens: %w", err)
	}
	if remaining > 0 {
		return domain.ErrQuestionBudgetExceeded
	}
	return nil
}

func questionBudgetDate(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.FixedZone("JST", 9*60*60)
	}
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}
