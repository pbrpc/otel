//revive:disable:package-comments
package otel

import (
	"net/http"
	"testing"

	"github.com/pbrpc/testing/mocks/roundtripper"
)

func TestNewTransport(t *testing.T) {
	t.Run("sends through the transport it wraps", func(t *testing.T) {
		base := roundtripper.Record(roundtripper.Respond(http.StatusNoContent, nil, ""))
		transport := NewTransport(base)

		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://service/", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		response, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		response.Body.Close()

		sent := base.Sent()
		if len(sent) != 1 || sent[0].Request.URL.Host != "service" {
			t.Errorf("sent = %v, want one request to service", sent)
		}
	})
}
