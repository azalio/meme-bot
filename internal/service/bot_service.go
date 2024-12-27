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
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	StopReceivingUpdates()
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
}

// BotServiceImpl реализует основной функционал Telegram бота
type BotServiceImpl struct {
	config         *config.Config
	logger         *logger.Logger
	Bot            BotAPI
	artService     ImageGenerator
	promptEnhancer *PromptEnhancer
	stopChan       chan struct{}
	updateChan     tgbotapi.UpdatesChannel
}

// NewBotService создает новый экземпляр сервиса бота
func NewBotService(
	cfg *config.Config,
	log *logger.Logger,
	auth YandexAuthService,
	gpt YandexGPTService,
) (*BotServiceImpl, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	// Create ImageGenerationService that combines both FusionBrain and YandexArt
	imageService := NewImageGenerationService(cfg, log, auth, gpt)

	// Create PromptEnhancer service
	promptEnhancer := NewPromptEnhancer(log, gpt)

	return &BotServiceImpl{
		config:         cfg,
		logger:         log,
		Bot:            bot,
		artService:     imageService,
		promptEnhancer: promptEnhancer,
		stopChan:       make(chan struct{}),
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
			args = "Придумай и опиши какой-нибудь мем как будто это картинка"
		}

		// Сначала улучшаем промпт через GPT
		enhancedPrompt, err := s.promptEnhancer.EnhancePrompt(ctx, args)
		if err != nil {
			s.logger.Error(ctx, "Failed to enhance prompt", map[string]interface{}{
				"error": err.Error(),
				"args":  args,
			})
			// В случае ошибки используем оригинальный промпт
			enhancedPrompt = args
		}

		// Генерируем изображение с улучшенным промптом
		return s.artService.GenerateImage(ctx, enhancedPrompt)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// SendMessage отправляет текстовое сообщение в указанный чат и возвращает отправленное сообщение
func (s *BotServiceImpl) SendMessage(ctx context.Context, chatID int64, message string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, message)
	return s.Bot.Send(msg)
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

// DeleteMessage удаляет сообщение по его ID
func (s *BotServiceImpl) DeleteMessage(ctx context.Context, chatID int64, messageID int) error {
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
	_, err := s.Bot.Request(deleteMsg)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}
