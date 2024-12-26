package metrics

import (
	"context"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// InitMetrics инициализирует метрики и экспортер Prometheus.
func InitMetrics() (*metric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	// Создаем ресурс с информацией о сервисе и SDK.
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("meme-bot"),             // Указываем имя сервиса
			semconv.TelemetrySDKLanguageGo,                        // Указываем язык SDK (Go)
			semconv.TelemetrySDKNameKey.String("opentelemetry"),   // Указываем имя SDK (opentelemetry)
			semconv.TelemetrySDKVersionKey.String(otel.Version()), // Указываем версию SDK
		),
	)
	if err != nil {
		return nil, err
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res), // Передаем ресурс в MeterProvider
	)

	otel.SetMeterProvider(provider)

	return provider, nil
}

// StartMetricsServer запускает HTTP сервер для экспорта метрик Prometheus.
func StartMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()
}
