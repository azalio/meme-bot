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
	config         *config.Config
	logger         *logger.Logger
	authService    YandexAuthService
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
		config:         cfg,
		logger:         log,
		authService:    auth,
		promptEnhancer: promptEnhancer,
	}
}

// GenerateImage генерирует изображение по промпту
func (s *YandexArtServiceImpl) GenerateImage(ctx context.Context, promptText string) ([]byte, error) {
	s.logger.Info(ctx, "Starting Yandex Art image generation", map[string]interface{}{
		"prompt_length": len(promptText),
	})
	// Получаем IAM токен
	s.logger.Debug(ctx, "Requesting IAM token", nil)
	iamToken, err := s.authService.GetIAMToken(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to get IAM token", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("getting IAM token: %w", err)
	}

	// Генерируем улучшенный промпт
	s.logger.Debug(ctx, "Enhancing prompt", map[string]interface{}{
		"original_prompt": promptText,
	})
	enhancedPrompt, _, err := s.promptEnhancer.EnhancePrompt(ctx, promptText)
	if err != nil {
		s.logger.Error(ctx, "Prompt enhancement failed, using original", map[string]interface{}{
			"error":           err.Error(),
			"original_prompt": promptText,
		})
		enhancedPrompt = promptText
	}

	// Создаем запрос на генерацию
	operationID, err := s.startImageGeneration(ctx, enhancedPrompt, iamToken)
	if err != nil {
		s.logger.Error(ctx, "Failed to start image generation", map[string]interface{}{
			"error":           err.Error(),
			"enhanced_prompt": enhancedPrompt,
		})
		return nil, fmt.Errorf("starting image generation: %w", err)
	}

	s.logger.Debug(ctx, "Image generation started", map[string]interface{}{
		"operation_id": operationID,
		"prompt":       enhancedPrompt,
	})

	// Ожидаем завершения и получаем результат
	imageData, err := s.waitForImageAndGet(ctx, operationID, iamToken)
	if err != nil {
		s.logger.Error(ctx, "Failed to get generated image", map[string]interface{}{
			"error":        err.Error(),
			"operation_id": operationID,
		})
		return nil, fmt.Errorf("waiting for image: %w", err)
	}

	s.logger.Info(ctx, "Successfully generated image", map[string]interface{}{
		"operation_id": operationID,
		"image_size":   len(imageData),
	})

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
	startTime := time.Now()
	defer func() {
		metrics.APIResponseTime.Observe(time.Since(startTime).Seconds(), attribute.String("service", "yandex_art"))
	}()
	s.logger.Info(ctx, "Initiating image generation request", map[string]interface{}{
		"prompt_length": len(prompt),
	})

	folderID := os.Getenv("YANDEX_ART_FOLDER_ID")
	if folderID == "" {
		s.logger.Error(ctx, "Missing required environment variable", map[string]interface{}{
			"variable": "YANDEX_ART_FOLDER_ID",
		})
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
		s.logger.Error(ctx, "Failed to marshal generation request", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", imageGenerationURL, bytes.NewBuffer(requestBody))
	if err != nil {
		s.logger.Error(ctx, "Failed to create HTTP request", map[string]interface{}{
			"error": err.Error(),
			"url":   imageGenerationURL,
		})
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+iamToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error(ctx, "HTTP request failed", map[string]interface{}{
			"error": err.Error(),
			"url":   imageGenerationURL,
		})
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResponse); err == nil {
			s.logger.Error(ctx, "API returned error response", map[string]interface{}{
				"status_code": resp.StatusCode,
				"error":       errResponse,
			})
			return "", fmt.Errorf("API error response: %v", errResponse)
		}
		s.logger.Error(ctx, "Unexpected API response status", map[string]interface{}{
			"status_code": resp.StatusCode,
		})
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var operation YandexARTOperation
	if err := json.NewDecoder(resp.Body).Decode(&operation); err != nil {
		s.logger.Error(ctx, "Failed to decode API response", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if operation.ID == "" {
		s.logger.Error(ctx, "Invalid API response: missing operation ID", nil)
		return "", fmt.Errorf("no operation ID in response")
	}

	s.logger.Info(ctx, "Generation operation started", map[string]interface{}{
		"operation_id": operation.ID,
		"model_uri":    request.ModelUri,
	})

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

	s.logger.Info(ctx, "Starting to wait for image generation", map[string]interface{}{
		"operation_id": operationID,
		"max_attempts": maxAttempts,
		"interval":     "5s",
	})

	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			s.logger.Error(ctx, "Operation cancelled", map[string]interface{}{
				"error":        ctx.Err().Error(),
				"operation_id": operationID,
				"attempt":      attempt + 1,
			})
			return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-ticker.C:
			s.logger.Debug(ctx, "Checking operation status", map[string]interface{}{
				"attempt":      attempt + 1,
				"max_attempts": maxAttempts,
				"operation_id": operationID,
			})

			req, err := http.NewRequestWithContext(ctx, "GET", operationURLBase+operationID, nil)
			if err != nil {
				s.logger.Error(ctx, "Failed to create status request", map[string]interface{}{
					"error":        err.Error(),
					"operation_id": operationID,
					"url":          operationURLBase + operationID,
				})
				return nil, fmt.Errorf("creating status request: %w", err)
			}

			req.Header.Set("Authorization", "Bearer "+iamToken)

			resp, err := client.Do(req)
			if err != nil {
				s.logger.Error(ctx, "Status request failed", map[string]interface{}{
					"error":        err.Error(),
					"operation_id": operationID,
					"attempt":      attempt + 1,
				})
				if ctx.Err() != nil {
					return nil, fmt.Errorf("operation cancelled during request: %w", ctx.Err())
				}
				continue
			}
			defer resp.Body.Close()

			var operation YandexARTOperation
			err = json.NewDecoder(resp.Body).Decode(&operation)
			if err != nil {
				s.logger.Error(ctx, "Failed to decode operation status", map[string]interface{}{
					"error":        err.Error(),
					"operation_id": operationID,
					"attempt":      attempt + 1,
				})
				if ctx.Err() != nil {
					return nil, fmt.Errorf("operation cancelled during response reading: %w", ctx.Err())
				}
				continue
			}

			s.logger.Debug(ctx, "Received operation status", map[string]interface{}{
				"operation_id": operationID,
				"done":         operation.Done,
				"attempt":      attempt + 1,
			})
			if operation.Done {
				if operation.Response.Image == "" {
					s.logger.Error(ctx, "Operation completed without image data", map[string]interface{}{
						"operation_id": operationID,
					})
					return nil, fmt.Errorf("operation completed but no image data received")
				}

				imageData, err := base64.StdEncoding.DecodeString(operation.Response.Image)
				if err != nil {
					s.logger.Error(ctx, "Failed to decode image data", map[string]interface{}{
						"error":        err.Error(),
						"operation_id": operationID,
					})
					return nil, fmt.Errorf("decoding base64 image: %w", err)
				}

				s.logger.Info(ctx, "Successfully retrieved generated image", map[string]interface{}{
					"operation_id": operationID,
					"image_size":   len(imageData),
					"attempts":     attempt + 1,
				})
				return imageData, nil
			}
		}
	}

	s.logger.Error(ctx, "Generation operation timed out", map[string]interface{}{
		"operation_id": operationID,
		"max_attempts": maxAttempts,
		"total_time":   fmt.Sprintf("%ds", maxAttempts*5),
	})
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
