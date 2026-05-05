package logging

import (
	"log/slog"
	"os"
)

// Setup initializes the global slog logger.
// APP_ENV=production → JSON with Cloud Logging compatible field names.
// Otherwise          → human-readable text for local development.
//
// After this call, log.Printf and friends are automatically redirected to slog.
func Setup(appEnv string) {
	var handler slog.Handler

	if appEnv == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:       slog.LevelInfo,
			ReplaceAttr: cloudLoggingReplaceAttr,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	slog.SetDefault(slog.New(handler))
}

// cloudLoggingReplaceAttr maps slog's default field names to Cloud Logging conventions.
// https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
func cloudLoggingReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.LevelKey:
		a.Key = "severity"
		if level, ok := a.Value.Any().(slog.Level); ok {
			a.Value = slog.StringValue(toCloudLoggingSeverity(level))
		}
	case slog.MessageKey:
		a.Key = "message"
	case slog.TimeKey:
		a.Key = "timestamp"
	}
	return a
}

func toCloudLoggingSeverity(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARNING"
	case level >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}
