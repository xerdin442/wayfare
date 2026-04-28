package metrics

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	metricapi "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type MetricConfig struct {
	ServiceName string
	Environment string
}

func InitMetrics(ctx context.Context, cfg *MetricConfig) (func(context.Context) error, error) {
	// Exporter
	metricExporter, err := NewMetricExporter()
	if err != nil {
		return nil, err
	}

	// Provider
	metricProvider, err := newMetricProvider(cfg, metricExporter)
	if err != nil {
		return nil, err
	}
	otel.SetMeterProvider(metricProvider)

	return metricProvider.Shutdown, nil
}

func GetMeter(name string) metricapi.Meter {
	return otel.GetMeterProvider().Meter(name)
}

func NewMetricExporter() (*prometheus.Exporter, error) {
	return prometheus.New(
		prometheus.WithNamespace("wayfare"),
	)
}

func GetMetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func newMetricProvider(cfg *MetricConfig, exporter metric.Reader) (*metric.MeterProvider, error) {
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	metricProvider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)

	return metricProvider, nil
}
