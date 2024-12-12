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

// YandexAuthServiceImpl реализует сервис аутентификации для Yandex Cloud API
// Структура содержит:
// - config: конфигурация с OAuth токеном
// - logger: логгер для записи событий
// - mu: RWMutex для потокобезопасного доступа к токену
// - token: кэшированный IAM токен
type YandexAuthServiceImpl struct {
	config *config.Config
	logger *logger.Logger
	mu     sync.RWMutex // Защищает доступ к полю token
	token  string       // Кэшированный IAM токен
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
// NewYandexAuthService создает новый экземпляр сервиса аутентификации
// Параметры:
// - cfg: конфигурация с OAuth токеном для Yandex Cloud
// - log: логгер для записи событий
// Возвращает настроенный сервис и запускает горутину для периодического обновления токена
func NewYandexAuthService(cfg *config.Config, log *logger.Logger) *YandexAuthServiceImpl {
	service := &YandexAuthServiceImpl{
		config: cfg,
		logger: log,
	}

	// Запускаем фоновое обновление токена каждые 50 минут
	// IAM токены действительны в течение 12 часов, но мы обновляем чаще
	go service.refreshTokenPeriodically()

	return service
}

// GetIAMToken возвращает текущий IAM токен
// Использует RLock для безопасного чтения токена
// При отсутствии токена делегирует обновление методу refreshToken
// GetIAMToken возвращает действующий IAM токен
// Сначала проверяет кэшированный токен (используя RLock для эффективности)
// Если токен отсутствует, запускает процесс обновления (с полной блокировкой)
// Параметры:
// - ctx: контекст для отмены операции
// Возвращает:
// - string: действующий IAM токен
// - error: ошибку в случае проблем с получением токена
func (s *YandexAuthServiceImpl) GetIAMToken(ctx context.Context) (string, error) {
	s.logger.Debug("Trying to get IAM token")
	// Используем RLock для чтения - позволяет параллельный доступ
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
// refreshToken обновляет IAM токен с полной блокировкой
// Использует mutex для обеспечения атомарности операции обновления токена
// Параметры:
// - ctx: контекст для отмены операции
// Возвращает:
// - string: новый IAM токен
// - error: ошибку в случае проблем с обновлением
func (s *YandexAuthServiceImpl) refreshToken(ctx context.Context) (string, error) {
	s.logger.Debug("Пробуем получить новый токен")
	// Используем полную блокировку т.к. будем изменять token
	s.mu.Lock()
	defer s.mu.Unlock()

	// Повторная проверка после получения блокировки
	// Токен мог быть обновлен другой горутиной пока мы ждали Lock
	if s.token != "" {
		return s.token, nil
	}

	newToken, err := s.RefreshIAMToken(ctx, s.config.YandexOAuthToken)
	if err != nil {
		return "", err
	}

	s.token = newToken
	return newToken, nil
}

// refreshTokenPeriodically запускает периодическое обновление токена
// refreshTokenPeriodically запускает периодическое обновление IAM токена
// Выполняется в отдельной горутине каждые 50 минут
// При ошибке обновления логирует её и продолжает попытки
// Остановка сервиса должна производиться через закрытие контекста
func (s *YandexAuthServiceImpl) refreshTokenPeriodically() {
	ticker := time.NewTicker(50 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		_, err := s.refreshToken(ctx)
		if err != nil {
			s.logger.Error("Failed to refresh token: %v", err)
			// При ошибке сбрасываем текущий токен
			s.mu.Lock()
			s.token = ""
			s.mu.Unlock()
		} else {
			s.logger.Info("Successfully refreshed IAM token")
		}
	}
}
