package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
)

const (
	gptCompletionURL = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
	modelName        = "yandexgpt-lite"
)

// YandexGPTServiceImpl реализует сервис для работы с Yandex GPT API
type YandexGPTServiceImpl struct {
	config      *config.Config
	logger      *logger.Logger
	authService YandexAuthService
	mu          sync.RWMutex
	token       string
	lastRefresh time.Time
}

// NewYandexGPTService создает новый экземпляр GPT сервиса
func NewYandexGPTService(cfg *config.Config, log *logger.Logger, auth YandexAuthService) *YandexGPTServiceImpl {
	return &YandexGPTServiceImpl{
		config:      cfg,
		logger:      log,
		authService: auth,
		mu:          sync.RWMutex{},
	}
}

func (s *YandexGPTServiceImpl) getToken(ctx context.Context) (string, error) {
	s.mu.RLock()
	token := s.token
	lastRefresh := s.lastRefresh
	s.mu.RUnlock()

	// Если токен есть и он свежий (менее 11 часов), используем его
	if token != "" && time.Since(lastRefresh) < 11*time.Hour {
		return token, nil
	}

	// Получаем новый токен
	newToken, err := s.authService.GetIAMToken(ctx)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.token = newToken
	s.lastRefresh = time.Now()
	s.mu.Unlock()

	return newToken, nil
}

// GenerateImagePrompt генерирует промпт и подпись для создания изображения
func (s *YandexGPTServiceImpl) GenerateImagePrompt(ctx context.Context, userPrompt string) (string, string, error) {
	s.logger.Debug(ctx, "Requesting IAM token", nil)
	iamToken, err := s.getToken(ctx)
	if err != nil {
		return "", "", fmt.Errorf("getting IAM token: %w", err)
	}

	// Создаем запрос к Yandex GPT API
	request := GPTRequest{
		ModelUri: fmt.Sprintf("gpt://%s/%s", s.config.YandexArtFolderID, modelName),
		CompletionOptions: CompletionOptions{
			Stream:      false,
			Temperature: 0.6,
			MaxTokens:   "200",
		},
		Messages: []GPTMessage{
			{
				Role: "system",
				Text: `
				Ты выступаешь в роли креативного мем-редактора и стендапера в одном лице. Твоя задача — преобразовать короткое описание мема так, чтобы получилась злободневная, ироничная и запоминающаяся шутка, содержащая:
				1. Небольшую завязку (контекст или ситуацию), которая намекает на современную поп-культуру, тренд или повседневную проблему.
				2. Юмористический поворот с использованием абсурда, гиперболы или контраста.
				3. Эмоциональные слова и лёгкий сленг, которые усилят комичность.
				4. Отсылку к чему-то неожиданному (исторический факт, известная личность, бытовая мелочь), чтобы вызвать «эффект сюрприза».
				5. Финальную формулировку для подписи на изображении (короткую, не более 1–2 строк).

				Ответ должен быть в формате JSON:
				{
					"context": "Контекст/ситуация",
					"detail": "Остроумная деталь",
					"caption": "Итоговая подпись для картинки"
				}`,
			},
			{
				Role: "user",
				Text: fmt.Sprintf(`Создай краткое описание мема на тему: %s. Опиши основные элементы, цвета и настроение.`, userPrompt),
			},
		},
	}

	// Отправляем запрос
	s.logger.Debug(ctx, "Initiating GPT request", map[string]interface{}{
		"prompt_length": len(userPrompt),
	})

	response, err := s.sendGPTRequest(ctx, iamToken, request)
	if err != nil {
		s.logger.Error(ctx, "Failed to generate enhanced prompt, falling back to original", map[string]interface{}{
			"error":           err.Error(),
			"original_prompt": userPrompt,
		})
		return userPrompt, "", nil
	}

	// Проверяем наличие ответа
	if len(response.Result.Alternatives) == 0 {
		s.logger.Error(ctx, "Empty GPT response, falling back to original prompt", map[string]interface{}{
			"original_prompt": userPrompt,
		})
		return userPrompt, "", nil
	}

	// Удаляем обратные кавычки из ответа
	responseText := strings.Trim(response.Result.Alternatives[0].Message.Text, "`")

	// Пытаемся распарсить JSON-ответ
	var promptResponse GPTPromptResponse
	if err := json.Unmarshal([]byte(responseText), &promptResponse); err != nil {
		s.logger.Error(ctx, "Failed to parse GPT JSON response, using original text", map[string]interface{}{
			"error": err.Error(),
			"text":  responseText,
		})
		return userPrompt, "", nil
	}

	// Формируем итоговый промпт из context и detail
	enhancedPrompt := promptResponse.Context + "\n\n" + promptResponse.Detail

	s.logger.Debug(ctx, "Successfully parsed GPT response", map[string]interface{}{
		"context": promptResponse.Context,
		"detail":  promptResponse.Detail,
		"caption": promptResponse.Caption,
	})

	return enhancedPrompt, promptResponse.Caption, nil
}

// sendGPTRequest отправляет запрос к Yandex GPT API и обрабатывает ответ
func (s *YandexGPTServiceImpl) sendGPTRequest(ctx context.Context, iamToken string, request GPTRequest) (*GPTResponse, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		s.logger.Error(ctx, "Failed to marshal GPT request", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	s.logger.Debug(ctx, "Preparing GPT service request", map[string]interface{}{
		"url":    gptCompletionURL,
		"method": "POST",
	})
	req, err := http.NewRequestWithContext(ctx, "POST", gptCompletionURL, bytes.NewBuffer(requestBody))
	if err != nil {
		s.logger.Error(ctx, "Failed to create GPT request", map[string]interface{}{
			"error": err.Error(),
			"url":   gptCompletionURL,
		})
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+iamToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-folder-id", s.config.YandexArtFolderID)

	s.logger.Debug(ctx, "Sending GPT request", map[string]interface{}{
		"folder_id": s.config.YandexArtFolderID,
		"headers":   req.Header,
	})

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error(ctx, "Failed to execute GPT request", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Debug(ctx, "Received GPT response", map[string]interface{}{
		"status_code": resp.StatusCode,
	})
	if resp.StatusCode == http.StatusUnauthorized {
		// Сбрасываем токен и пробуем еще раз
		s.mu.Lock()
		s.token = ""
		s.mu.Unlock()
		
		s.logger.Info(ctx, "Token expired, retrying with new token", nil)
		return s.sendGPTRequest(ctx, iamToken, request)
	}

	if resp.StatusCode != http.StatusOK {
		// Пытаемся прочитать тело ошибки
		var errResponse GPTErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResponse); err == nil {
			s.logger.Error(ctx, "GPT service returned error", map[string]interface{}{
				"status_code": resp.StatusCode,
				"error":       errResponse,
			})
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		s.logger.Error(ctx, "GPT service returned error with undecodable body", map[string]interface{}{
			"status_code": resp.StatusCode,
		})
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response GPTResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		s.logger.Error(ctx, "Failed to decode GPT response", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	s.logger.Debug(ctx, "Successfully processed GPT response", map[string]interface{}{
		"alternatives_count": len(response.Result.Alternatives),
	})

	return &response, nil
}

// Структуры данных для работы с Yandex GPT API

// GPTRequest описывает формат запроса к API
type GPTRequest struct {
	ModelUri          string            `json:"modelUri"`
	CompletionOptions CompletionOptions `json:"completionOptions"`
	Messages          []GPTMessage      `json:"messages"`
}

type CompletionOptions struct {
	Stream      bool    `json:"stream"`
	Temperature float64 `json:"temperature"`
	MaxTokens   string  `json:"maxTokens"`
}

type GPTMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type GPTResponse struct {
	Result struct {
		Alternatives []struct {
			Message struct {
				Role string `json:"role"`
				Text string `json:"text"`
			} `json:"message"`
			Status string `json:"status"`
		} `json:"alternatives"`
		Usage struct {
			InputTextTokens  string `json:"inputTextTokens"`
			CompletionTokens string `json:"completionTokens"`
			TotalTokens      string `json:"totalTokens"`
		} `json:"usage"`
		ModelVersion string `json:"modelVersion"`
	} `json:"result"`
}

// GPTPromptResponse представляет структурированный ответ от GPT
type GPTPromptResponse struct {
	Context string `json:"context"`
	Detail  string `json:"detail"`
	Caption string `json:"caption"`
}

// GPTErrorResponse описывает структуру ошибки от API
type GPTErrorResponse struct {
	Error struct {
		GrpcCode   int      `json:"grpcCode"`
		HttpCode   int      `json:"httpCode"`
		Message    string   `json:"message"`
		HttpStatus string   `json:"httpStatus"`
		Details    []string `json:"details"`
	} `json:"error"`
}

// truncateText обрезает текст до указанной длины, сохраняя целые предложения
func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	lastDot := strings.LastIndex(text[:maxLength], ".")
	if lastDot == -1 {
		return text[:maxLength]
	}

	return text[:lastDot+1]
}
