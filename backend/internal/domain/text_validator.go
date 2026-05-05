package domain

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

func ValidateTextLength(input string, max int) error {
	length := utf8.RuneCountInString(input)
	if length == 0 || length > max {
		return ErrInvalidInput
	}

	return nil
}

func ValidateLineCount(input string, max int) error {
	lineCount := strings.Count(input, "\n") + 1
	if lineCount > max {
		return ErrInvalidInput
	}

	return nil
}

func ValidateMaxLineLength(input string, max int) error {
	for _, line := range strings.Split(input, "\n") {
		if utf8.RuneCountInString(line) > max {
			return ErrInvalidInput
		}
	}

	return nil
}

func ValidateSourceURL(input string) error {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || len(trimmed) > 2048 {
		return ErrInvalidInput
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ErrInvalidInput
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidInput
	}

	if parsed.Hostname() == "" {
		return ErrInvalidInput
	}

	return nil
}
