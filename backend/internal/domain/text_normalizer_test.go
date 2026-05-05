package domain_test

import (
	"testing"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

func TestNormalizeText(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes null bytes",
			input: "ab\x00cd",
			want:  "abcd",
		},
		{
			name:  "removes filtered control runes and keeps newlines",
			input: "A\x01B\nC\x1fD",
			want:  "AB\nCD",
		},
		{
			name:  "removes bidi control runes",
			input: "abc\u202Edef",
			want:  "abcdef",
		},
		{
			name:  "removes zero width runes",
			input: "zero\u200Bwidth",
			want:  "zerowidth",
		},
		{
			name:  "normalizes nfc composed text",
			input: "カ\u3099クセイ",
			want:  "ガクセイ",
		},
		{
			name:  "keeps valid japanese text",
			input: "  これは正常な日本語の文章です。  ",
			want:  "これは正常な日本語の文章です。",
		},
		{
			name:  "keeps valid english text",
			input: "  This is a valid English sentence.  ",
			want:  "This is a valid English sentence.",
		},
		{
			name:  "strips multiple urls",
			input: "See http://a.example and https://b.example plus www.c.example now",
			want:  "See and plus now",
		},
		{
			name:  "returns empty string for empty input",
			input: "",
			want:  "",
		},
		{
			name:  "collapses three or more newlines to two",
			input: "first\n\n\n\nsecond",
			want:  "first\n\nsecond",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.NormalizeText(tc.input)
			if got != tc.want {
				t.Fatalf("NormalizeText(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeMetaText(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "keeps urls in metadata",
			input: "  Title https://example.com  ",
			want:  "Title https://example.com",
		},
		{
			name:  "does not collapse newlines",
			input: "first\n\n\n\nsecond",
			want:  "first\n\n\n\nsecond",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.NormalizeMetaText(tc.input)
			if got != tc.want {
				t.Fatalf("NormalizeMetaText(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestStripURLs(t *testing.T) {
	input := "Before http://a.example  middle\twww.b.example after"
	want := "Before middle after"

	got := domain.StripURLs(input)
	if got != want {
		t.Fatalf("StripURLs(%q) = %q, want %q", input, got, want)
	}
}
