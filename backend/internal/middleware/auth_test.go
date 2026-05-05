package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/labstack/echo/v4"
)

type stubTokenVerifier struct {
	verifyFunc              func(ctx context.Context, idToken string) (*auth.Token, error)
	verifyWithoutRevokeFunc func(ctx context.Context, idToken string) (*auth.Token, error)
}

func (s stubTokenVerifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	if s.verifyWithoutRevokeFunc != nil {
		return s.verifyWithoutRevokeFunc(ctx, idToken)
	}
	return s.verifyFunc(ctx, idToken)
}

func (s stubTokenVerifier) VerifyIDTokenAndCheckRevoked(ctx context.Context, idToken string) (*auth.Token, error) {
	return s.verifyFunc(ctx, idToken)
}

func TestNewFirebaseMiddlewareRejectsNilVerifier(t *testing.T) {
	middleware, err := NewFirebaseMiddleware(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if middleware != nil {
		t.Fatal("expected nil middleware")
	}
}

func TestAuthenticateSetsFirebaseAuthOnSuccess(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	middleware, err := NewFirebaseMiddleware(stubTokenVerifier{
		verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
			if idToken != "valid-token" {
				t.Fatalf("unexpected token: %s", idToken)
			}
			return &auth.Token{UID: "firebase-uid-123"}, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := middleware.Authenticate(func(c echo.Context) error {
		firebaseUID, ok := GetFirebaseUID(c)
		if !ok {
			t.Fatal("expected firebase uid")
		}
		if firebaseUID != "firebase-uid-123" {
			t.Fatalf("unexpected firebase uid: %s", firebaseUID)
		}

		token, ok := GetFirebaseToken(c)
		if !ok {
			t.Fatal("expected firebase token")
		}
		if token.UID != "firebase-uid-123" {
			t.Fatalf("unexpected token uid: %s", token.UID)
		}

		return c.NoContent(http.StatusNoContent)
	})

	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestAuthenticateRejectsRevokedToken(t *testing.T) {
	originalClassifier := isFirebaseIDTokenClientError
	defer func() {
		isFirebaseIDTokenClientError = originalClassifier
	}()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer revoked-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	revokedErr := errors.New("revoked token")
	isFirebaseIDTokenClientError = func(err error) bool {
		return err == revokedErr
	}

	middleware, err := NewFirebaseMiddleware(stubTokenVerifier{
		verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
			if idToken != "revoked-token" {
				t.Fatalf("unexpected token: %s", idToken)
			}
			return nil, revokedErr
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := middleware.Authenticate(func(c echo.Context) error {
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
	if httpErr.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestAuthenticateRejectsMissingOrMalformedAuthorizationHeader(t *testing.T) {
	testCases := []struct {
		name          string
		authorization string
	}{
		{name: "MissingHeader"},
		{name: "BearerOnly", authorization: "Bearer"},
		{name: "NonBearerScheme", authorization: "Basic xyz"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			verifierCalled := false
			middleware, err := NewFirebaseMiddleware(stubTokenVerifier{
				verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
					verifierCalled = true
					return nil, errors.New("verifier should not be called")
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			nextCalled := false
			handler := middleware.Authenticate(func(c echo.Context) error {
				nextCalled = true
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
			if httpErr.Code != http.StatusUnauthorized {
				t.Fatalf("unexpected status: %d", httpErr.Code)
			}
			if verifierCalled {
				t.Fatal("verifier should not be called")
			}
			if nextCalled {
				t.Fatal("next handler should not be called")
			}
		})
	}
}

func TestAuthenticateReturnsServiceUnavailableOnVerifierFailure(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	middleware, err := NewFirebaseMiddleware(stubTokenVerifier{
		verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
			if idToken != "valid-token" {
				t.Fatalf("unexpected token: %s", idToken)
			}
			return nil, errors.New("firebase unavailable")
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := middleware.Authenticate(func(c echo.Context) error {
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
	if httpErr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestAuthenticateFailsClosedWhenRevocationCheckUnavailable(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	middleware, err := NewFirebaseMiddleware(stubTokenVerifier{
		verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
			if idToken != "valid-token" {
				t.Fatalf("unexpected token: %s", idToken)
			}
			return nil, errors.New("firebase auth backend unavailable")
		},
		verifyWithoutRevokeFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
			t.Fatal("basic verification should not be called when revocation check fails")
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := middleware.Authenticate(func(c echo.Context) error {
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
	if httpErr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestAuthenticateRejectsNilOrEmptyUIDToken(t *testing.T) {
	testCases := []struct {
		name  string
		token *auth.Token
	}{
		{name: "NilToken", token: nil},
		{name: "EmptyUIDToken", token: &auth.Token{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			middleware, err := NewFirebaseMiddleware(stubTokenVerifier{
				verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
					if idToken != "valid-token" {
						t.Fatalf("unexpected token: %s", idToken)
					}
					return tc.token, nil
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			nextCalled := false
			handler := middleware.Authenticate(func(c echo.Context) error {
				nextCalled = true
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
			if httpErr.Code != http.StatusServiceUnavailable {
				t.Fatalf("unexpected status: %d", httpErr.Code)
			}
			if nextCalled {
				t.Fatal("next handler should not be called")
			}
			if firebaseUID, ok := GetFirebaseUID(c); ok || firebaseUID != "" {
				t.Fatalf("expected empty firebase uid, got %q (ok=%v)", firebaseUID, ok)
			}
			if token, ok := GetFirebaseToken(c); ok || token != nil {
				t.Fatalf("expected empty firebase token, got %#v (ok=%v)", token, ok)
			}
		})
	}
}
