//revive:disable:package-comments
package otel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/pbrpc/lifecycle"
	"github.com/pbrpc/otel/logging"
	"github.com/pbrpc/otel/metrics"
	"github.com/pbrpc/otel/tracing"
)

type providerInitializers struct {
	logging func(
		context.Context, *resource.Resource, string,
	) (*slog.Logger, lifecycle.CleanupFunc, error)
	tracing func(
		context.Context, *resource.Resource,
	) (lifecycle.CleanupFunc, error)
	metrics func(
		context.Context, *resource.Resource,
	) (lifecycle.CleanupFunc, error)
}

// Init initializes all OpenTelemetry providers: logging first (so tracing and
// metrics initialization can log), then tracing, then metrics.
//
// Name and version are supplied by the caller, which reads them from the
// server configuration every other part of the process reads. They win over
// OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES, so a server's identity has
// one source rather than one per telemetry vendor.
//
// Every exporter speaks OTLP over HTTP, so OTEL_EXPORTER_OTLP_ENDPOINT names
// the collector's HTTP receiver.
//
// Returns the logger and a CleanupFunc that tears down all providers in
// reverse order (metrics, tracing, logging).
func Init(
	ctx context.Context, serviceName, version string,
) (*slog.Logger, lifecycle.CleanupFunc, error) {
	return initProviders(ctx, serviceName, version, providerInitializers{
		logging: logging.Init,
		tracing: tracing.Init,
		metrics: metrics.Init,
	})
}

func initProviders(
	ctx context.Context,
	serviceName string,
	version string,
	initializers providerInitializers,
) (*slog.Logger, lifecycle.CleanupFunc, error) {
	// resource.Merge only fails when resources have incompatible schema URLs.
	// Since we use NewSchemaless (no schema URL), this cannot fail.
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)

	var stack lifecycle.Stack

	// Logging first so tracing and metrics init can log.
	log, logShutdown, err := initializers.logging(ctx, res, serviceName)
	if err != nil {
		return nil, nil, fmt.Errorf("logging init: %w", err)
	}
	stack.Push(logShutdown)

	tracerShutdown, err := initializers.tracing(ctx, res)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("tracing init: %w", err),
			stack.Unwind(ctx),
		)
	}
	stack.Push(tracerShutdown)

	meterShutdown, err := initializers.metrics(ctx, res)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("metrics init: %w", err),
			stack.Unwind(ctx),
		)
	}
	stack.Push(meterShutdown)

	return log, stack.Unwind, nil
}
