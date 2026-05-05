package domain

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var (
	urlPattern                  = regexp.MustCompile(`(https?://|www\.)\S+`)
	horizontalWhitespacePattern = regexp.MustCompile(`[^\S\r\n]+`)
	extraNewlinePattern         = regexp.MustCompile(`\n{3,}`)
)

func NormalizeText(input string) string {
	normalized := normalizeBaseText(input)
	normalized = StripURLs(normalized)
	normalized = extraNewlinePattern.ReplaceAllString(normalized, "\n\n")
	return strings.TrimSpace(normalized)
}

func NormalizeMetaText(input string) string {
	return strings.TrimSpace(normalizeBaseText(input))
}

func StripURLs(input string) string {
	stripped := urlPattern.ReplaceAllString(input, "")
	return horizontalWhitespacePattern.ReplaceAllString(stripped, " ")
}

func normalizeBaseText(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))

	for _, r := range input {
		if shouldDropNormalizedRune(r) {
			continue
		}
		builder.WriteRune(r)
	}

	return norm.NFC.String(builder.String())
}

func shouldDropNormalizedRune(r rune) bool {
	return isFilteredControlRune(r) || isBidiControlRune(r) || isZeroWidthRune(r)
}

func isFilteredControlRune(r rune) bool {
	return (r >= 0x0000 && r <= 0x0008) ||
		r == 0x000B ||
		r == 0x000C ||
		(r >= 0x000E && r <= 0x001F) ||
		r == 0x007F
}

func isBidiControlRune(r rune) bool {
	switch r {
	case 0x202A, 0x202B, 0x202C, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069:
		return true
	default:
		return false
	}
}

func isZeroWidthRune(r rune) bool {
	switch r {
	case 0x200B, 0x200C, 0x200D, 0xFEFF:
		return true
	default:
		return false
	}
}
