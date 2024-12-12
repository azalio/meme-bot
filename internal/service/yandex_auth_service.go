package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
)

// YandexAuthServiceImpl реализует интерфейс YandexAuthService
type YandexAuthServiceImpl struct {
	config *config.Config
	logger *logger.Logger
	mu     sync.RWMutex
	token  string
}

type OAuth2Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type IAMTokenResponse struct {
	IAMToken string `json:"iamToken"`
}

// NewYandexAuthService создает новый экземпляр сервиса аутентификации
func NewYandexAuthService(cfg *config.Config, log *logger.Logger) *YandexAuthServiceImpl {
	service := &YandexAuthServiceImpl{
		config: cfg,
		logger: log,
	}

	go service.refreshTokenPeriodically()

	return service
}

// GetIAMToken возвращает текущий IAM токен
// Использует RLock для безопасного чтения токена
// При отсутствии токена делегирует обновление методу refreshToken
func (s *YandexAuthServiceImpl) GetIAMToken(ctx context.Context) (string, error) {
	s.logger.Debug("Trying to get IAM token")
	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()
	
	if token != "" {
		return token, nil
	}

	s.logger.Debug("No token found, refreshing")
	return s.refreshToken(ctx)
}

// RefreshIAMToken выполняет HTTP запрос для получения нового IAM токена
// Не содержит блокировок, так как вызывается только из refreshToken,
// который уже обеспечивает необходимую синхронизацию
func (s *YandexAuthServiceImpl) RefreshIAMToken(ctx context.Context, oauthToken string) (string, error) {
	// IAM token exchange endpoint
	iamTokenURL := "https://iam.api.cloud.yandex.net/iam/v1/tokens"

	// Создаем тело запроса
	requestBody := map[string]string{
		"yandexPassportOauthToken": oauthToken,
	}
	requestBodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshalling request body: %w", err)
	}
	s.logger.Debug("Request body: %s", string(requestBodyJSON))

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "POST", iamTokenURL, bytes.NewBuffer(requestBodyJSON))
	if err != nil {
		return "", fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	s.logger.Debug("Request headers: %v", req.Header)

	// Выполняем запрос
	s.logger.Debug("Отправляем запрос на получение токена")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}
	s.logger.Debug("Response status: %d", resp.StatusCode)
	s.logger.Debug("Response body: %s", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var iamTokenResponse IAMTokenResponse
	if err := json.Unmarshal(bodyBytes, &iamTokenResponse); err != nil {
		return "", fmt.Errorf("decoding IAM token response: %w", err)
	}

	s.logger.Info("IAM token получен успешно")
	return iamTokenResponse.IAMToken, nil
}

// refreshToken обновляет IAM токен
// Использует полную блокировку для атомарного обновления токена
func (s *YandexAuthServiceImpl) refreshToken(ctx context.Context) (string, error) {
	s.logger.Debug("Пробуем получить новый токен")
	s.mu.Lock()
	defer s.mu.Unlock()

	newToken, err := s.RefreshIAMToken(ctx, s.config.YandexOAuthToken)
	if err != nil {
		return "", err
	}

	s.token = newToken
	return newToken, nil
}

// refreshTokenPeriodically запускает периодическое обновление токена
func (s *YandexAuthServiceImpl) refreshTokenPeriodically() {
	ticker := time.NewTicker(50 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		_, err := s.refreshToken(ctx)
		if err != nil {
			s.logger.Error("Failed to refresh token: %v", err)
		} else {
			s.logger.Info("Successfully refreshed IAM token")
		}
	}
}
