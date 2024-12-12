# Meme Bot

Telegram бот для генерации мемов с использованием Yandex GPT и Yandex Art API.

## Описание

Бот использует Yandex GPT для генерации описания мема на основе пользовательского ввода и Yandex Art для создания изображения. Проект построен с использованием принципов Clean Architecture и современных паттернов разработки.

## Архитектурные паттерны

### Dependency Injection

В проекте используется внедрение зависимостей для уменьшения связанности компонентов. Пример:

```go
// Service factory с внедрением зависимостей
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

### Interface Segregation

Сервисы определяются через интерфейсы для лучшей абстракции:

```go
type BotService interface {
    GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
    HandleCommand(ctx context.Context, command string, args string) ([]byte, error)
    SendMessage(ctx context.Context, chatID int64, message string) error
    SendPhoto(ctx context.Context, chatID int64, photo []byte) error
    Stop()
}

type YandexAuthService interface {
    GetIAMToken(ctx context.Context) (string, error)
}
```

## Требования

- Go 1.23.2 или выше
- Telegram Bot Token
- Yandex OAuth Token
- Yandex Cloud Folder ID

## Установка

1. Клонировать репозиторий:
```bash
git clone https://github.com/azalio/meme-bot.git
cd meme-bot
```

2. Создать файл .env со следующим содержимым:
```env
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
YANDEX_OAUTH_TOKEN=your_yandex_oauth_token
YANDEX_ART_FOLDER_ID=your_folder_id
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
│   └── main.go           # Точка входа в приложение, инициализация DI
├── internal/
│   ├── config/          # Конфигурация приложения
│   │   └── config.go    # Загрузка и валидация конфигурации
│   ├── middleware/      # Middleware для логирования и обработки ошибок
│   │   ├── logger.go    # Middleware для логирования запросов
│   │   └── recover.go   # Middleware для обработки паник
│   └── service/         # Бизнес-логика и сервисы
│       ├── interfaces.go # Определения интерфейсов сервисов
│       ├── bot.go       # Реализация Telegram бота
│       ├── yandex_auth.go # Сервис аутентификации Yandex
│       ├── yandex_gpt.go  # Сервис работы с YandexGPT
│       └── yandex_art.go  # Сервис генерации изображений
├── pkg/
│   └── logger/          # Пакет для логирования
│       └── logger.go    # Thread-safe логгер с уровнями
├── .env                 # Конфигурационные переменные
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

### Взаимодействие компонентов

1. **Config** загружает конфигурацию из переменных окружения и проверяет корректность.

2. **Logger** обеспечивает потокобезопасное логирование с разными уровнями (INFO, ERROR, DEBUG).

3. **Service Layer**:
   - **AuthService**: Управляет IAM токенами для Yandex Cloud
   - **GPTService**: Улучшает пользовательские промпты через YandexGPT
   - **ArtService**: Генерирует изображения используя Yandex Art API
   - **BotService**: Обрабатывает команды пользователя и координирует другие сервисы

## Архитектурные решения

### Factory Method Pattern

Создание сервисов происходит через factory методы:

```go
// Фабричный метод для создания сервиса аутентификации
func NewYandexAuthService(cfg *config.Config, log *logger.Logger) *YandexAuthServiceImpl {
    service := &YandexAuthServiceImpl{
        config: cfg,
        logger: log,
    }
    go service.refreshTokenPeriodically()
    return service
}
```

### Service Layer Pattern

Бизнес-логика инкапсулирована в сервисах:

```go
type YandexGPTServiceImpl struct {
    config      *config.Config
    logger      *logger.Logger
    authService YandexAuthService
}

func (s *YandexGPTServiceImpl) GenerateImagePrompt(ctx context.Context, userPrompt string) (string, error) {
    // Бизнес-логика генерации промпта
}
```

### Error Handling Patterns

1. **Error Wrapping**:
```go
if err != nil {
    return nil, fmt.Errorf("failed to generate image: %w", err)
}
```

2. **Graceful Degradation**:
```go
enhancedPrompt, err := s.gptService.GenerateImagePrompt(ctx, promptText)
if err != nil {
    s.logger.Error("Failed to generate enhanced prompt: %v, using original prompt", err)
    enhancedPrompt = promptText
}
```

3. **Context Cancellation**:
```go
select {
case <-ctx.Done():
    return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
case <-ticker.C:
    // Продолжаем обработку
}
```

### Рекомендации по доработке

1. **Тестирование**:
   - Добавить unit-тесты для сервисов
   - Реализовать integration тесты с моками API
   - Добавить benchmark тесты для критичных операций

2. **Мониторинг**:
   - Добавить метрики Prometheus
   - Реализовать health checks
   - Настроить трейсинг запросов

3. **Оптимизация**:
   - Кэширование результатов генерации
   - Пулл соединений для HTTP клиентов
   - Rate limiting для API запросов

## Вклад в разработку

Приветствуются любые предложения по улучшению проекта через Issues и Pull Requests.

## Лицензия

MIT