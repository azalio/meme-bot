package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
)

const (
	imageGenerationURL = "https://llm.api.cloud.yandex.net/foundationModels/v1/imageGenerationAsync"
	operationURLBase   = "https://llm.api.cloud.yandex.net:443/operations/"
)

// YandexArtServiceImpl реализует интерфейс YandexArtService
type YandexArtServiceImpl struct {
	config        *config.Config
	logger        *logger.Logger
	authService   YandexAuthService
	promptEnhancer *PromptEnhancer
}

// NewYandexArtService создает новый экземпляр сервиса генерации изображений
func NewYandexArtService(
	cfg *config.Config,
	log *logger.Logger,
	auth YandexAuthService,
	gpt YandexGPTService,
) *YandexArtServiceImpl {
	promptEnhancer := NewPromptEnhancer(log, gpt)
	return &YandexArtServiceImpl{
		config:        cfg,
		logger:        log,
		authService:   auth,
		promptEnhancer: promptEnhancer,
	}
}

// GenerateImage генерирует изображение по промпту
func (s *YandexArtServiceImpl) GenerateImage(ctx context.Context, promptText string) ([]byte, error) {
	s.logger.Info("Starting image generation")
	s.logger.Debug("Prompt text: %s", promptText)

	// Получаем IAM токен
	s.logger.Debug("Вход в метод GetIAMToken")
	iamToken, err := s.authService.GetIAMToken(ctx)
	if err != nil {
		s.logger.Error("Error in GetIAMToken: %s", err)
		return nil, fmt.Errorf("getting IAM token: %w", err)
	}

	// Генерируем улучшенный промпт
	s.logger.Debug("Enhancing prompt")
	enhancedPrompt, err := s.promptEnhancer.EnhancePrompt(ctx, promptText)
	if err != nil {
		s.logger.Error("Failed to enhance prompt: %v, using original prompt", err)
		enhancedPrompt = promptText
	}

	// Создаем запрос на генерацию
	operationID, err := s.startImageGeneration(ctx, enhancedPrompt, iamToken)
	if err != nil {
		return nil, fmt.Errorf("starting image generation: %w", err)
	}

	// Ожидаем завершения и получаем результат
	imageData, err := s.waitForImageAndGet(ctx, operationID, iamToken)
	if err != nil {
		return nil, fmt.Errorf("waiting for image: %w", err)
	}

	return imageData, nil
}

// startImageGeneration инициирует асинхронный процесс генерации изображения в Yandex Art API
// Параметры:
// - ctx: контекст для отмены операции
// - prompt: текстовое описание желаемого изображения
// - iamToken: токен для аутентификации в API
// Возвращает:
// - string: ID операции для отслеживания прогресса
// - error: ошибку в случае проблем с запуском генерации
func (s *YandexArtServiceImpl) startImageGeneration(ctx context.Context, prompt string, iamToken string) (string, error) {
	s.logger.Info("Starting image generation")
	s.logger.Debug("Using prompt: %s", prompt)
	folderID := os.Getenv("YANDEX_ART_FOLDER_ID")
	if folderID == "" {
		s.logger.Error("YANDEX_ART_FOLDER_ID not set")
		return "", fmt.Errorf("YANDEX_ART_FOLDER_ID not set")
	}

	request := YandexARTRequest{
		ModelUri: fmt.Sprintf("art://%s/yandex-art/latest", folderID),
		GenerationOptions: GenerationOptions{
			Seed: "1863",
			AspectRatio: AspectRatio{
				WidthRatio:  "1",
				HeightRatio: "1",
			},
		},
		Messages: []Message{
			{
				Weight: "1",
				Text:   prompt,
			},
		},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		s.logger.Error("Failed to marshal request: %v", err)
		return "", fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", imageGenerationURL, bytes.NewBuffer(requestBody))
	if err != nil {
		s.logger.Error("Failed to create request: %v", err)
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+iamToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Request failed: %v", err)
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResponse); err == nil {
			s.logger.Error("API error response: %v", errResponse)
			return "", fmt.Errorf("API error response: %v", errResponse)
		}
		s.logger.Error("Unexpected status code: %d", resp.StatusCode)
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var operation YandexARTOperation
	if err := json.NewDecoder(resp.Body).Decode(&operation); err != nil {
		s.logger.Error("Failed to decode response: %v", err)
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if operation.ID == "" {
		s.logger.Error("No operation ID in response")
		return "", fmt.Errorf("no operation ID in response")
	}

	s.logger.Info("Image generation started")
	s.logger.Debug("Operation ID: %s", operation.ID)
	return operation.ID, nil
}

// waitForImageAndGet выполняет поллинг статуса операции генерации изображения
// и возвращает результат после завершения
// Параметры:
// - ctx: контекст для отмены операции
// - operationID: идентификатор операции генерации
// - iamToken: токен для аутентификации в API
// Возвращает:
// - []byte: сгенерированное изображение в формате PNG
// - error: ошибку в случае проблем с получением результата
// Метод будет повторять запросы каждые 5 секунд в течение 5 минут
func (s *YandexArtServiceImpl) waitForImageAndGet(ctx context.Context, operationID string, iamToken string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	maxAttempts := 60 // 5 minutes with 5-second intervals
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			s.logger.Error("Operation cancelled by context: %v", ctx.Err())
			return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-ticker.C:
			s.logger.Debug("Checking operation status, attempt %d/%d", attempt+1, maxAttempts)

			req, err := http.NewRequestWithContext(ctx, "GET", operationURLBase+operationID, nil)
			if err != nil {
				s.logger.Error("Failed to create request: %v", err)
				return nil, fmt.Errorf("creating status request: %w", err)
			}

			req.Header.Set("Authorization", "Bearer "+iamToken)

			resp, err := client.Do(req)
			if err != nil {
				s.logger.Error("Request failed: %v", err)
				if ctx.Err() != nil {
					return nil, fmt.Errorf("operation cancelled during request: %w", ctx.Err())
				}
				continue
			}
			defer resp.Body.Close()

			var operation YandexARTOperation
			err = json.NewDecoder(resp.Body).Decode(&operation)
			if err != nil {
				s.logger.Error("Failed to decode response: %v", err)
				if ctx.Err() != nil {
					return nil, fmt.Errorf("operation cancelled during response reading: %w", ctx.Err())
				}
				continue
			}

			s.logger.Debug("Operation status: done=%v", operation.Done)
			if operation.Done {
				if operation.Response.Image == "" {
					s.logger.Error("Operation completed but no image data received")
					return nil, fmt.Errorf("operation completed but no image data received")
				}

				imageData, err := base64.StdEncoding.DecodeString(operation.Response.Image)
				if err != nil {
					s.logger.Error("Failed to decode base64 image: %v", err)
					return nil, fmt.Errorf("decoding base64 image: %w", err)
				}

				s.logger.Info("Image generation completed successfully")
				return imageData, nil
			}
		}
	}

	s.logger.Error("Operation timed out after %d attempts", maxAttempts)
	return nil, fmt.Errorf("operation timed out after %d attempts", maxAttempts)
}

// YandexARTRequest представляет структуру запроса к API генерации изображений
// Документация: https://cloud.yandex.ru/docs/ai/vision/api-ref/
type YandexARTRequest struct {
	ModelUri          string            `json:"modelUri"`
	GenerationOptions GenerationOptions `json:"generationOptions"`
	Messages          []Message         `json:"messages"`
}

type GenerationOptions struct {
	Seed        string      `json:"seed"`
	AspectRatio AspectRatio `json:"aspectRatio"`
}

type AspectRatio struct {
	WidthRatio  string `json:"widthRatio"`
	HeightRatio string `json:"heightRatio"`
}

type Message struct {
	Weight string `json:"weight"`
	Text   string `json:"text"`
}

type YandexARTOperation struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	CreatedAt   interface{} `json:"createdAt"`
	CreatedBy   string      `json:"createdBy"`
	ModifiedAt  interface{} `json:"modifiedAt"`
	Done        bool        `json:"done"`
	Metadata    interface{} `json:"metadata"`
	Response    struct {
		Image string `json:"image"`
	} `json:"response,omitempty"`
}
