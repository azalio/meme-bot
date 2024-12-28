package service

import (
	"context"
	"fmt"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotAPI interface defines the methods we need from telegram bot
// This abstraction allows us to mock the Telegram API for testing and decouples
// our service layer from the specific implementation of the Telegram API.
type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	StopReceivingUpdates()
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
}

// BotServiceImpl implements the core functionality of the Telegram bot.
// It follows the Service Layer pattern, encapsulating business logic related to the bot.
type BotServiceImpl struct {
	config         *config.Config          // Application configuration
	logger         *logger.Logger          // Logger for structured logging
	Bot            BotAPI                  // Abstraction of the Telegram API
	artService     ImageGenerator          // Service for generating images
	promptEnhancer *PromptEnhancer         // Service for enhancing prompts using GPT
	stopChan       chan struct{}           // Channel for graceful shutdown
	updateChan     tgbotapi.UpdatesChannel // Channel for receiving Telegram updates
}

// NewBotService creates a new instance of the bot service.
// It uses Dependency Injection to pass required dependencies (config, logger, auth, gpt).
// This approach makes the service more testable and flexible.
func NewBotService(
	cfg *config.Config,
	log *logger.Logger,
	auth YandexAuthService,
	gpt YandexGPTService,
) (*BotServiceImpl, error) {
	// Initialize the Telegram bot API
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	// Create ImageGenerationService that combines both FusionBrain and YandexArt
	imageService := NewImageGenerationService(cfg, log, auth, gpt)

	// Create PromptEnhancer service for improving user prompts
	promptEnhancer := NewPromptEnhancer(log, gpt)

	return &BotServiceImpl{
		config:         cfg,
		logger:         log,
		Bot:            bot,
		artService:     imageService,
		promptEnhancer: promptEnhancer,
		stopChan:       make(chan struct{}), // Initialize stop channel for graceful shutdown
	}, nil
}

// Stop safely shuts down the bot.
// It implements the Graceful Shutdown pattern by closing the stop channel
// and stopping the reception of updates from Telegram.
func (s *BotServiceImpl) Stop() {
	if s.stopChan != nil {
		close(s.stopChan)
	}
	if s.Bot != nil {
		s.Bot.StopReceivingUpdates()
	}
}

// GetUpdatesChan returns a channel for receiving updates from Telegram.
// This method follows the Observer pattern, allowing the bot to react to incoming messages.
func (s *BotServiceImpl) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	s.updateChan = s.Bot.GetUpdatesChan(config)
	return s.updateChan
}

// HandleCommand processes bot commands using the Command pattern.
// It currently supports the "meme" command, which generates an image based on the provided prompt.
func (s *BotServiceImpl) HandleCommand(ctx context.Context, command string, args string) ([]byte, error, string) {
	switch command {
	case "meme":
		// Use a default prompt if none is provided
		if args == "" {
			args = "Придумай и опиши какой-нибудь мем. Используй любые свои фантазии. Используй современные злободневные тренды. Пусть это будет немного философский мем."
		}

		// Enhance the prompt using GPT
		enhancedPrompt, caption, err := s.promptEnhancer.EnhancePrompt(ctx, args)
		if err != nil {
			s.logger.Error(ctx, "Failed to enhance prompt", map[string]interface{}{
				"error": err.Error(),
				"args":  args,
			})
			// Fallback to the original prompt in case of error
			enhancedPrompt = args
		}

		// Generate an image using the enhanced prompt
		image, err := s.artService.GenerateImage(ctx, enhancedPrompt)
		return image, err, caption
	default:
		return nil, fmt.Errorf("unknown command: %s", command), ""
	}
}

// SendMessage sends a text message to the specified chat.
// It encapsulates the Telegram API's message sending functionality.
func (s *BotServiceImpl) SendMessage(ctx context.Context, chatID int64, message string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, message)
	return s.Bot.Send(msg)
}

// SendPhoto sends an image to the specified chat.
// It includes validation for the photo data to prevent errors.
func (s *BotServiceImpl) SendPhoto(ctx context.Context, chatID int64, photo []byte, caption string) error {
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

	photoMsg.Caption = caption

	_, err := s.Bot.Send(photoMsg)
	if err != nil {
		return fmt.Errorf("failed to send photo: %w", err)
	}
	return nil
}

// DeleteMessage deletes a message by its ID.
// This method provides a clean interface for message deletion.
func (s *BotServiceImpl) DeleteMessage(ctx context.Context, chatID int64, messageID int) error {
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
	_, err := s.Bot.Request(deleteMsg)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}
