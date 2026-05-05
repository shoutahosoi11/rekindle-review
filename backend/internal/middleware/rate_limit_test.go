package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type stubRateLimitStore struct {
	current  int64
	exceeded bool
	err      error
	userID   string
	bucket   string
	limit    int64
}

func (s *stubRateLimitStore) IncrementAndCheck(ctx context.Context, userID, bucket string, limit int64) (int64, bool, error) {
	s.userID = userID
	s.bucket = bucket
	s.limit = limit
	if s.err != nil {
		return 0, false, s.err
	}
	return s.current, s.exceeded, nil
}

func TestRateLimitAllowsRequestWithinLimit(t *testing.T) {
	store := &stubRateLimitStore{current: 1}
	middleware, err := NewRateLimitMiddleware(store, "ingest", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(ContextFirebaseUIDKey, "firebase-uid-1")

	handler := middleware.Limit(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if store.userID != "firebase-uid-1" {
		t.Fatalf("unexpected user id: %s", store.userID)
	}
	if store.bucket != "ingest" {
		t.Fatalf("unexpected bucket: %s", store.bucket)
	}
	if store.limit != 100 {
		t.Fatalf("unexpected limit: %d", store.limit)
	}
}

func TestRateLimitRejectsExceededRequest(t *testing.T) {
	store := &stubRateLimitStore{current: 101, exceeded: true}
	middleware, err := NewRateLimitMiddleware(store, "ingest", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(ContextFirebaseUIDKey, "firebase-uid-1")

	handler := middleware.Limit(func(c echo.Context) error {
		t.Fatal("next handler should not be called")
		return nil
	})

	err = handler(c)
	if err == nil {
		t.Fatal("expected error")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
	if rec.Header().Get(echo.HeaderRetryAfter) != "86400" {
		t.Fatalf("unexpected Retry-After: %s", rec.Header().Get(echo.HeaderRetryAfter))
	}
}

func TestRateLimitReturnsServiceUnavailableOnStoreError(t *testing.T) {
	store := &stubRateLimitStore{err: errors.New("database unavailable")}
	middleware, err := NewRateLimitMiddleware(store, "ingest", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(ContextFirebaseUIDKey, "firebase-uid-1")

	err = middleware.Limit(func(c echo.Context) error {
		t.Fatal("next handler should not be called")
		return nil
	})(c)
	if err == nil {
		t.Fatal("expected error")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}
