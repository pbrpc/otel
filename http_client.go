//revive:disable:package-comments
package otel

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewTransport wraps base with OpenTelemetry instrumentation: a client span
// per request, and trace propagation on its headers. It sits wherever in a
// transport chain the requests are addressed to the host they go to, so the
// span names that host.
func NewTransport(base http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(base)
}
