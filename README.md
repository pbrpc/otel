# otel

## Installation

```bash
go get github.com/pbrpc/otel
```

## Transport

`NewTransport` wraps an `http.RoundTripper` with OpenTelemetry instrumentation:
a client span per request, and trace propagation on its headers. It goes
wherever in a transport chain the requests are addressed to the host they are
sent to, so the span names that host.

```go
base, err := transport.From(nil)
…
client := &http.Client{Transport: otel.NewTransport(base)}
```
