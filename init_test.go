//revive:disable:package-comments
package otel

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	"github.com/pbrpc/lifecycle"
)

const testInstanceID = "instance-under-test"

// disableExporters turns every exporter off, so a test that wants one on sets
// it after this.
func disableExporters(t *testing.T) {
	t.Helper()

	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")
}

func TestInit(t *testing.T) {
	t.Run("with all exporters disabled", func(t *testing.T) {
		disableExporters(t)

		logger, shutdown, err := Init(t.Context(), "test-service", "1.2.3", testInstanceID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logger == nil {
			t.Fatal("expected non-nil logger")
		}
		if shutdown == nil {
			t.Fatal("expected non-nil shutdown func")
		}
		if err := shutdown(t.Context()); err != nil {
			t.Errorf("shutdown error: %v", err)
		}
	})

	t.Run("with unsupported log exporter", func(t *testing.T) {
		disableExporters(t)
		t.Setenv("OTEL_LOGS_EXPORTER", "console")

		if _, _, err := Init(t.Context(), "test-service", "1.2.3", testInstanceID); err == nil {
			t.Fatal("expected error for unsupported log exporter")
		}
	})

	t.Run("with unsupported trace exporter", func(t *testing.T) {
		disableExporters(t)
		t.Setenv("OTEL_TRACES_EXPORTER", "invalid")

		if _, _, err := Init(t.Context(), "test-service", "1.2.3", testInstanceID); err == nil {
			t.Fatal("expected error for unsupported trace exporter")
		}
	})

	t.Run("with unsupported metric exporter", func(t *testing.T) {
		disableExporters(t)
		t.Setenv("OTEL_METRICS_EXPORTER", "invalid")

		if _, _, err := Init(t.Context(), "test-service", "1.2.3", testInstanceID); err == nil {
			t.Fatal("expected error for unsupported metric exporter")
		}
	})
}

type providerStub struct {
	t                *testing.T
	ctx              context.Context
	order            []string
	loggingErr       error
	tracingErr       error
	metricsErr       error
	logShutdownErr   error
	traceShutdownErr error
	meterShutdownErr error
}

func (s *providerStub) initializers() providerInitializers {
	return providerInitializers{
		logging: s.initLogging,
		tracing: s.initTracing,
		metrics: s.initMetrics,
	}
}

func (s *providerStub) initLogging(
	ctx context.Context,
	_ *resource.Resource,
	_ string,
	_ string,
) (*slog.Logger, lifecycle.CleanupFunc, error) {
	s.checkContext(ctx)
	if s.loggingErr != nil {
		return nil, nil, s.loggingErr
	}

	return slog.Default(), s.shutdown("logging", s.logShutdownErr), nil
}

func (s *providerStub) initTracing(
	ctx context.Context,
	res *resource.Resource,
) (lifecycle.CleanupFunc, error) {
	s.checkContext(ctx)
	s.checkResource(res)
	if s.tracingErr != nil {
		return nil, s.tracingErr
	}

	return s.shutdown("tracing", s.traceShutdownErr), nil
}

func (s *providerStub) initMetrics(
	ctx context.Context,
	_ *resource.Resource,
) (lifecycle.CleanupFunc, error) {
	s.checkContext(ctx)
	if s.metricsErr != nil {
		return nil, s.metricsErr
	}

	return s.shutdown("metrics", s.meterShutdownErr), nil
}

func (s *providerStub) shutdown(name string, err error) lifecycle.CleanupFunc {
	return func(ctx context.Context) error {
		s.checkContext(ctx)
		s.order = append(s.order, name)

		return err
	}
}

func (s *providerStub) checkContext(ctx context.Context) {
	s.t.Helper()
	if ctx != s.ctx {
		s.t.Errorf("context = %v, want test context %v", ctx, s.ctx)
	}
}

// checkResource asserts the resource every provider is built on names the
// service, its version, and this instance.
func (s *providerStub) checkResource(res *resource.Resource) {
	s.t.Helper()

	want := map[attribute.Key]string{
		semconv.ServiceNameKey:       "test-service",
		semconv.ServiceVersionKey:    "1.2.3",
		semconv.ServiceInstanceIDKey: testInstanceID,
	}

	for key, value := range want {
		if got, _ := res.Set().Value(key); got.AsString() != value {
			s.t.Errorf("resource %s = %q, want %q", key, got.AsString(), value)
		}
	}
}

func TestInitProviders(t *testing.T) {
	t.Run("returns lifecycle stack shutdown", func(t *testing.T) {
		ctx := t.Context()
		logErr := errors.New("logs unflushed")
		meterErr := errors.New("metrics unflushed")
		stub := providerStub{
			t:                t,
			ctx:              ctx,
			logShutdownErr:   logErr,
			meterShutdownErr: meterErr,
		}

		logger, shutdown, err := initProviders(
			ctx, "test-service", "1.2.3", testInstanceID, stub.initializers(),
		)
		if err != nil {
			t.Fatalf("initProviders() error = %v, want nil", err)
		}
		if logger == nil {
			t.Fatal("initProviders() logger = nil, want logger")
		}

		err = shutdown(ctx)
		if !errors.Is(err, logErr) {
			t.Errorf("shutdown error = %v, want error wrapping %v", err, logErr)
		}
		if !errors.Is(err, meterErr) {
			t.Errorf("shutdown error = %v, want error wrapping %v", err, meterErr)
		}
		if want := []string{"metrics", "tracing", "logging"}; !slices.Equal(stub.order, want) {
			t.Errorf("shutdown order = %v, want %v", stub.order, want)
		}
	})

	t.Run("returns logging initialization failure", func(t *testing.T) {
		ctx := t.Context()
		initErr := errors.New("logging unavailable")
		stub := providerStub{t: t, ctx: ctx, loggingErr: initErr}

		_, _, err := initProviders(ctx, "test-service", "1.2.3", testInstanceID, stub.initializers())
		if !errors.Is(err, initErr) {
			t.Errorf("initProviders() error = %v, want error wrapping %v", err, initErr)
		}
		if len(stub.order) != 0 {
			t.Errorf("shutdown order = %v, want none", stub.order)
		}
	})

	t.Run("cleans up after tracing initialization failure", func(t *testing.T) {
		ctx := t.Context()
		initErr := errors.New("tracing unavailable")
		cleanupErr := errors.New("logging cleanup")
		stub := providerStub{
			t:              t,
			ctx:            ctx,
			tracingErr:     initErr,
			logShutdownErr: cleanupErr,
		}

		_, _, err := initProviders(ctx, "test-service", "1.2.3", testInstanceID, stub.initializers())
		if !errors.Is(err, initErr) {
			t.Errorf("initProviders() error = %v, want error wrapping %v", err, initErr)
		}
		if !errors.Is(err, cleanupErr) {
			t.Errorf("initProviders() error = %v, want cleanup error wrapping %v", err, cleanupErr)
		}
		if want := []string{"logging"}; !slices.Equal(stub.order, want) {
			t.Errorf("shutdown order = %v, want %v", stub.order, want)
		}
	})

	t.Run("cleans up after metrics initialization failure", func(t *testing.T) {
		ctx := t.Context()
		initErr := errors.New("metrics unavailable")
		cleanupErr := errors.New("logging cleanup")
		stub := providerStub{
			t:              t,
			ctx:            ctx,
			metricsErr:     initErr,
			logShutdownErr: cleanupErr,
		}

		_, _, err := initProviders(ctx, "test-service", "1.2.3", testInstanceID, stub.initializers())
		if !errors.Is(err, initErr) {
			t.Errorf("initProviders() error = %v, want error wrapping %v", err, initErr)
		}
		if !errors.Is(err, cleanupErr) {
			t.Errorf("initProviders() error = %v, want cleanup error wrapping %v", err, cleanupErr)
		}
		if want := []string{"tracing", "logging"}; !slices.Equal(stub.order, want) {
			t.Errorf("shutdown order = %v, want %v", stub.order, want)
		}
	})
}
