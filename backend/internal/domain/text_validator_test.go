package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

func TestValidateTextLength(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		max     int
		wantErr bool
	}{
		{
			name:    "rejects zero length",
			input:   "",
			max:     5,
			wantErr: true,
		},
		{
			name:    "accepts max length",
			input:   "abc",
			max:     3,
			wantErr: false,
		},
		{
			name:    "rejects max plus one length",
			input:   "abcd",
			max:     3,
			wantErr: true,
		},
		{
			name:    "counts emoji as one rune",
			input:   "😀",
			max:     1,
			wantErr: false,
		},
		{
			name:    "rejects emoji when max is zero",
			input:   "😀",
			max:     0,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateTextLength(tc.input, tc.max)
			assertInvalidInputState(t, err, tc.wantErr)
		})
	}
}

func TestValidateLineCount(t *testing.T) {
	if err := domain.ValidateLineCount("a\nb\nc", 3); err != nil {
		t.Fatalf("ValidateLineCount returned unexpected error: %v", err)
	}

	err := domain.ValidateLineCount("a\nb\nc", 2)
	assertInvalidInputState(t, err, true)
}

func TestValidateMaxLineLength(t *testing.T) {
	if err := domain.ValidateMaxLineLength("ab\n😀", 2); err != nil {
		t.Fatalf("ValidateMaxLineLength returned unexpected error: %v", err)
	}

	err := domain.ValidateMaxLineLength("abc\nok", 2)
	assertInvalidInputState(t, err, true)
}

func TestValidateSourceURL(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "accepts http",
			input:   "http://example.com/path",
			wantErr: false,
		},
		{
			name:    "accepts https",
			input:   "https://example.com/path",
			wantErr: false,
		},
		{
			name:    "rejects javascript scheme",
			input:   "javascript:alert(1)",
			wantErr: true,
		},
		{
			name:    "rejects data scheme",
			input:   "data:text/plain,hello",
			wantErr: true,
		},
		{
			name:    "rejects file scheme",
			input:   "file:///tmp/test.txt",
			wantErr: true,
		},
		{
			name:    "rejects missing host",
			input:   "https:///missing-host",
			wantErr: true,
		},
		{
			name:    "rejects length over 2048",
			input:   makeURLWithLength(2049),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateSourceURL(tc.input)
			assertInvalidInputState(t, err, tc.wantErr)
		})
	}
}

func assertInvalidInputState(t *testing.T, err error, wantErr bool) {
	t.Helper()

	if !wantErr {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

func makeURLWithLength(totalLength int) string {
	const prefix = "https://example.com/"
	if totalLength <= len(prefix) {
		return prefix[:totalLength]
	}

	return prefix + strings.Repeat("a", totalLength-len(prefix))
}
