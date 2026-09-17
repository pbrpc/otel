# otel

## Installation

```bash
go get github.com/pbrpc/otel
```

## Client

`NewHTTPClient` wraps a supplied `http.RoundTripper` with OpenTelemetry trace
propagation. A nil transport selects the standard cleartext HTTP/2 transport
configured the same way as
[http-transport](https://github.com/pbrpc/http-transport).

```go
client := otel.NewHTTPClient(nil)
```

An injected transport is placed beneath the tracing transport:

```go
client := httpclient.NewHTTPClient(roundTripper)
```
