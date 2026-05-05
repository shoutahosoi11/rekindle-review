package inprocess

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

const (
	defaultDispatcherJobTimeout    = 5 * time.Minute
	defaultDispatcherMaxConcurrent = 3
)

type JobRunner interface {
	ProcessQuestionGenerationJob(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error
}

type QuestionGenerationDispatcher struct {
	runner     JobRunner
	jobTimeout time.Duration
	sem        chan struct{}
	wg         sync.WaitGroup
}

func NewQuestionGenerationDispatcher(runner JobRunner, maxConcurrent int) *QuestionGenerationDispatcher {
	if maxConcurrent <= 0 {
		maxConcurrent = defaultDispatcherMaxConcurrent
	}
	return &QuestionGenerationDispatcher{
		runner:     runner,
		jobTimeout: defaultDispatcherJobTimeout,
		sem:        make(chan struct{}, maxConcurrent),
	}
}

func (d *QuestionGenerationDispatcher) EnqueueQuestionGeneration(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error {
	if d == nil || d.runner == nil {
		return domain.ErrInvalidInput
	}

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		d.sem <- struct{}{}
		defer func() { <-d.sem }()

		// The request context is canceled after the HTTP response, so jobs use a
		// bounded background context and rely on DB state for recovery.
		bgCtx, cancel := context.WithTimeout(context.Background(), d.jobTimeout)
		defer cancel()

		if err := d.runner.ProcessQuestionGenerationJob(bgCtx, jobID, userID); err != nil {
			log.Printf("inprocess dispatcher: process job error: job_id=%s user_id=%s err=%v", jobID, userID, err)
		}
	}()

	return nil
}

func (d *QuestionGenerationDispatcher) Wait() {
	if d == nil {
		return
	}
	d.wg.Wait()
}
