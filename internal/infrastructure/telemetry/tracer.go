package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracerInterface defines the interface for distributed tracing
type TracerInterface interface {
	// StartSpan starts a new span with the given name
	StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span)

	// StartSpanWithAttributes starts a new span with attributes
	StartSpanWithAttributes(ctx context.Context, spanName string, attrs map[string]interface{}, opts ...trace.SpanStartOption) (context.Context, trace.Span)

	// GetSpan returns the current span from context
	GetSpan(ctx context.Context) trace.Span

	// SetStatus sets the status of a span
	SetStatus(span trace.Span, code codes.Code, description string)

	// RecordError records an error on the span
	RecordError(span trace.Span, err error, description string)

	// AddEvent adds an event to the span
	AddEvent(span trace.Span, name string, attrs map[string]interface{})

	// SetAttributes sets attributes on a span
	SetAttributes(span trace.Span, attrs map[string]interface{})

	// GetTraceID returns the trace ID from the span
	GetTraceID(span trace.Span) string

	// GetSpanID returns the span ID
	GetSpanID(span trace.Span) string
}

// OpenTelemetryTracer implements TracerInterface using OpenTelemetry
type OpenTelemetryTracer struct {
	tracer trace.Tracer
	name   string
}

// NewOpenTelemetryTracer creates a new OpenTelemetry tracer
func NewOpenTelemetryTracer(name string) *OpenTelemetryTracer {
	return &OpenTelemetryTracer{
		tracer: otel.Tracer(name),
		name:   name,
	}
}

// StartSpan starts a new span with the given name
func (t *OpenTelemetryTracer) StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, spanName, opts...)
}

// StartSpanWithAttributes starts a new span with attributes
func (t *OpenTelemetryTracer) StartSpanWithAttributes(ctx context.Context, spanName string, attrs map[string]interface{}, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	spanAttrs := t.convertAttributes(attrs)
	allOpts := append(opts, trace.WithAttributes(spanAttrs...))
	return t.tracer.Start(ctx, spanName, allOpts...)
}

// GetSpan returns the current span from context
func (t *OpenTelemetryTracer) GetSpan(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// SetStatus sets the status of a span
func (t *OpenTelemetryTracer) SetStatus(span trace.Span, code codes.Code, description string) {
	span.SetStatus(code, description)
}

// RecordError records an error on the span
func (t *OpenTelemetryTracer) RecordError(span trace.Span, err error, description string) {
	if err != nil {
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.description", description),
		))
		span.SetStatus(codes.Error, err.Error())
	}
}

// AddEvent adds an event to the span
func (t *OpenTelemetryTracer) AddEvent(span trace.Span, name string, attrs map[string]interface{}) {
	eventAttrs := t.convertAttributes(attrs)
	span.AddEvent(name, trace.WithAttributes(eventAttrs...))
}

// SetAttributes sets attributes on a span
func (t *OpenTelemetryTracer) SetAttributes(span trace.Span, attrs map[string]interface{}) {
	spanAttrs := t.convertAttributes(attrs)
	span.SetAttributes(spanAttrs...)
}

// GetTraceID returns the trace ID from the span
func (t *OpenTelemetryTracer) GetTraceID(span trace.Span) string {
	spanCtx := span.SpanContext()
	if spanCtx.HasTraceID() {
		return spanCtx.TraceID().String()
	}
	return ""
}

// GetSpanID returns the span ID
func (t *OpenTelemetryTracer) GetSpanID(span trace.Span) string {
	spanCtx := span.SpanContext()
	if spanCtx.HasSpanID() {
		return spanCtx.SpanID().String()
	}
	return ""
}

// convertAttributes converts a map to OpenTelemetry attributes
func (t *OpenTelemetryTracer) convertAttributes(attrs map[string]interface{}) []attribute.KeyValue {
	var result []attribute.KeyValue
	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			result = append(result, attribute.String(k, val))
		case int:
			result = append(result, attribute.Int(k, val))
		case int64:
			result = append(result, attribute.Int64(k, val))
		case float64:
			result = append(result, attribute.Float64(k, val))
		case bool:
			result = append(result, attribute.Bool(k, val))
		case []string:
			result = append(result, attribute.StringSlice(k, val))
		case []int:
			result = append(result, attribute.IntSlice(k, val))
		case []int64:
			result = append(result, attribute.Int64Slice(k, val))
		case []float64:
			result = append(result, attribute.Float64Slice(k, val))
		case []bool:
			result = append(result, attribute.BoolSlice(k, val))
		default:
			// For any other type, convert to string
			result = append(result, attribute.String(k, fmt.Sprintf("%v", val)))
		}
	}
	return result
}

// Helper functions for common span operations

// StartHTTPSpan starts a span for HTTP requests
func StartHTTPSpan(ctx context.Context, tracer TracerInterface, method, path string) (context.Context, trace.Span) {
	return tracer.StartSpanWithAttributes(ctx, fmt.Sprintf("%s %s", method, path), map[string]interface{}{
		"http.method": method,
		"http.target": path,
		"span.kind":   "server",
		"component":   "http",
	})
}

// StartDatabaseSpan starts a span for database operations
func StartDatabaseSpan(ctx context.Context, tracer TracerInterface, operation, table string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("db.%s %s", operation, table)
	return tracer.StartSpanWithAttributes(ctx, spanName, map[string]interface{}{
		"db.operation": operation,
		"db.table":     table,
		"db.system":    "postgresql",
		"span.kind":    "client",
		"component":    "database",
	})
}

// StartServiceSpan starts a span for service operations
func StartServiceSpan(ctx context.Context, tracer TracerInterface, service, operation string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("%s.%s", service, operation)
	return tracer.StartSpanWithAttributes(ctx, spanName, map[string]interface{}{
		"service.name":      service,
		"service.operation": operation,
		"span.kind":         "internal",
		"component":         "service",
	})
}

// StartMessagingSpan starts a span for messaging operations
func StartMessagingSpan(ctx context.Context, tracer TracerInterface, system, operation, destination string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("%s %s %s", system, operation, destination)
	return tracer.StartSpanWithAttributes(ctx, spanName, map[string]interface{}{
		"messaging.system":      system,
		"messaging.operation":   operation,
		"messaging.destination": destination,
		"span.kind":             "producer",
		"component":             "messaging",
	})
}

// WithSpanError is a helper to record errors and set span status
func WithSpanError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
