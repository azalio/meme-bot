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

	// FusionBrainSuccessCounter подсчитывает количество успешных генераций через FusionBrain.
	FusionBrainSuccessCounter *Counter

	// FusionBrainFailureCounter подсчитывает количество неуспешных генераций через FusionBrain.
	FusionBrainFailureCounter *Counter

	// YandexArtSuccessCounter подсчитывает количество успешных генераций через YandexArt.
	YandexArtSuccessCounter *Counter

	// YandexArtFailureCounter подсчитывает количество неуспешных генераций через YandexArt.
	YandexArtFailureCounter *Counter

	// Новые метрики
	CommandDuration      *Histogram
	PromptGenerationTime *Histogram
	APIResponseTime      *Histogram
	ActiveGoroutines     *Gauge
	MemoryUsage          *Gauge
	OpenHTTPConnections  *Gauge
	PromptQuality        *Histogram
	ImageQuality         *Histogram
	ActiveUsers          *Counter
	CommandFrequency     *Counter
	UserResponseTime     *Histogram
	APIErrors            *Counter
	MessageSendErrors    *Counter
	CommandErrors        *Counter
	ImageGenerationTime  *Histogram
	RequestsPerSecond    *Counter
	ServiceAvailability  *Gauge
	Downtime             *Counter
	UserSatisfaction     *Gauge
	ReturningUsers       *Counter
	UnauthorizedAccess   *Counter
	AuthErrors           *Counter
	CommandPopularity    *Counter
	RequestTrends        *Counter

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

type Gauge struct {
	gauge metric.Float64ObservableGauge
	value float64
	mu    sync.Mutex
}

func (mp *MetricProvider) NewGauge(name, description string) (*Gauge, error) {
	gauge, err := mp.meter.Float64ObservableGauge(
		name,
		metric.WithDescription(description),
	)
	if err != nil {
		return nil, err
	}
	return &Gauge{gauge: gauge}, nil
}

func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = value
}

func (g *Gauge) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

func (g *Gauge) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
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

		// Инициализация счетчиков для FusionBrain
		FusionBrainSuccessCounter, err = mp.NewCounter(
			"meme_bot_fusionbrain_success_total",
			"Total number of successful image generations via FusionBrain",
		)
		if err != nil {
			log.Printf("Failed to create FusionBrain success counter: %v", err)
		}

		FusionBrainFailureCounter, err = mp.NewCounter(
			"meme_bot_fusionbrain_failure_total",
			"Total number of failed image generations via FusionBrain",
		)
		if err != nil {
			log.Printf("Failed to create FusionBrain failure counter: %v", err)
		}

		// Инициализация счетчиков для YandexArt
		YandexArtSuccessCounter, err = mp.NewCounter(
			"meme_bot_yandexart_success_total",
			"Total number of successful image generations via YandexArt",
		)
		if err != nil {
			log.Printf("Failed to create YandexArt success counter: %v", err)
		}

		YandexArtFailureCounter, err = mp.NewCounter(
			"meme_bot_yandexart_failure_total",
			"Total number of failed image generations via YandexArt",
		)
		if err != nil {
			log.Printf("Failed to create YandexArt failure counter: %v", err)
		}

		// Инициализация новых метрик
		CommandDuration, err = mp.NewHistogram(
			"meme_bot_command_duration_seconds",
			"Time taken to process commands",
		)
		if err != nil {
			log.Printf("Failed to create command duration histogram: %v", err)
		}

		PromptGenerationTime, err = mp.NewHistogram(
			"meme_bot_prompt_generation_duration_seconds",
			"Time taken to generate enhanced prompts",
		)
		if err != nil {
			log.Printf("Failed to create prompt generation time histogram: %v", err)
		}

		APIResponseTime, err = mp.NewHistogram(
			"meme_bot_api_response_duration_seconds",
			"Time taken to get responses from external APIs",
			metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10),
		)
		if err != nil {
			log.Printf("Failed to create API response time histogram: %v", err)
		}

		ActiveGoroutines, err = mp.NewGauge(
			"meme_bot_active_goroutines",
			"Number of active goroutines",
		)
		if err != nil {
			log.Printf("Failed to create active goroutines gauge: %v", err)
		}

		MemoryUsage, err = mp.NewGauge(
			"meme_bot_memory_usage_bytes",
			"Current memory usage of the bot",
		)
		if err != nil {
			log.Printf("Failed to create memory usage gauge: %v", err)
		}

		OpenHTTPConnections, err = mp.NewGauge(
			"meme_bot_open_http_connections",
			"Number of open HTTP connections",
		)
		if err != nil {
			log.Printf("Failed to create open HTTP connections gauge: %v", err)
		}

		// Добавьте инициализацию остальных метрик по аналогии...
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
func (mp *MetricProvider) NewHistogram(name, description string, opts ...metric.Float64HistogramOption) (*Histogram, error) {
	histogram, err := mp.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		opts...,
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

// Observe записывает значение в гистограмму с лейблами
func (h *Histogram) Observe(value float64, labels ...attribute.KeyValue) {
	if h == nil || h.histogram == nil {
		return
	}
	h.histogram.Record(context.Background(), value, metric.WithAttributes(labels...))
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
