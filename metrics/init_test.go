//revive:disable:package-comments
package metrics

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/pbrpc/connect-testing/mocks/roundtripper"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
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
	t.Run("otlp", func(t *testing.T) {
		t.Setenv(envVar, "otlp")

		shutdown, err := initMeter(
			t.Context(),
			testResource(),
			otlpmetrichttp.WithHTTPClient(testHTTPClient()),
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if shutdown == nil {
			t.Fatal("expected non-nil shutdown")
		}

		if err := shutdown(t.Context()); err != nil {
			t.Fatalf("shutdown error: %v", err)
		}
	})

	for _, value := range []string{"none", ""} {
		t.Run("disabled by "+value, func(t *testing.T) {
			t.Setenv(envVar, value)

			shutdown, err := Init(t.Context(), testResource())
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if shutdown == nil {
				t.Fatal("expected non-nil shutdown")
			}
			if err := shutdown(t.Context()); err != nil {
				t.Fatalf("expected nil error from noop shutdown, got %v", err)
			}
		})
	}

	t.Run("invalid", func(t *testing.T) {
		t.Setenv(envVar, "invalid")

		shutdown, err := Init(t.Context(), testResource())
		if err == nil {
			t.Fatal("expected error for unsupported exporter")
		}
		if shutdown != nil {
			t.Fatal("expected nil shutdown on error")
		}
	})

	t.Run("exporter creation failure", func(t *testing.T) {
		t.Setenv(envVar, "otlp")

		shutdown, err := initMeter(
			t.Context(),
			testResource(),
			otlpmetrichttp.WithEndpoint("unused.invalid:4318"),
			otlpmetrichttp.WithInsecure(),
			otlpmetrichttp.WithTLSClientConfig(&tls.Config{MinVersion: tls.VersionTLS12}),
		)
		if err == nil {
			t.Fatal("expected exporter creation error")
		}
		if shutdown != nil {
			t.Fatal("expected nil shutdown on error")
		}
	})
}
