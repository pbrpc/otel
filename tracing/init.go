//revive:disable:package-comments
package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/pbrpc/lifecycle"
)

const envVar = "OTEL_TRACES_EXPORTER"

// Init initializes the OTel TracerProvider based on OTEL_TRACES_EXPORTER.
//
//   - "otlp": OTLP/HTTP exporter to the collector's HTTP receiver
//   - "" or "none": no tracing (returns noop shutdown)
//
// Returns a ShutdownFunc to flush and close the provider.
func Init(
	ctx context.Context, res *resource.Resource,
) (lifecycle.CleanupFunc, error) {
	return initTracer(ctx, res)
}

func initTracer(
	ctx context.Context,
	res *resource.Resource,
	exporterOptions ...otlptracehttp.Option,
) (lifecycle.CleanupFunc, error) {
	exporterType := os.Getenv(envVar)

	if exporterType == "" || exporterType == "none" {
		noop := func(context.Context) error { return nil }
		return noop, nil
	}

	if exporterType != "otlp" {
		return nil, fmt.Errorf("unsupported %s: %s (supported: otlp, none)", envVar, exporterType)
	}

	exporter, err := otlptracehttp.New(ctx, exporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	return tp.Shutdown, nil
}
