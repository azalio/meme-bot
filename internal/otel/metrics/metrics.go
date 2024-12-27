// Package metrics предоставляет функционал для сбора и экспорта метрик приложения.
// Метрики позволяют отслеживать различные показатели работы бота, такие как
// количество выполненных команд, ошибок и время генерации мемов.
package metrics

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// MetricProvider представляет собой обертку над провайдером метрик OpenTelemetry.
// Он управляет созданием и настройкой метрик, а также их экспортом в Prometheus.
type MetricProvider struct {
	provider *sdkmetric.MeterProvider // провайдер метрик OpenTelemetry
	meter    metric.Meter             // инструмент для создания метрик
}

var (
	// CommandCounter подсчитывает количество выполненных команд разных типов.
	// Используется для анализа популярности различных команд бота.
	CommandCounter *Counter

	// ErrorCounter подсчитывает количество возникших ошибок по типам.
	// Помогает отслеживать надежность работы бота и выявлять проблемные места.
	ErrorCounter *Counter

	// GenerationDuration измеряет время, затраченное на генерацию мемов.
	// Помогает отслеживать производительность генерации мемов и выявлять аномалии.
	GenerationDuration *Histogram

	// once гарантирует, что инициализация метрик произойдет только один раз
	once sync.Once
)

// Counter представляет собой счетчик метрик.
// Счетчики используются для подсчета событий, например, количества вызовов команд
// или возникших ошибок. Значение счетчика может только увеличиваться.
type Counter struct {
	counter metric.Int64Counter
}

// Histogram представляет собой гистограмму метрик.
// Гистограммы используются для измерения распределения значений, например,
// времени выполнения операций. Они позволяют анализировать не только среднее
// значение, но и процентили (например, 95% запросов укладываются в определенное время).
type Histogram struct {
	histogram metric.Float64Histogram
}

// InitMetrics инициализирует систему метрик и настраивает экспорт в Prometheus.
// Эта функция должна быть вызвана при старте приложения, до использования любых метрик.
// Prometheus - это система мониторинга, которая будет собирать и хранить наши метрики.
func InitMetrics() (*MetricProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	// Создаем ресурс с информацией о сервисе и SDK
	// context.Background() используется здесь только потому что это требование API,
	// в данном случае контекст не используется для отмены или дедлайнов,
	// так как операция создания ресурса мгновенная и локальная
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("meme-bot"),
			semconv.TelemetrySDKLanguageGo,
			semconv.TelemetrySDKName("opentelemetry"),
			semconv.TelemetrySDKVersion(otel.Version()),
		),
	)
	if err != nil {
		return nil, err
	}

	// Паттерн Functional Options:
	// - Позволяет гибко конфигурировать объекты через функциональные опции
	// - WithReader и WithResource - это функции-опции, которые модифицируют базовую конфигурацию
	// - Можно добавлять новые опции без изменения сигнатуры конструктора
	// - Делает код более читаемым и поддерживаемым
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(provider)

	mp := &MetricProvider{
		provider: provider,
		meter:    provider.Meter("meme-bot"),
	}

	// Инициализируем метрики один раз
	once.Do(func() {
		var err error
		// Инициализация счетчика команд
		CommandCounter, err = mp.NewCounter(
			"meme_bot_commands_total",
			"Total number of commands processed by type",
		)
		if err != nil {
			log.Printf("Failed to create command counter: %v", err)
		}

		// Инициализация счетчика ошибок
		ErrorCounter, err = mp.NewCounter(
			"meme_bot_errors_total",
			"Total number of errors by type",
		)
		if err != nil {
			log.Printf("Failed to create error counter: %v", err)
		}

		// Инициализация гистограммы времени генерации
		GenerationDuration, err = mp.NewHistogram(
			"meme_bot_generation_duration_seconds",
			"Time taken to generate memes",
		)
		if err != nil {
			log.Printf("Failed to create generation duration histogram: %v", err)
		}
	})

	return mp, nil
}

// NewCounter создает новый счетчик с указанным именем и описанием.
// name - уникальное имя метрики в формате snake_case
// description - человекочитаемое описание того, что измеряет эта метрика
func (mp *MetricProvider) NewCounter(name, description string) (*Counter, error) {
	counter, err := mp.meter.Int64Counter(
		name,
		metric.WithDescription(description),
	)
	if err != nil {
		return nil, err
	}
	return &Counter{counter: counter}, nil
}

// NewHistogram создает новую гистограмму
func (mp *MetricProvider) NewHistogram(name, description string) (*Histogram, error) {
	histogram, err := mp.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
	)
	if err != nil {
		return nil, err
	}
	return &Histogram{histogram: histogram}, nil
}

// Inc увеличивает счетчик для определенного лейбла
func (c *Counter) Inc(label string) {
	if c == nil || c.counter == nil {
		return
	}
	c.counter.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("type", label)),
	)
}

// Observe записывает значение в гистограмму
func (h *Histogram) Observe(value float64) {
	if h == nil || h.histogram == nil {
		return
	}
	h.histogram.Record(context.Background(), value)
}

// StartMetricsServer запускает HTTP сервер для экспорта метрик Prometheus
func StartMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()
}

// Shutdown корректно завершает работу провайдера метрик, освобождая ресурсы.
// Должна вызываться при завершении работы приложения.
func (mp *MetricProvider) Shutdown(ctx context.Context) error {
	if mp.provider != nil {
		return mp.provider.Shutdown(ctx)
	}
	return nil
}
