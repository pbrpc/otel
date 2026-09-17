//revive:disable:package-comments
package otel

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewHTTPClient creates an HTTP client with OpenTelemetry trace propagation.
func NewHTTPClient(base http.RoundTripper) *http.Client {
	return &http.Client{Transport: otelhttp.NewTransport(base)}
}
