//revive:disable:package-comments
package otel

import (
	"net/http"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/pbrpc/testing/mocks/roundtripper"
)

func TestFromEnv(t *testing.T) {
	t.Run("builds the configured standard client", func(t *testing.T) {
		client := NewHTTPClient(nil)
		if _, ok := client.Transport.(*otelhttp.Transport); !ok {
			t.Fatalf("transport = %T, want the tracing transport", client.Transport)
		}
	})

	t.Run("wraps an injected transport", func(t *testing.T) {
		t.Setenv("HTTP2_SEND_PING_TIMEOUT", "not-a-duration")

		base := roundtripper.Record(roundtripper.Respond(http.StatusNoContent, nil, ""))
		client := NewHTTPClient(base)

		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://service/", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err = client.Do(request); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sent := base.Sent()
		if len(sent) != 1 || sent[0].Request.URL.Host != "service" {
			t.Errorf("sent = %v, want one request to service", sent)
		}
	})
}
