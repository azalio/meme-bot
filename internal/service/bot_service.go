package service

import (
    "context"
    "fmt"
    
    "github.com/azalio/meme-bot/internal/config"
    "github.com/azalio/meme-bot/pkg/logger"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotServiceImpl реализует основной функционал Telegram бота
// Структура содержит:
// - config: конфигурация приложения (токены, настройки)
// - logger: логгер для записи событий и ошибок
// - Bot: экземпляр Telegram API клиента
// - artService: сервис для генерации изображений
// - stopChan: канал для сигнала остановки бота
// - updateChan: канал для получения обновлений от Telegram
type BotServiceImpl struct {
    config     *config.Config
    logger     *logger.Logger
    Bot        *tgbotapi.BotAPI
    artService YandexArtService
    stopChan   chan struct{}
    updateChan tgbotapi.UpdatesChannel
}

// NewBotService создает новый экземпляр сервиса бота
// Параметры:
// - cfg: конфигурация приложения
// - log: логгер для записи событий
// - art: сервис генерации изображений
// Возвращает:
// - *BotServiceImpl: настроенный экземпляр бота
// - error: ошибку в случае проблем с инициализацией
func NewBotService(
    cfg *config.Config,
    log *logger.Logger,
    art YandexArtService,
) (*BotServiceImpl, error) {
    bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
    if err != nil {
        return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
    }

    return &BotServiceImpl{
        config:     cfg,
        logger:     log,
        Bot:        bot,
        artService: art,
        stopChan:   make(chan struct{}),
    }, nil
}

// Stop безопасно останавливает работу бота
// Закрывает канал остановки и прекращает получение обновлений
// Метод идемпотентен - может быть вызван многократно
func (s *BotServiceImpl) Stop() {
    if s.stopChan != nil {
        close(s.stopChan)
    }
    if s.Bot != nil {
        s.Bot.StopReceivingUpdates()
    }
}

// GetUpdatesChan возвращает канал для получения обновлений от Telegram
// Параметры:
// - config: конфигурация обновлений (таймаут, offset и т.д.)
// Возвращает:
// - канал, через который будут приходить обновления
func (s *BotServiceImpl) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
    s.updateChan = s.Bot.GetUpdatesChan(config)
    return s.updateChan
}

// HandleCommand обрабатывает команды бота
// Параметры:
// - ctx: контекст для отмены операции
// - command: название команды (например, "meme")
// - args: аргументы команды (текст после команды)
// Возвращает:
// - []byte: сгенерированное изображение для команды meme
// - error: ошибку в случае проблем с обработкой команды
func (s *BotServiceImpl) HandleCommand(ctx context.Context, command string, args string) ([]byte, error) {
    switch command {
    case "meme":
        if args == "" {
            args = "нарисуй смешного кота в стиле мема"
        }
        return s.artService.GenerateImage(ctx, args)
    default:
        return nil, fmt.Errorf("unknown command: %s", command)
    }
}

// SendMessage отправляет текстовое сообщение в указанный чат
// Параметры:
// - ctx: контекст для отмены операции
// - chatID: идентификатор чата для отправки
// - message: текст сообщения
// Возвращает:
// - error: ошибку в случае проблем с отправкой
func (s *BotServiceImpl) SendMessage(ctx context.Context, chatID int64, message string) error {
    msg := tgbotapi.NewMessage(chatID, message)
    _, err := s.Bot.Send(msg)
    if err != nil {
        return fmt.Errorf("failed to send message: %w", err)
    }
    return nil
}

// SendPhoto отправляет изображение в указанный чат
// Параметры:
// - ctx: контекст для отмены операции
// - chatID: идентификатор чата для отправки
// - photo: байты изображения для отправки
// Возвращает:
// - error: ошибку в случае проблем с отправкой
func (s *BotServiceImpl) SendPhoto(ctx context.Context, chatID int64, photo []byte) error {
    photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
        Name:  "meme.png",
        Bytes: photo,
    })
    
    _, err := s.Bot.Send(photoMsg)
    if err != nil {
        return fmt.Errorf("failed to send photo: %w", err)
    }
    return nil
}