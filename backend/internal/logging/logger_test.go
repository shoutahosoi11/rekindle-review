package logging

import (
	"log/slog"
	"testing"
)

func TestToCloudLoggingSeverity(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelError, "ERROR"},
		{slog.LevelError + 4, "ERROR"},
		{slog.LevelWarn, "WARNING"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelDebug - 1, "DEBUG"},
	}

	for _, tt := range tests {
		got := toCloudLoggingSeverity(tt.level)
		if got != tt.expected {
			t.Errorf("toCloudLoggingSeverity(%v) = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestCloudLoggingReplaceAttr(t *testing.T) {
	tests := []struct {
		input   slog.Attr
		wantKey string
	}{
		{slog.Any(slog.LevelKey, slog.LevelInfo), "severity"},
		{slog.String(slog.MessageKey, "hello"), "message"},
		{slog.String(slog.TimeKey, "2024-01-01"), "timestamp"},
		{slog.String("custom_key", "val"), "custom_key"},
	}

	for _, tt := range tests {
		got := cloudLoggingReplaceAttr(nil, tt.input)
		if got.Key != tt.wantKey {
			t.Errorf("cloudLoggingReplaceAttr(%q).Key = %q, want %q", tt.input.Key, got.Key, tt.wantKey)
		}
	}
}
