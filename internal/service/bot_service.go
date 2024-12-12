package service

import (
    "context"
    "fmt"
    
    "github.com/azalio/meme-bot/internal/config"
    "github.com/azalio/meme-bot/pkg/logger"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotServiceImpl struct {
    config     *config.Config
    logger     *logger.Logger
    Bot        *tgbotapi.BotAPI
    artService YandexArtService
    stopChan   chan struct{}
    updateChan tgbotapi.UpdatesChannel
}

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

func (s *BotServiceImpl) Stop() {
    if s.stopChan != nil {
        close(s.stopChan)
    }
    if s.Bot != nil {
        s.Bot.StopReceivingUpdates()
    }
}

func (s *BotServiceImpl) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
    s.updateChan = s.Bot.GetUpdatesChan(config)
    return s.updateChan
}

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

func (s *BotServiceImpl) SendMessage(ctx context.Context, chatID int64, message string) error {
    msg := tgbotapi.NewMessage(chatID, message)
    _, err := s.Bot.Send(msg)
    if err != nil {
        return fmt.Errorf("failed to send message: %w", err)
    }
    return nil
}

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