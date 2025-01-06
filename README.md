# Meme Bot

Telegram бот для генерации мемов с использованием Yandex GPT, Yandex Art API и Fusion Brain API.

[![Build, Push, and Deploy meme-bot](https://github.com/azalio/meme-bot/actions/workflows/deploy.yml/badge.svg)](https://github.com/azalio/meme-bot/actions/workflows/deploy.yml)

## Описание

Этот проект представляет собой Telegram бота, который генерирует мемы на основе текста, введенного пользователем. Бот использует Yandex GPT для создания описания мема и два независимых сервиса (Yandex Art API и Fusion Brain API) для генерации изображения. Проект построен с использованием принципов Clean Architecture и современных паттернов разработки.

## Как это работает?

1. **Пользователь отправляет команду `/meme [текст]`** в Telegram.
2. Бот отправляет запрос в Yandex GPT, чтобы улучшить текст и создать описание для мема.
3. Бот **параллельно** использует два сервиса для генерации изображения:
   - **Yandex Art API**
   - **Fusion Brain API**
4. Как только один из сервисов успешно генерирует изображение, оно отправляется пользователю.
5. Бот также собирает метрики о работе сервисов (успешные и неудачные попытки генерации).

## Архитектурные паттерны

### 1. **Clean Architecture**
Проект разделен на слои:
- **Внешний слой**: Взаимодействие с внешними API (Telegram, Yandex GPT, Yandex Art, Fusion Brain).
- **Слой бизнес-логики**: Обработка команд, генерация мемов.
- **Слой данных**: Конфигурация и переменные окружения.

### 2. **Dependency Injection**
Зависимости (например, сервисы для работы с API) передаются в конструкторы, что делает код более тестируемым и гибким.

Пример:
```go
func NewBotService(cfg *config.Config, log *logger.Logger, art YandexArtService) (*BotServiceImpl, error) {
    bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
    if err != nil {
        return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
    }
    return &BotServiceImpl{
        config:     cfg,
        logger:     log,
        artService: art,
        stopChan:   make(chan struct{}),
    }, nil
}
```

### 3. **Interface Segregation**
Сервисы определяются через интерфейсы, что позволяет легко заменять реализации.

Пример:
```go
type BotService interface {
    GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
    HandleCommand(ctx context.Context, command string, args string) ([]byte, error)
    SendMessage(ctx context.Context, chatID int64, message string) error
    SendPhoto(ctx context.Context, chatID int64, photo []byte) error
    Stop()
}
```

### 4. **Factory Method**
Создание сервисов происходит через фабричные методы, что упрощает управление зависимостями.

Пример:
```go
func NewYandexAuthService(cfg *config.Config, log *logger.Logger) *YandexAuthServiceImpl {
    service := &YandexAuthServiceImpl{
        config: cfg,
        logger: log,
    }
    go service.refreshTokenPeriodically()
    return service
}
```

### 5. **Graceful Shutdown**
Приложение корректно завершает работу при получении сигнала завершения (например, `SIGINT` или `SIGTERM`).

Пример:
```go
func (a *App) shutdown(ctx context.Context) {
    a.log.Info(ctx, "Starting graceful shutdown", nil)
    a.bot.Stop()
    a.wg.Wait()
}
```

### 6. **Worker Pool**
Для обработки команд используется пул горутин, чтобы избежать перегрузки системы.

Пример:
```go
workerPool := make(chan struct{}, workerPoolSize)
go func(update tgbotapi.Update) {
    workerPool <- struct{}{}
    defer func() { <-workerPool }()
    // Обработка команды
}(update)
```

### 7. **Parallel Execution**
Для генерации изображений используются два сервиса (Yandex Art и Fusion Brain), которые работают параллельно. Как только один из них успешно завершает генерацию, результат возвращается пользователю.

Пример:
```go
func (s *ImageGenerationService) GenerateImage(ctx context.Context, promptText string) ([]byte, error) {
    resultChan := make(chan []byte)
    errorChan := make(chan error)

    // Запускаем генерацию в двух горутинах
    go func() {
        imageData, err := s.fusionBrain.GenerateImage(ctx, promptText)
        if err == nil {
            resultChan <- imageData
            return
        }
        errorChan <- err
    }()

    go func() {
        imageData, err := s.yandexArt.GenerateImage(ctx, promptText)
        if err == nil {
            resultChan <- imageData
            return
        }
        errorChan <- err
    }()

    // Ожидаем первый успешный результат или все ошибки
    var errors []error
    for i := 0; i < 2; i++ {
        select {
        case imageData := <-resultChan:
            return imageData, nil
        case err := <-errorChan:
            errors = append(errors, err)
            if len(errors) == 2 {
                return nil, fmt.Errorf("all image generation services failed: %w", errors[0])
            }
        }
    }

    return nil, fmt.Errorf("unexpected error: no results received")
}
```

### 8. **Metrics Collection**
Проект собирает метрики о работе сервисов (успешные и неудачные попытки генерации) с использованием Prometheus.

Пример:
```go
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
```

## Требования

- Go 1.21 или выше
- Telegram Bot Token
- Yandex OAuth Token
- Yandex Cloud Folder ID
- Fusion Brain API Key

## Установка

1. Клонировать репозиторий:
```bash
git clone https://github.com/azalio/meme-bot.git
cd meme-bot
```

2. Создать файл `.env` со следующим содержимым:
```env
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
YANDEX_OAUTH_TOKEN=your_yandex_oauth_token
YANDEX_ART_FOLDER_ID=your_folder_id
FUSION_BRAIN_API_KEY=your_fusion_brain_api_key
FUSION_BRAIN_SECRET_KEY=your_fusion_brain_secret_key
```

3. Установить зависимости:
```bash
go mod download
```

4. Собрать проект:
```bash
go build -o meme-bot cmd/main.go
```

## Использование

1. Запустить бота:
```bash
./meme-bot
```

2. В Telegram использовать следующие команды:
- `/start` - Начать работу с ботом
- `/help` - Показать справку
- `/meme [текст]` - Сгенерировать мем с описанием

## Структура проекта

```
.
├── cmd/
│   └── main.go           # Точка входа в приложение
├── internal/
│   ├── config/           # Конфигурация приложения
│   ├── service/          # Бизнес-логика и сервисы
│   └── otel/             # Инструменты для мониторинга
├── pkg/
│   └── logger/           # Логирование
├── cloudflare/           # Cloudflare Workers
│   └── index.js          # Скрипт для генерации изображений через AI
├── charts/               # Kubernetes Helm-чарты
├── .env                  # Конфигурационные переменные
├── Dockerfile            # Конфигурация Docker
├── go.mod
├── go.sum
└── README.md
```

## Рекомендации по доработке

1. **Тестирование**:
   - Добавить unit-тесты для сервисов.
   - Реализовать integration тесты с моками API.

2. **Мониторинг**:
   - Добавить метрики Prometheus.
   - Настроить трейсинг запросов.

3. **Оптимизация**:
   - Кэширование результатов генерации.
   - Пулл соединений для HTTP клиентов.

## Вклад в разработку

Приветствуются любые предложения по улучшению проекта через Issues и Pull Requests.

## Лицензия

MIT
