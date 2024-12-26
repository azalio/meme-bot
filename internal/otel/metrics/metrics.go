package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

// initMetrics инициализирует метрики и экспортер Prometheus.
func InitMetrics() (*metric.MeterProvider, error) {

	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
	)

	otel.SetMeterProvider(provider)

	return provider, nil
}

// startMetricsServer запускает HTTP сервер для экспорта метрик Prometheus.
func StartMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()
}
