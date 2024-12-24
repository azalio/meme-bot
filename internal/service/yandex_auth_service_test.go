package service

import (
	"context"
	"testing"
	"time"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLogger имитирует интерфейс логгера для тестирования
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(format string, v ...interface{})   { m.Called(format, v) }
func (m *MockLogger) Error(format string, v ...interface{})  { m.Called(format, v) }
func (m *MockLogger) Debug(format string, v ...interface{})  { m.Called(format, v) }
func (m *MockLogger) RefreshIAMToken(ctx context.Context, oauthToken string) (string, error) {
	args := m.Called(ctx, oauthToken)
	return args.String(0), args.Error(1)
}

func TestNewYandexAuthService(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		YandexOAuthToken: "test-token",
	}
	log := logger.New()

	// Act
	svc := NewYandexAuthService(cfg, log)

	// Assert
	assert.NotNil(t, svc)
	assert.Equal(t, cfg, svc.config)
	assert.NotNil(t, svc.logger)
}

func TestGetIAMToken(t *testing.T) {
	tests := []struct {
		name          string
		cachedToken   string
		mockToken     string
		mockError     error
		expectedToken string
		expectedError bool
	}{
		{
			name:          "returns cached token",
			cachedToken:   "cached-token",
			mockToken:     "",
			mockError:     nil,
			expectedToken: "cached-token",
			expectedError: false,
		},
		{
			name:          "refreshes when no cached token",
			cachedToken:   "",
			mockToken:     "new-token",
			mockError:     nil,
			expectedToken: "new-token",
			expectedError: false,
		},
		{
			name:          "handles refresh error",
			cachedToken:   "",
			mockToken:     "",
			mockError:     fmt.Errorf("refresh error"),
			expectedToken: "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := &config.Config{YandexOAuthToken: "test-token"}
			mockLogger := new(MockLogger)
			mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
			mockLogger.On("RefreshIAMToken", mock.Anything, mock.Anything).Return(tt.mockToken, tt.mockError)

			svc := NewYandexAuthService(cfg, mockLogger)
			svc.token = tt.cachedToken

			// Act
			token, err := svc.GetIAMToken(context.Background())

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedToken, token)
			}
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRefreshTokenPeriodically(t *testing.T) {
	// Arrange
	cfg := &config.Config{YandexOAuthToken: "test-token"}
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("RefreshIAMToken", mock.Anything, mock.Anything).Return("new-token", nil)

	svc := NewYandexAuthService(cfg, mockLogger)

	// Создаем контекст с отменой для контроля теста
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Act
	done := make(chan struct{})
	go func() {
		svc.refreshTokenPeriodically()
		close(done)
	}()

	// Assert
	select {
	case <-ctx.Done():
		// Проверяем состояние токена
		svc.mu.RLock()
		token := svc.token
		svc.mu.RUnlock()
		assert.NotEmpty(t, token)
		mockLogger.AssertExpectations(t)
	case <-done:
		t.Error("refreshTokenPeriodically завершился преждевременно")
	}
}
