package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/repository/sqlcgen"
)

type rateLimitRepository struct {
	queries *sqlcgen.Queries
}

func NewRateLimitRepository(db *sql.DB) domain.RateLimitRepository {
	return &rateLimitRepository{
		queries: sqlcgen.New(db),
	}
}

func (r *rateLimitRepository) IncrementAndCheck(ctx context.Context, userID, bucket string, limit int64) (int64, bool, error) {
	current, err := r.queries.IncrementRateLimitCounter(ctx, sqlcgen.IncrementRateLimitCounterParams{
		UserID: userID,
		Bucket: bucket,
	})
	if err != nil {
		return 0, false, fmt.Errorf("rate limit repo: increment counter: %w", err)
	}

	return current, current > limit, nil
}
