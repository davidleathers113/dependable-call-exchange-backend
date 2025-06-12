package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds the configuration for OpenTelemetry
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	// OTLP endpoint for traces and metrics
	OTLPEndpoint string
	// Enable/disable telemetry
	Enabled bool
	// Sampling rate (0.0 to 1.0)
	SamplingRate float64
	// Export timeout
	ExportTimeout time.Duration
	// Batch timeout for trace exporter
	BatchTimeout time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		ServiceName:    "dce-backend",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		OTLPEndpoint:   "localhost:4317",
		Enabled:        true,
		SamplingRate:   1.0,
		ExportTimeout:  30 * time.Second,
		BatchTimeout:   5 * time.Second,
	}
}

// Provider holds the OpenTelemetry providers
type Provider struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	Resource       *resource.Resource
	shutdown       []func(context.Context) error
}

// Shutdown gracefully shuts down all providers
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error
	for _, fn := range p.shutdown {
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// InitializeOpenTelemetry sets up OpenTelemetry with OTLP exporters
func InitializeOpenTelemetry(ctx context.Context, cfg *Config) (*Provider, error) {
	if !cfg.Enabled {
		// Return no-op providers when disabled
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		// Return a provider with no-op implementations
		return &Provider{
			TracerProvider: trace.NewNoopTracerProvider(),
			MeterProvider:  nil, // OpenTelemetry doesn't provide a no-op meter provider
		}, nil
	}

	// Create resource
	res, err := newResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize trace provider
	tp, tpShutdown, err := newTraceProvider(ctx, cfg, res)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace provider: %w", err)
	}

	// Initialize metric provider
	mp, mpShutdown, err := newMeterProvider(ctx, cfg, res)
	if err != nil {
		tpShutdown(ctx) // Clean up trace provider
		return nil, fmt.Errorf("failed to create meter provider: %w", err)
	}

	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		TracerProvider: tp,
		MeterProvider:  mp,
		Resource:       res,
		shutdown: []func(context.Context) error{
			tpShutdown,
			mpShutdown,
		},
	}, nil
}

// newResource creates a new resource with service information
func newResource(cfg *Config) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
			attribute.String("service.namespace", "dce"),
		),
	)
}

// newTraceProvider creates a new trace provider with OTLP exporter
func newTraceProvider(ctx context.Context, cfg *Config, res *resource.Resource) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	// Create OTLP trace exporter
	exporter, err := otlptrace.New(
		ctx,
		otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(), // TODO: Configure TLS for production
			otlptracegrpc.WithTimeout(cfg.ExportTimeout),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create sampler based on configuration
	var sampler sdktrace.Sampler
	if cfg.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SamplingRate)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(cfg.BatchTimeout),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	return tp, tp.Shutdown, nil
}

// newMeterProvider creates a new meter provider with OTLP exporter
func newMeterProvider(ctx context.Context, cfg *Config, res *resource.Resource) (*sdkmetric.MeterProvider, func(context.Context) error, error) {
	// Create OTLP metric exporter
	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(), // TODO: Configure TLS for production
		otlpmetricgrpc.WithTimeout(cfg.ExportTimeout),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create meter provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(10*time.Second), // Export metrics every 10 seconds
			),
		),
		sdkmetric.WithResource(res),
	)

	return mp, mp.Shutdown, nil
}

// Tracer returns a tracer for the given name
func Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return otel.Tracer(name, opts...)
}

// Meter returns a meter for the given name
func Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return otel.Meter(name, opts...)
}

// RecordError records an error on the span with additional context
func RecordError(span trace.Span, err error, opts ...trace.EventOption) {
	if err != nil {
		span.RecordError(err, opts...)
		span.SetStatus(codes.Error, err.Error())
	}
}

// AddEvent adds an event to the span with attributes
func AddEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	span.AddEvent(name, trace.WithAttributes(attrs...))
}
