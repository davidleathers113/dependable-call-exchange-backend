# Instrumentation Package

This package contains service instrumentation wrappers that add observability features (tracing, metrics) to business services.

## Purpose

The instrumentation package serves as a bridge between business logic (services) and infrastructure concerns (telemetry). By placing instrumentation wrappers here, we:

1. Avoid circular dependencies between service and infrastructure packages
2. Keep business logic clean and focused
3. Centralize observability concerns
4. Make instrumentation optional and replaceable

## Available Wrappers

- `CallRoutingTracedService`: Adds OpenTelemetry tracing to the call routing service

## Usage Example

```go
// In your dependency injection or service initialization
baseService := callrouting.NewService(repo)
tracedService := instrumentation.NewCallRoutingTracedService(
    baseService,
    tracer,
    metricsRegistry,
)
```