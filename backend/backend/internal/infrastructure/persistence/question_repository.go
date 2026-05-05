package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/repository/sqlcgen"
	"github.com/sqlc-dev/pqtype"
)

type questionRepository struct {
	db      *sql.DB
	queries *sqlcgen.Queries
}

func NewQuestionRepository(db *sql.DB) domain.QuestionRepository {
	return &questionRepository{
		db:      db,
		queries: sqlcgen.New(db),
	}
}

func (r *questionRepository) Save(ctx context.Context, q *domain.Question, meta *domain.QuestionMeta) error {
	optionsJSON, err := json.Marshal(q.Options)
	if err != nil {
		return fmt.Errorf("question repo: marshal options: %w", err)
	}

	questionID, err := uuid.Parse(q.ID)
	if err != nil {
		return fmt.Errorf("question repo: parse question id: %w", err)
	}

	creatorID, err := uuid.Parse(meta.CreatorID)
	if err != nil {
		return fmt.Errorf("question repo: parse creator id: %w", err)
	}

	query := `
INSERT INTO questions (
    id,
    user_id,
    source_type,
    question_type,
    body,
    options,
    correct_answer,
    explanation,
    is_ai_generated,
    generation_id,
    highlight_id,
    perspective,
    version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)`

	if _, err := r.db.ExecContext(ctx, query,
		questionID,
		creatorID,
		string(meta.SourceType),
		string(q.QuestionType),
		q.Content,
		pqtype.NullRawMessage{RawMessage: optionsJSON, Valid: true},
		q.CorrectAnswer,
		sql.NullString{String: q.Explanation, Valid: strings.TrimSpace(q.Explanation) != ""},
		meta.IsAIGenerated,
		parseOptionalUUID(meta.GenerationID),
		parseOptionalUUID(meta.HighlightID),
		normalizePerspective(meta.Perspective),
		normalizeQuestionVersion(meta.Version),
	); err != nil {
		return fmt.Errorf("question repo: save: %w", err)
	}

	return nil
}

func (r *questionRepository) SupersedeActiveQuestionsForHighlight(ctx context.Context, userID uuid.UUID, highlightID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE questions
SET superseded_at = NOW(),
    updated_at = NOW()
WHERE user_id = $1
  AND highlight_id = $2
  AND superseded_at IS NULL
`, userID, highlightID)
	if err != nil {
		return fmt.Errorf("question repo: supersede active questions for highlight: %w", err)
	}
	return nil
}

func (r *questionRepository) ListByCreatorID(ctx context.Context, creatorID string, limit int) ([]*domain.Question, error) {
	userID, err := uuid.Parse(creatorID)
	if err != nil {
		return nil, fmt.Errorf("question repo: parse creator id: %w", err)
	}

	rows, err := r.queries.ListQuestionsByCreatorID(ctx, sqlcgen.ListQuestionsByCreatorIDParams{
		UserID: userID,
		Limit:  normalizeQuestionLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("question repo: list by creator id: %w", err)
	}

	questions := make([]*domain.Question, 0, len(rows))
	for _, row := range rows {
		questions = append(questions, &domain.Question{
			ID:            row.ID.String(),
			QuestionType:  domain.QuestionType(row.QuestionType),
			Content:       row.Body,
			Options:       decodeQuestionOptionsMessage(row.Options),
			CorrectAnswer: row.CorrectAnswer,
			Explanation:   nullStringOrEmpty(row.Explanation),
		})
	}

	return questions, nil
}

func (r *questionRepository) ListSavedByUserID(ctx context.Context, userID string, limit int) ([]*domain.SavedQuestion, error) {
	if limit <= 0 {
		limit = 50
	}

	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("question repo: parse user id for saved list: %w", err)
	}

	query := `
SELECT q.id, q.question_type, q.body, q.options, q.correct_answer, q.explanation, sq.note, sq.updated_at
FROM saved_questions sq
JOIN questions q ON q.id = sq.question_id
WHERE sq.user_id = $1
ORDER BY sq.updated_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, uID, limit)
	if err != nil {
		return nil, fmt.Errorf("question repo: list saved by user id: %w", err)
	}
	defer rows.Close()

	savedQuestions := make([]*domain.SavedQuestion, 0)
	for rows.Next() {
		var (
			qID           uuid.UUID
			questionType  string
			body          string
			optionsJSON   []byte
			correctAnswer string
			explanation   string
			note          sql.NullString
			savedAt       sql.NullTime
		)

		if err := rows.Scan(&qID, &questionType, &body, &optionsJSON, &correctAnswer, &explanation, &note, &savedAt); err != nil {
			return nil, fmt.Errorf("question repo: scan saved list: %w", err)
		}

		savedQuestions = append(savedQuestions, &domain.SavedQuestion{
			Question: domain.Question{
				ID:            qID.String(),
				QuestionType:  domain.QuestionType(questionType),
				Content:       body,
				Options:       decodeQuestionOptionsBytes(optionsJSON),
				CorrectAnswer: correctAnswer,
				Explanation:   explanation,
			},
			Note:    note.String,
			SavedAt: savedAt.Time,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("question repo: rows saved list: %w", err)
	}

	return savedQuestions, nil
}

func (r *questionRepository) ListIncorrectByUserID(ctx context.Context, userID string, limit int) ([]*domain.IncorrectQuestion, error) {
	if limit <= 0 {
		limit = 50
	}

	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("question repo: parse user id for incorrect list: %w", err)
	}

	query := `
SELECT q.id, q.question_type, q.body, q.options, q.correct_answer, q.explanation,
       sq.note, COALESCE(a.updated_at, a.created_at)
FROM answers a
JOIN questions q ON q.id = a.question_id
LEFT JOIN saved_questions sq ON sq.user_id = a.user_id AND sq.question_id = a.question_id
WHERE a.user_id = $1
  AND a.is_correct = FALSE
ORDER BY COALESCE(a.updated_at, a.created_at) DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, uID, limit)
	if err != nil {
		return nil, fmt.Errorf("question repo: list incorrect by user id: %w", err)
	}
	defer rows.Close()

	incorrectQuestions := make([]*domain.IncorrectQuestion, 0)
	for rows.Next() {
		var (
			qID           uuid.UUID
			questionType  string
			body          string
			optionsJSON   []byte
			correctAnswer string
			explanation   string
			note          sql.NullString
			answeredAt    sql.NullTime
		)

		if err := rows.Scan(&qID, &questionType, &body, &optionsJSON, &correctAnswer, &explanation, &note, &answeredAt); err != nil {
			return nil, fmt.Errorf("question repo: scan incorrect list: %w", err)
		}

		incorrectQuestions = append(incorrectQuestions, &domain.IncorrectQuestion{
			Question: domain.Question{
				ID:            qID.String(),
				QuestionType:  domain.QuestionType(questionType),
				Content:       body,
				Options:       decodeQuestionOptionsBytes(optionsJSON),
				CorrectAnswer: correctAnswer,
				Explanation:   explanation,
			},
			Note:       note.String,
			AnsweredAt: answeredAt.Time,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("question repo: rows incorrect list: %w", err)
	}

	return incorrectQuestions, nil
}

func (r *questionRepository) ListPreparedByUserIDAndHighlightIDs(ctx context.Context, userID string, highlightIDs []uuid.UUID, limit int) ([]*domain.Question, error) {
	if len(highlightIDs) == 0 {
		return make([]*domain.Question, 0), nil
	}
	if limit <= 0 {
		limit = 20
	}

	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("question repo: parse user id for prepared questions: %w", err)
	}

	query := `
SELECT q.id,
       q.question_type,
       q.body,
       q.options,
       q.correct_answer,
       q.explanation
FROM questions q
LEFT JOIN answers a
  ON a.question_id = q.id
 AND a.user_id = $1
WHERE q.user_id = $1
  AND q.superseded_at IS NULL
  AND q.highlight_id::text = ANY($2)
ORDER BY CASE WHEN a.question_id IS NULL THEN 0 ELSE 1 END ASC, q.created_at DESC
LIMIT $3`

	rows, err := r.db.QueryContext(ctx, query, uID, pq.Array(uuidTextSlice(highlightIDs)), limit)
	if err != nil {
		return nil, fmt.Errorf("question repo: list prepared by highlight ids: %w", err)
	}
	defer rows.Close()

	questions := make([]*domain.Question, 0)
	for rows.Next() {
		var (
			qID           uuid.UUID
			questionType  string
			body          string
			optionsJSON   []byte
			correctAnswer string
			explanation   sql.NullString
		)

		if err := rows.Scan(&qID, &questionType, &body, &optionsJSON, &correctAnswer, &explanation); err != nil {
			return nil, fmt.Errorf("question repo: scan prepared question: %w", err)
		}

		questions = append(questions, &domain.Question{
			ID:            qID.String(),
			QuestionType:  domain.QuestionType(questionType),
			Content:       body,
			Options:       decodeQuestionOptionsBytes(optionsJSON),
			CorrectAnswer: correctAnswer,
			Explanation:   nullStringOrEmpty(explanation),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("question repo: rows prepared question: %w", err)
	}

	return questions, nil
}

func (r *questionRepository) ListPerspectivesByHighlightID(ctx context.Context, userID string, highlightID uuid.UUID) ([]string, error) {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("question repo: parse user id for perspectives: %w", err)
	}

	query := `
SELECT perspective
FROM questions
WHERE user_id = $1
  AND highlight_id = $2
  AND perspective <> ''
ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, uID, highlightID)
	if err != nil {
		return nil, fmt.Errorf("question repo: list perspectives: %w", err)
	}
	defer rows.Close()

	perspectives := make([]string, 0)
	for rows.Next() {
		var perspective sql.NullString
		if err := rows.Scan(&perspective); err != nil {
			return nil, fmt.Errorf("question repo: scan perspective: %w", err)
		}
		if perspective.Valid && strings.TrimSpace(perspective.String) != "" {
			perspectives = append(perspectives, strings.TrimSpace(perspective.String))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("question repo: rows perspective: %w", err)
	}

	return perspectives, nil
}

func (r *questionRepository) ListUsedHighlightIDsByUserID(ctx context.Context, userID string, highlightIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(highlightIDs) == 0 {
		return make([]uuid.UUID, 0), nil
	}

	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("question repo: parse user id for used highlights: %w", err)
	}

	highlightIDStrings := make([]string, 0, len(highlightIDs))
	for _, highlightID := range highlightIDs {
		highlightIDStrings = append(highlightIDStrings, highlightID.String())
	}

	query := `
SELECT DISTINCT highlight_id
FROM questions
WHERE user_id = $1
  AND highlight_id IS NOT NULL
  AND highlight_id::text = ANY($2)`

	rows, err := r.db.QueryContext(ctx, query, uID, pq.Array(highlightIDStrings))
	if err != nil {
		return nil, fmt.Errorf("question repo: list used highlight ids: %w", err)
	}
	defer rows.Close()

	usedHighlightIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var highlightID uuid.UUID
		if err := rows.Scan(&highlightID); err != nil {
			return nil, fmt.Errorf("question repo: scan used highlight id: %w", err)
		}
		usedHighlightIDs = append(usedHighlightIDs, highlightID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("question repo: rows used highlight ids: %w", err)
	}

	return usedHighlightIDs, nil
}

func (r *questionRepository) FindByID(ctx context.Context, id string) (*domain.Question, *domain.QuestionMeta, *domain.QuestionStats, error) {
	questionID, err := uuid.Parse(id)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("question repo: parse question id: %w", err)
	}

	row, err := r.queries.FindQuestionByID(ctx, questionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil, domain.ErrNotFound
		}
		return nil, nil, nil, fmt.Errorf("question repo: find by id: %w", err)
	}

	q := &domain.Question{
		ID:            row.ID.String(),
		QuestionType:  domain.QuestionType(row.QuestionType),
		Content:       row.Body,
		Options:       decodeQuestionOptionsMessage(row.Options),
		CorrectAnswer: row.CorrectAnswer,
		Explanation:   nullStringOrEmpty(row.Explanation),
	}
	meta := &domain.QuestionMeta{
		QuestionID:    row.ID.String(),
		CreatorID:     row.UserID.String(),
		SourceType:    domain.SourceType(row.SourceType),
		HighlightID:   nullUUIDString(row.HighlightID),
		GenerationID:  nullUUIDString(row.GenerationID),
		IsAIGenerated: row.IsAiGenerated,
	}
	stats := &domain.QuestionStats{
		QuestionID:   row.ID.String(),
		AnswerCount:  int(row.AnswerCount),
		CorrectCount: int(row.CorrectCount),
	}

	return q, meta, stats, nil
}

func (r *questionRepository) GetByID(ctx context.Context, id string) (*domain.Question, error) {
	questionID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("question repo: parse question id: %w", err)
	}

	row, err := r.queries.GetQuestionByID(ctx, questionID)
	if err != nil {
		return nil, wrapQuestionError("get by id", err)
	}

	return &domain.Question{
		ID:            row.ID.String(),
		QuestionType:  domain.QuestionType(row.QuestionType),
		Content:       row.Body,
		Options:       decodeQuestionOptionsMessage(row.Options),
		CorrectAnswer: row.CorrectAnswer,
		Explanation:   nullStringOrEmpty(row.Explanation),
	}, nil
}

func (r *questionRepository) UpdateStats(ctx context.Context, questionID string, isCorrect bool) error {
	id, err := uuid.Parse(questionID)
	if err != nil {
		return fmt.Errorf("question repo: parse question id for update stats: %w", err)
	}

	if err := r.queries.UpdateQuestionStats(ctx, sqlcgen.UpdateQuestionStatsParams{
		ID:        id,
		IsCorrect: isCorrect,
	}); err != nil {
		return fmt.Errorf("question repo: update stats: %w", err)
	}

	return nil
}

func (r *questionRepository) SaveGeneration(ctx context.Context, userID, sourceType, sourceID, promptUsed, modelUsed string) (string, error) {
	query := `
INSERT INTO question_generations (user_id, source_type, source_id, prompt_used, model_used)
VALUES ($1, $2, $3, $4, $5)
RETURNING id`

	uID, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("question repo: parse user id: %w", err)
	}

	var sID *uuid.UUID
	if sourceID != "" {
		id, err := uuid.Parse(sourceID)
		if err == nil {
			sID = &id
		}
	}

	var genID uuid.UUID
	err = r.db.QueryRowContext(ctx, query, uID, sourceType, sID, promptUsed, modelUsed).Scan(&genID)
	if err != nil {
		return "", fmt.Errorf("question repo: save generation: %w", err)
	}
	return genID.String(), nil
}

func (r *questionRepository) SaveForUser(ctx context.Context, userID, questionID, note string) error {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("question repo: parse user id for save: %w", err)
	}

	qID, err := uuid.Parse(questionID)
	if err != nil {
		return fmt.Errorf("question repo: parse question id for save: %w", err)
	}

	if err := r.queries.SaveQuestionForUser(ctx, sqlcgen.SaveQuestionForUserParams{
		UserID:     uID,
		QuestionID: qID,
		Note:       sql.NullString{String: note, Valid: true},
	}); err != nil {
		return fmt.Errorf("question repo: save for user: %w", err)
	}

	return nil
}

func (r *questionRepository) GetDailyGeneratedCount(ctx context.Context, userID uuid.UUID, day time.Time) (int, error) {
	query := `
SELECT count
FROM user_daily_generation_counts
WHERE user_id = $1
  AND date = $2`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID, day.Format("2006-01-02")).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("question repo: get daily generated count: %w", err)
	}

	return count, nil
}

func (r *questionRepository) ReserveDailyGeneratedCount(ctx context.Context, userID uuid.UUID, day time.Time, delta int, limit int) (bool, error) {
	if delta <= 0 {
		return true, nil
	}
	if limit <= 0 {
		return false, nil
	}

	query := `
INSERT INTO user_daily_generation_counts (user_id, date, count)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, date)
DO UPDATE SET
    count = user_daily_generation_counts.count + EXCLUDED.count
WHERE user_daily_generation_counts.count + EXCLUDED.count <= $4
RETURNING count`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID, day.Format("2006-01-02"), delta, limit).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("question repo: reserve daily generated count: %w", err)
	}

	return true, nil
}

func (r *questionRepository) GetUserLastQuestionSyncAt(ctx context.Context, userID uuid.UUID) (*time.Time, error) {
	var lastSync sql.NullTime
	if err := r.db.QueryRowContext(ctx, `
SELECT last_sync_at
FROM users
WHERE id = $1
`, userID).Scan(&lastSync); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("question repo: get user last question sync at: %w", err)
	}
	if !lastSync.Valid {
		return nil, nil
	}
	return &lastSync.Time, nil
}

func (r *questionRepository) UpdateUserLastQuestionSyncAt(ctx context.Context, userID uuid.UUID, syncedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE users
SET last_sync_at = $2,
    updated_at = NOW()
WHERE id = $1
`, userID, syncedAt.UTC())
	if err != nil {
		return fmt.Errorf("question repo: update user last question sync at: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("question repo: update user last question sync rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *questionRepository) QueueHighlightsWithinDailyLimit(
	ctx context.Context,
	userID uuid.UUID,
	day time.Time,
	limit int,
	highlightIDs []uuid.UUID,
	questionCountByHighlightID map[uuid.UUID]int,
	requestedAt time.Time,
) ([]uuid.UUID, bool, error) {
	if len(highlightIDs) == 0 || len(questionCountByHighlightID) == 0 {
		return make([]uuid.UUID, 0), true, nil
	}
	if limit <= 0 {
		return nil, false, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("question repo: begin sync queue transaction: %w", err)
	}
	defer tx.Rollback()

	queueQuery := `
UPDATE highlights
SET
    status = 'pending',
    retry_count = 0,
    generation_requested_at = $3,
    processing_started_at = NULL,
    completed_at = NULL,
    failed_at = NULL,
    last_error = NULL,
    updated_at = NOW()
WHERE user_id = $1
  AND id::text = ANY($2)
  AND status NOT IN ('pending', 'processing')
RETURNING id`

	rows, err := tx.QueryContext(ctx, queueQuery, userID, pq.Array(uuidStrings(highlightIDs)), requestedAt.UTC())
	if err != nil {
		return nil, false, fmt.Errorf("question repo: queue sync highlights: %w", err)
	}
	defer rows.Close()

	queuedIDs := make([]uuid.UUID, 0, len(highlightIDs))
	queuedQuestionCount := 0
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, false, fmt.Errorf("question repo: scan sync queued highlight id: %w", err)
		}
		queuedIDs = append(queuedIDs, id)
		queuedQuestionCount += questionCountByHighlightID[id]
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("question repo: rows sync queued highlight ids: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, false, fmt.Errorf("question repo: close sync queued highlight rows: %w", err)
	}

	if queuedQuestionCount <= 0 {
		if err := tx.Commit(); err != nil {
			return nil, false, fmt.Errorf("question repo: commit empty sync queue transaction: %w", err)
		}
		return queuedIDs, true, nil
	}

	reserveQuery := `
INSERT INTO user_daily_generation_counts (user_id, date, count)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, date)
DO UPDATE SET
    count = user_daily_generation_counts.count + EXCLUDED.count
WHERE user_daily_generation_counts.count + EXCLUDED.count <= $4
RETURNING count`

	var count int
	err = tx.QueryRowContext(ctx, reserveQuery, userID, day.Format("2006-01-02"), queuedQuestionCount, limit).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("question repo: reserve sync daily generated count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("question repo: commit sync queue transaction: %w", err)
	}

	return queuedIDs, true, nil
}

func (r *questionRepository) EnqueueRegeneration(ctx context.Context, userID string, highlightID uuid.UUID, questionID string) error {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("question repo: parse user id for regeneration queue: %w", err)
	}

	query := `
INSERT INTO regeneration_queue (
    user_id,
    highlight_id,
    requested_from_question_id,
    reason,
    status,
    requested_at,
    updated_at
) VALUES ($1, $2, $3, 'answer_submitted', 'pending', NOW(), NOW())
ON CONFLICT DO NOTHING`

	if _, err := r.db.ExecContext(ctx, query, uID, highlightID, parseOptionalUUID(questionID)); err != nil {
		return fmt.Errorf("question repo: enqueue regeneration: %w", err)
	}

	return nil
}

func (r *questionRepository) ClaimPendingRegenerationTasks(ctx context.Context, limit int) ([]*domain.RegenerationTask, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
WITH claimed AS (
    SELECT id
    FROM regeneration_queue
    WHERE status = 'pending'
      AND requested_at <= NOW()
    ORDER BY requested_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
),
updated AS (
    UPDATE regeneration_queue AS rq
    SET
        status = 'processing',
        processing_started_at = NOW(),
        updated_at = NOW()
    FROM claimed
    WHERE rq.id = claimed.id
    RETURNING
        rq.id,
        rq.user_id,
        rq.highlight_id,
        rq.retry_count,
        rq.requested_at,
        rq.requested_from_question_id,
        rq.reason
)
SELECT
    updated.id,
    updated.user_id,
    updated.highlight_id,
    updated.retry_count,
    updated.requested_at,
    updated.requested_from_question_id,
    updated.reason,
    h.id,
    h.user_id,
    h.book_id,
    h.book_title,
    h.book_author,
    h.asin,
    h.content,
    h.explanation,
    h.content_hash,
    h.location,
    h.highlighted_at,
    h.source,
    h.source_app,
    h.source_url,
    h.status,
    h.retry_count,
    h.last_error,
    h.generation_requested_at,
    h.processing_started_at,
    h.completed_at,
    h.failed_at,
    h.created_at,
    h.updated_at
FROM updated
JOIN highlights h
  ON h.id = updated.highlight_id`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("question repo: claim regeneration tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*domain.RegenerationTask, 0)
	for rows.Next() {
		var (
			task                domain.RegenerationTask
			requestedFromQID    uuid.NullUUID
			highlightBookID     uuid.NullUUID
			highlightBookTitle  sql.NullString
			highlightBookAuthor sql.NullString
			highlightASIN       sql.NullString
			highlightExplain    sql.NullString
			highlightHash       sql.NullString
			highlightLocation   sql.NullString
			highlightedAt       sql.NullTime
			highlightSource     sql.NullString
			highlightSourceApp  sql.NullString
			highlightSourceURL  sql.NullString
			highlightStatus     sql.NullString
			highlightLastError  sql.NullString
			highlightRequestAt  sql.NullTime
			highlightProcessAt  sql.NullTime
			highlightDoneAt     sql.NullTime
			highlightFailedAt   sql.NullTime
			highlightCreatedAt  time.Time
			highlightUpdatedAt  time.Time
			highlight           domain.Highlight
		)

		if err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.HighlightID,
			&task.RetryCount,
			&task.RequestedAt,
			&requestedFromQID,
			&task.Reason,
			&highlight.ID,
			&highlight.UserID,
			&highlightBookID,
			&highlightBookTitle,
			&highlightBookAuthor,
			&highlightASIN,
			&highlight.Content,
			&highlightExplain,
			&highlightHash,
			&highlightLocation,
			&highlightedAt,
			&highlightSource,
			&highlightSourceApp,
			&highlightSourceURL,
			&highlightStatus,
			&highlight.RetryCount,
			&highlightLastError,
			&highlightRequestAt,
			&highlightProcessAt,
			&highlightDoneAt,
			&highlightFailedAt,
			&highlightCreatedAt,
			&highlightUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("question repo: scan regeneration task: %w", err)
		}

		highlight.BookID = fromNullUUID(highlightBookID)
		highlight.BookTitle = fromNullString(highlightBookTitle)
		highlight.BookAuthor = fromNullString(highlightBookAuthor)
		highlight.ASIN = fromNullString(highlightASIN)
		highlight.Explanation = fromNullString(highlightExplain)
		highlight.ContentHash = fromNullString(highlightHash)
		highlight.Location = fromNullString(highlightLocation)
		highlight.HighlightedAt = fromNullTime(highlightedAt)
		highlight.Source = fromNullStringValue(highlightSource)
		highlight.SourceApp = fromNullString(highlightSourceApp)
		highlight.SourceURL = fromNullString(highlightSourceURL)
		highlight.Status = domain.HighlightStatus(strings.TrimSpace(highlightStatus.String))
		highlight.LastError = fromNullString(highlightLastError)
		highlight.ProcessingAt = fromNullTime(highlightProcessAt)
		highlight.CompletedAt = fromNullTime(highlightDoneAt)
		highlight.FailedAt = fromNullTime(highlightFailedAt)
		highlight.CreatedAt = highlightCreatedAt
		highlight.UpdatedAt = highlightUpdatedAt
		if highlightRequestAt.Valid {
			highlight.RequestedAt = highlightRequestAt.Time
		} else {
			highlight.RequestedAt = highlightCreatedAt
		}

		if requestedFromQID.Valid {
			task.RequestedFromQuestion = &requestedFromQID.UUID
		}
		task.Highlight = &highlight
		tasks = append(tasks, &task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("question repo: rows regeneration task: %w", err)
	}

	return tasks, nil
}

func (r *questionRepository) DeferRegenerationTasks(ctx context.Context, taskIDs []uuid.UUID, lastError string) error {
	if len(taskIDs) == 0 {
		return nil
	}

	query := `
UPDATE regeneration_queue
SET
    status = 'pending',
    requested_at = NOW() + INTERVAL '1 hour',
    processing_started_at = NULL,
    last_error = LEFT($2, 500),
    updated_at = NOW()
WHERE id::text = ANY($1)`

	if _, err := r.db.ExecContext(ctx, query, pq.Array(uuidTextSlice(taskIDs)), strings.TrimSpace(lastError)); err != nil {
		return fmt.Errorf("question repo: defer regeneration tasks: %w", err)
	}

	return nil
}

func (r *questionRepository) MarkRegenerationTasksCompleted(ctx context.Context, taskIDs []uuid.UUID) error {
	if len(taskIDs) == 0 {
		return nil
	}

	query := `
UPDATE regeneration_queue
SET
    status = 'completed',
    completed_at = NOW(),
    failed_at = NULL,
    last_error = NULL,
    updated_at = NOW()
WHERE id::text = ANY($1)`

	if _, err := r.db.ExecContext(ctx, query, pq.Array(uuidTextSlice(taskIDs))); err != nil {
		return fmt.Errorf("question repo: mark regeneration completed: %w", err)
	}

	return nil
}

func (r *questionRepository) MarkRegenerationTasksFailed(ctx context.Context, taskIDs []uuid.UUID, lastError string, maxRetry int) error {
	if len(taskIDs) == 0 {
		return nil
	}
	if maxRetry <= 0 {
		maxRetry = 3
	}

	query := `
UPDATE regeneration_queue
SET
    retry_count = retry_count + 1,
    status = CASE WHEN retry_count + 1 >= $3 THEN 'failed' ELSE 'pending' END,
    processing_started_at = NULL,
    failed_at = CASE WHEN retry_count + 1 >= $3 THEN NOW() ELSE failed_at END,
    last_error = LEFT($2, 500),
    updated_at = NOW()
WHERE id::text = ANY($1)`

	if _, err := r.db.ExecContext(ctx, query, pq.Array(uuidTextSlice(taskIDs)), strings.TrimSpace(lastError), maxRetry); err != nil {
		return fmt.Errorf("question repo: mark regeneration failed: %w", err)
	}

	return nil
}

func wrapQuestionError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("question repo: %s: %w", action, domain.ErrNotFound)
	}

	return fmt.Errorf("question repo: %s: %w", action, err)
}

func normalizeQuestionLimit(limit int) int32 {
	if limit <= 0 {
		return 50
	}

	return int32(limit)
}

func parseOptionalUUID(value string) uuid.NullUUID {
	if value == "" {
		return uuid.NullUUID{}
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.NullUUID{}
	}

	return uuid.NullUUID{UUID: parsed, Valid: true}
}

func nullUUIDString(value uuid.NullUUID) string {
	if !value.Valid {
		return ""
	}

	return value.UUID.String()
}

func nullStringOrEmpty(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}

func normalizePerspective(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return domain.QuestionPerspectiveDefinition
	}
	return trimmed
}

func normalizeQuestionVersion(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func uuidTextSlice(values []uuid.UUID) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, value.String())
	}
	return items
}

func decodeQuestionOptionsMessage(value pqtype.NullRawMessage) []string {
	if !value.Valid {
		return []string{}
	}

	return decodeQuestionOptionsBytes(value.RawMessage)
}

func decodeQuestionOptionsBytes(value []byte) []string {
	options := make([]string, 0)
	if len(value) == 0 {
		return options
	}

	if err := json.Unmarshal(value, &options); err != nil {
		return []string{}
	}

	return options
}
