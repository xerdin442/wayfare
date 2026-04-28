package tracing

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func NewHttpClient() *http.Client {
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}
