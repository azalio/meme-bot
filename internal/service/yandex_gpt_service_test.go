package service

import (
	"context"
	"testing"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService - мок для сервиса аутентификации
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) GetIAMToken(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) RefreshIAMToken(ctx context.Context, oauthToken string) (string, error) {
	args := m.Called(ctx, oauthToken)
	return args.String(0), args.Error(1)
}

func TestNewYandexGPTService(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		YandexArtFolderID: "test-folder",
	}
	log := logger.New()
	authService := new(MockAuthService)

	// Act
	svc := NewYandexGPTService(cfg, log, authService)

	// Assert
	assert.NotNil(t, svc)
	assert.Equal(t, cfg, svc.config)
	assert.NotNil(t, svc.logger)
	assert.Equal(t, authService, svc.authService)
}

func TestGenerateImagePrompt(t *testing.T) {
	tests := []struct {
		name           string
		userPrompt     string
		iamToken       string
		refreshToken   string
		expectedPrompt string
		expectedError  bool
		mockError      error
	}{
		{
			name:           "successful prompt generation",
			userPrompt:     "test prompt",
			iamToken:       "test-token",
			expectedPrompt: "test prompt", // В случае ошибки возвращается исходный промпт
			expectedError:  false,
		},
		{
			name:           "handles empty prompt",
			userPrompt:     "",
			iamToken:       "test-token",
			expectedPrompt: "",
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := &config.Config{YandexArtFolderID: "test-folder"}
			log := logger.New()
			authService := new(MockAuthService)
			authService.On("GetIAMToken", mock.Anything).Return(tt.iamToken, tt.mockError)
			if tt.mockError != nil {
				authService.On("RefreshIAMToken", mock.Anything, mock.Anything).Return(tt.refreshToken, nil)
			}

			svc := NewYandexGPTService(cfg, log, authService)

			// Act
			prompt, err := svc.GenerateImagePrompt(context.Background(), tt.userPrompt)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPrompt, prompt)
			}

			authService.AssertExpectations(t)
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		maxLength  int
		expected   string
	}{
		{
			name:      "text shorter than max length",
			text:      "Short text.",
			maxLength: 20,
			expected:  "Short text.",
		},
		{
			name:      "text longer than max length with dot",
			text:      "This is a long text. With multiple sentences.",
			maxLength: 20,
			expected:  "This is a long text.",
		},
		{
			name:      "text longer than max length without dot",
			text:      "ThisIsAVeryLongTextWithoutAnyDots",
			maxLength: 10,
			expected:  "ThisIsAVer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := truncateText(tt.text, tt.maxLength)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}
