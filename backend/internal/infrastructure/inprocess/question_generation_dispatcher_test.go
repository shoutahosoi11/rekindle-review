package inprocess

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeJobRunner struct {
	process func(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error
}

func (f *fakeJobRunner) ProcessQuestionGenerationJob(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error {
	if f.process == nil {
		return nil
	}
	return f.process(ctx, jobID, userID)
}

func TestQuestionGenerationDispatcherEnqueueReturnsImmediatelyAndRunsJob(t *testing.T) {
	called := make(chan struct{}, 1)
	dispatcher := NewQuestionGenerationDispatcher(&fakeJobRunner{
		process: func(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error {
			called <- struct{}{}
			return nil
		},
	}, 1)

	if err := dispatcher.EnqueueQuestionGeneration(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("EnqueueQuestionGeneration failed: %v", err)
	}

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("expected job runner to be called")
	}

	dispatcher.Wait()
}

func TestQuestionGenerationDispatcherIgnoresRunnerError(t *testing.T) {
	called := make(chan struct{}, 1)
	dispatcher := NewQuestionGenerationDispatcher(&fakeJobRunner{
		process: func(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error {
			called <- struct{}{}
			return errors.New("runner failed")
		},
	}, 1)

	if err := dispatcher.EnqueueQuestionGeneration(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("EnqueueQuestionGeneration failed: %v", err)
	}

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("expected job runner to be called")
	}

	dispatcher.Wait()
}

func TestQuestionGenerationDispatcherContinuesAfterRequestContextCanceled(t *testing.T) {
	called := make(chan bool, 1)
	dispatcher := NewQuestionGenerationDispatcher(&fakeJobRunner{
		process: func(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error {
			called <- ctx.Err() == nil
			return nil
		},
	}, 1)

	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := dispatcher.EnqueueQuestionGeneration(reqCtx, uuid.New(), uuid.New()); err != nil {
		t.Fatalf("EnqueueQuestionGeneration failed: %v", err)
	}

	select {
	case backgroundStillAlive := <-called:
		if !backgroundStillAlive {
			t.Fatal("expected dispatcher to use a live background context")
		}
	case <-time.After(time.Second):
		t.Fatal("expected job runner to be called")
	}

	dispatcher.Wait()
}

func TestQuestionGenerationDispatcherLimitsConcurrency(t *testing.T) {
	const maxConcurrent = 2
	const totalJobs = 5

	started := make(chan struct{}, totalJobs)
	release := make(chan struct{})
	var mu sync.Mutex
	current := 0
	peak := 0

	dispatcher := NewQuestionGenerationDispatcher(&fakeJobRunner{
		process: func(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error {
			mu.Lock()
			current++
			if current > peak {
				peak = current
			}
			mu.Unlock()

			started <- struct{}{}
			<-release

			mu.Lock()
			current--
			mu.Unlock()
			return nil
		},
	}, maxConcurrent)

	for index := 0; index < totalJobs; index++ {
		if err := dispatcher.EnqueueQuestionGeneration(context.Background(), uuid.New(), uuid.New()); err != nil {
			t.Fatalf("EnqueueQuestionGeneration failed: %v", err)
		}
	}

	for index := 0; index < maxConcurrent; index++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("expected %d jobs to start", maxConcurrent)
		}
	}

	select {
	case <-started:
		t.Fatal("expected concurrency limiter to block the third job")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	dispatcher.Wait()

	if peak > maxConcurrent {
		t.Fatalf("expected peak concurrency <= %d, got %d", maxConcurrent, peak)
	}
}
