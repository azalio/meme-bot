package service

import (
	"context"
	"testing"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGPTService имитирует интерфейс GPT сервиса для тестирования
type MockGPTService struct {
	mock.Mock
}

func (m *MockGPTService) GenerateImagePrompt(ctx context.Context, userPrompt string) (string, error) {
	args := m.Called(ctx, userPrompt)
	if userPrompt == "" {
		return "нарисуй смешного кота в стиле мема", nil
	}
	return args.String(0), args.Error(1)
}

func TestNewYandexArtService(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		YandexArtFolderID: "test-folder",
	}
	log := logger.New()
	authService := new(MockAuthService)
	gptService := new(MockGPTService)

	// Act
	svc := NewYandexArtService(cfg, log, authService, gptService)

	// Assert
	assert.NotNil(t, svc)
	assert.Equal(t, cfg, svc.config)
	assert.NotNil(t, svc.logger)
	assert.Equal(t, authService, svc.authService)
	assert.Equal(t, gptService, svc.gptService)
}

func TestGenerateImage(t *testing.T) {
	tests := []struct {
		name           string
		promptText     string
		enhancedPrompt string
		iamToken       string
		mockError      error
		expectedError  bool
		expectedBytes  []byte
	}{
		{
			name:           "successful image generation",
			promptText:     "test prompt",
			enhancedPrompt: "enhanced test prompt",
			iamToken:       "test-token",
			mockError:      nil,
			expectedError:  false,
			expectedBytes:  []byte("test image data"),
		},
		{
			name:           "handles empty prompt",
			promptText:     "",
			enhancedPrompt: "default prompt",
			iamToken:       "test-token",
			mockError:      nil,
			expectedError:  false,
			expectedBytes:  []byte("default image data"),
		},
		{
			name:           "handles auth error",
			promptText:     "test prompt",
			enhancedPrompt: "",
			iamToken:       "",
			mockError:      fmt.Errorf("auth error"),
			expectedError:  true,
			expectedBytes:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := &config.Config{YandexArtFolderID: "test-folder"}
			log := logger.New()
			authService := new(MockAuthService)
			gptService := new(MockGPTService)

			authService.On("GetIAMToken", mock.Anything).Return(tt.iamToken, tt.mockError)
			if tt.mockError == nil {
				authService.On("RefreshIAMToken", mock.Anything, mock.Anything).Return(tt.iamToken, nil)
				gptService.On("GenerateImagePrompt", mock.Anything, tt.promptText).Return(tt.enhancedPrompt, nil)
			}

			svc := NewYandexArtService(cfg, log, authService, gptService)

			// Act
			result, err := svc.GenerateImage(context.Background(), tt.promptText)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				if tt.expectedBytes != nil {
					assert.NotNil(t, result)
				}
			}

			authService.AssertExpectations(t)
			if tt.mockError == nil {
				gptService.AssertExpectations(t)
			}
		})
	}
}
