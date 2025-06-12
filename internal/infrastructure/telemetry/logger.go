package telemetry

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// SetupLogger creates a new structured logger with OpenTelemetry integration
func SetupLogger(level string) (*slog.Logger, error) {
	var logLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: logLevel == slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Add custom formatting if needed
			return a
		},
	}

	// Create a custom handler that adds trace context
	handler := &TracedHandler{
		Handler: slog.NewJSONHandler(os.Stdout, opts),
	}

	logger := slog.New(handler)

	return logger, nil
}

// TracedHandler is a custom slog handler that adds OpenTelemetry trace context
type TracedHandler struct {
	slog.Handler
}

// Handle adds trace context to log records
func (h *TracedHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract span from context
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		// Add trace ID and span ID as attributes
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)

		// Add trace flags if sampled
		if span.SpanContext().IsSampled() {
			r.AddAttrs(slog.Bool("sampled", true))
		}
	}

	return h.Handler.Handle(ctx, r)
}

// WithContext returns a new logger with the context
func WithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	return logger.With(extractTraceAttrs(ctx)...)
}

// extractTraceAttrs extracts trace attributes from context
func extractTraceAttrs(ctx context.Context) []any {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil
	}

	attrs := []any{
		"trace_id", span.SpanContext().TraceID().String(),
		"span_id", span.SpanContext().SpanID().String(),
	}

	if span.SpanContext().IsSampled() {
		attrs = append(attrs, "sampled", true)
	}

	return attrs
}

// LoggerWithTrace creates a logger that automatically includes trace context
type LoggerWithTrace struct {
	logger *slog.Logger
}

// NewLoggerWithTrace creates a new logger wrapper that includes trace context
func NewLoggerWithTrace(logger *slog.Logger) *LoggerWithTrace {
	return &LoggerWithTrace{logger: logger}
}

// Debug logs at debug level with trace context
func (l *LoggerWithTrace) Debug(ctx context.Context, msg string, args ...any) {
	WithContext(ctx, l.logger).Debug(msg, args...)
}

// Info logs at info level with trace context
func (l *LoggerWithTrace) Info(ctx context.Context, msg string, args ...any) {
	WithContext(ctx, l.logger).Info(msg, args...)
}

// Warn logs at warn level with trace context
func (l *LoggerWithTrace) Warn(ctx context.Context, msg string, args ...any) {
	WithContext(ctx, l.logger).Warn(msg, args...)
}

// Error logs at error level with trace context
func (l *LoggerWithTrace) Error(ctx context.Context, msg string, args ...any) {
	WithContext(ctx, l.logger).Error(msg, args...)
}

// With returns a new logger with additional attributes
func (l *LoggerWithTrace) With(args ...any) *LoggerWithTrace {
	return &LoggerWithTrace{logger: l.logger.With(args...)}
}
