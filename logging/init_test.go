//revive:disable:package-comments
package logging

import (
	"crypto/tls"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/pbrpc/testing/mocks/roundtripper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func testHTTPClient() *http.Client {
	return &http.Client{Transport: roundtripper.Respond(
		http.StatusOK,
		http.Header{"Content-Type": []string{"application/x-protobuf"}},
		"",
	)}
}

func testResource() *resource.Resource {
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(semconv.ServiceName("test-service")),
	)
	return res
}

func TestInit(t *testing.T) {
	// Every stdout format is exercised against the OTLP provider, since the
	// handler chain is what differs between them.
	for _, format := range []string{"none", "json", "text", "flat", "structured", "", "JSON"} {
		t.Run("otlp with LOG_FORMAT "+format, func(t *testing.T) {
			t.Setenv(envVar, "otlp")
			t.Setenv("LOG_FORMAT", format)

			logger, shutdown, err := initLogger(
				t.Context(),
				testResource(),
				"test-service",
				otlploghttp.WithHTTPClient(testHTTPClient()),
			)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
			if shutdown == nil {
				t.Fatal("expected non-nil shutdown")
			}

			if err := shutdown(t.Context()); err != nil {
				t.Fatalf("shutdown error: %v", err)
			}
		})
	}

	for _, value := range []string{"none", ""} {
		t.Run("disabled by "+value, func(t *testing.T) {
			t.Setenv(envVar, value)
			t.Setenv("LOG_FORMAT", "structured")

			logger, shutdown, err := Init(t.Context(), testResource(), "test-service")
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
			if shutdown == nil {
				t.Fatal("expected non-nil shutdown")
			}
			if err := shutdown(t.Context()); err != nil {
				t.Fatalf("expected nil error from noop shutdown, got %v", err)
			}
		})
	}

	t.Run("routes SDK errors to stdout", func(t *testing.T) {
		t.Setenv(envVar, "none")
		t.Setenv("LOG_FORMAT", "json")

		if _, _, err := Init(t.Context(), testResource(), "test-service"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// What the SDK does when an export fails. The handler Init installed
		// writes the record to stdout, so the only observable is that this
		// returns rather than recursing into the exporter.
		otel.Handle(errors.New("export failed"))
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv(envVar, "invalid")

		_, shutdown, err := Init(t.Context(), testResource(), "test-service")
		if err == nil {
			t.Fatal("expected error for unsupported exporter")
		}
		if shutdown != nil {
			t.Fatal("expected nil shutdown on error")
		}
	})

	t.Run("exporter creation failure", func(t *testing.T) {
		t.Setenv(envVar, "otlp")

		_, shutdown, err := initLogger(
			t.Context(),
			testResource(),
			"test-service",
			otlploghttp.WithEndpoint("unused.invalid:4318"),
			otlploghttp.WithInsecure(),
			otlploghttp.WithTLSClientConfig(&tls.Config{MinVersion: tls.VersionTLS12}),
		)
		if err == nil {
			t.Fatal("expected exporter creation error")
		}
		if shutdown != nil {
			t.Fatal("expected nil shutdown on error")
		}
	})
}

func TestDefaultAttributeLevels(t *testing.T) {
	want := [][]string{
		{},
		{"source", "service", "address", "component", "trace_id"},
		{"span_id"},
		{"method"},
		{},
	}

	if !reflect.DeepEqual(DefaultAttributeLevels, want) {
		t.Fatalf("levels = %v, want %v", DefaultAttributeLevels, want)
	}
}
