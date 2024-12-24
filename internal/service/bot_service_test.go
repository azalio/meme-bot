package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MockArtService имитирует сервис генерации изображений для тестирования
type MockArtService struct {
	mock.Mock
}

func (m *MockArtService) GenerateImage(ctx context.Context, promptText string) ([]byte, error) {
	args := m.Called(ctx, promptText)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

// MockBotAPI имитирует Telegram Bot API для тестирования
type MockBotAPI struct {
	mock.Mock
}

func (m *MockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

func (m *MockBotAPI) StopReceivingUpdates() {
	m.Called()
}

func (m *MockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.UpdatesChannel)
}

func TestNewBotService(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		TelegramToken: "test-token",
	}
	log := logger.New()
	artService := new(MockArtService)

	// Act
	svc, err := NewBotService(cfg, log, artService)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, cfg, svc.config)
	assert.NotNil(t, svc.logger)
	assert.Equal(t, artService, svc.artService)
	assert.NotNil(t, svc.stopChan)
}

func TestHandleCommand(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		args           string
		imageData      []byte
		expectedError  bool
	}{
		{
			name:           "successful meme generation",
			command:        "meme",
			args:           "test meme",
			imageData:      []byte("test image data"),
			expectedError:  false,
		},
		{
			name:           "empty args for meme",
			command:        "meme",
			args:           "",
			imageData:      []byte("default meme data"),
			expectedError:  false,
		},
		{
			name:           "unknown command",
			command:        "unknown",
			args:           "",
			imageData:      nil,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := &config.Config{TelegramToken: "test-token"}
			log := logger.New()
			artService := new(MockArtService)
			mockBot := new(MockBotAPI)

			if tt.command == "meme" {
				artService.On("GenerateImage", mock.Anything, mock.Anything).Return(tt.imageData, nil)
			}

			svc := &BotServiceImpl{
				config:     cfg,
				logger:     log,
				Bot:        mockBot,
				artService: artService,
				stopChan:   make(chan struct{}),
			}

			// Act
			result, err := svc.HandleCommand(context.Background(), tt.command, tt.args)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.imageData, result)
			}

			artService.AssertExpectations(t)
		})
	}
}

func TestSendMessage(t *testing.T) {
	tests := []struct {
		name          string
		chatID        int64
		message       string
		expectedError bool
		mockError     error
	}{
		{
			name:          "successful message send",
			chatID:        123,
			message:       "test message",
			expectedError: false,
			mockError:     nil,
		},
		{
			name:          "empty message",
			chatID:        123,
			message:       "",
			expectedError: false,
			mockError:     nil,
		},
		{
			name:          "send error",
			chatID:        -1,
			message:       "test message",
			expectedError: true,
			mockError:     fmt.Errorf("send error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := &config.Config{TelegramToken: "test-token"}
			log := logger.New()
			mockBot := new(MockBotAPI)
			artService := new(MockArtService)

			mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, tt.mockError)

			svc := &BotServiceImpl{
				config:     cfg,
				logger:     log,
				Bot:        mockBot,
				artService: artService,
				stopChan:   make(chan struct{}),
			}

			// Act
			err := svc.SendMessage(context.Background(), tt.chatID, tt.message)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockBot.AssertExpectations(t)
		})
	}
}

func TestSendPhoto(t *testing.T) {
	tests := []struct {
		name          string
		chatID        int64
		photo         []byte
		expectedError bool
		mockError     error
	}{
		{
			name:          "successful photo send",
			chatID:        123,
			photo:         []byte("test image data"),
			expectedError: false,
			mockError:     nil,
		},
		{
			name:          "empty photo data",
			chatID:        123,
			photo:         []byte{},
			expectedError: true,
			mockError:     fmt.Errorf("empty photo data"),
		},
		{
			name:          "send error",
			chatID:        -1,
			photo:         []byte("test image data"),
			expectedError: true,
			mockError:     fmt.Errorf("send error"),
		},
		{
			name:          "nil photo data",
			chatID:        123,
			photo:         nil,
			expectedError: true,
			mockError:     fmt.Errorf("nil photo data"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := &config.Config{TelegramToken: "test-token"}
			log := logger.New()
			mockBot := new(MockBotAPI)
			artService := new(MockArtService)

			mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, tt.mockError)

			svc := &BotServiceImpl{
				config:     cfg,
				logger:     log,
				Bot:        mockBot,
				artService: artService,
				stopChan:   make(chan struct{}),
			}

			// Act
			err := svc.SendPhoto(context.Background(), tt.chatID, tt.photo)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockBot.AssertExpectations(t)
		})
	}
}