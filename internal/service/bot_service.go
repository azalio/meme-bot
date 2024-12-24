package service

import (
    "context"
    "fmt"
    
    "github.com/azalio/meme-bot/internal/config"
    "github.com/azalio/meme-bot/pkg/logger"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotAPI interface defines the methods we need from telegram bot
type BotAPI interface {
    Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
    StopReceivingUpdates()
    GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
}

// BotServiceImpl реализует основной функционал Telegram бота
type BotServiceImpl struct {
    config     *config.Config
    logger     *logger.Logger
    Bot        BotAPI
    artService YandexArtService
    stopChan   chan struct{}
    updateChan tgbotapi.UpdatesChannel
}

// NewBotService создает новый экземпляр сервиса бота
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
func (s *BotServiceImpl) Stop() {
    if s.stopChan != nil {
        close(s.stopChan)
    }
    if s.Bot != nil {
        s.Bot.StopReceivingUpdates()
    }
}

// GetUpdatesChan возвращает канал для получения обновлений от Telegram
func (s *BotServiceImpl) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
    s.updateChan = s.Bot.GetUpdatesChan(config)
    return s.updateChan
}

// HandleCommand обрабатывает команды бота
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
func (s *BotServiceImpl) SendMessage(ctx context.Context, chatID int64, message string) error {
    msg := tgbotapi.NewMessage(chatID, message)
    _, err := s.Bot.Send(msg)
    if err != nil {
        return fmt.Errorf("failed to send message: %w", err)
    }
    return nil
}

// SendPhoto отправляет изображение в указанный чат
func (s *BotServiceImpl) SendPhoto(ctx context.Context, chatID int64, photo []byte) error {
    if photo == nil {
        return fmt.Errorf("nil photo data")
    }
    if len(photo) == 0 {
        return fmt.Errorf("empty photo data")
    }
    
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