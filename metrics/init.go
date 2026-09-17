//revive:disable:package-comments
package metrics

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/pbrpc/lifecycle"
)

const envVar = "OTEL_METRICS_EXPORTER"

// Init initializes the OTel MeterProvider based on OTEL_METRICS_EXPORTER.
//
//   - "otlp": OTLP/HTTP exporter to the collector's HTTP receiver
//   - "" or "none": no metrics (returns noop shutdown)
//
// Returns a CleanupFunc to flush and close the provider.
func Init(
	ctx context.Context, res *resource.Resource,
) (lifecycle.CleanupFunc, error) {
	return initMeter(ctx, res)
}

func initMeter(
	ctx context.Context,
	res *resource.Resource,
	exporterOptions ...otlpmetrichttp.Option,
) (lifecycle.CleanupFunc, error) {
	exporterType := os.Getenv(envVar)

	if exporterType == "" || exporterType == "none" {
		noop := func(context.Context) error { return nil }
		return noop, nil
	}

	if exporterType != "otlp" {
		return nil, fmt.Errorf("unsupported %s: %s (supported: otlp, none)", envVar, exporterType)
	}

	exporter, err := otlpmetrichttp.New(ctx, exporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(exporter)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}
