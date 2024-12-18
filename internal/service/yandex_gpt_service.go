package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
)

const (
	gptCompletionURL = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
	modelName        = "yandexgpt-lite"
)

// YandexGPTServiceImpl реализует сервис для работы с Yandex GPT API
// Сервис используется для улучшения пользовательских промптов перед генерацией изображений
// Структура содержит:
// - config: конфигурация с токенами и настройками
// - logger: компонент для логирования операций
// - authService: сервис аутентификации для получения IAM токенов
type YandexGPTServiceImpl struct {
	config      *config.Config
	logger      *logger.Logger
	authService YandexAuthService
}

// NewYandexGPTService создает новый экземпляр GPT сервиса
func NewYandexGPTService(cfg *config.Config, log *logger.Logger, auth YandexAuthService) *YandexGPTServiceImpl {
	return &YandexGPTServiceImpl{
		config:      cfg,
		logger:      log,
		authService: auth,
	}
}

// GenerateImagePrompt генерирует промпт для создания изображения
func (s *YandexGPTServiceImpl) GenerateImagePrompt(ctx context.Context, userPrompt string) (string, error) {
	s.logger.Debug("Trying to get IAM token")
	iamToken, err := s.authService.GetIAMToken(ctx)
	if err != nil {
		return "", fmt.Errorf("getting IAM token: %w", err)
	}

	// Создаем запрос к Yandex GPT API
	// ModelUri: указывает на конкретную версию модели в облаке
	// CompletionOptions:
	// - Stream: false - получаем ответ целиком, не потоком
	// - Temperature: 0.6 - баланс между креативностью и предсказуемостью
	// - MaxTokens: 200 - ограничиваем длину ответа для получения кратких описаний
	// Messages:
	// - System: задает общий контекст и стиль ответов
	// - User: содержит конкретный запрос на основе пользовательского промпта
	request := GPTRequest{
		ModelUri: fmt.Sprintf("gpt://%s/%s", s.config.YandexArtFolderID, modelName),
		CompletionOptions: CompletionOptions{
			Stream:      false,
			Temperature: 0.6,   // Умеренная креативность
			MaxTokens:   "200", // Ограничиваем длину ответа
		},
		Messages: []GPTMessage{
			{Role: "system", Text: `
            Используй подробную цепочку рассуждений для выработки ответа, но не включай эти рассуждения в свой финальный ответ. 
            Предоставь только окончательный результат. 
Создавай короткие и четкие описания для иллюстраций. Используй не более 2-3 предложений.`},
			{
				Role: "user",
				Text: fmt.Sprintf(`Пожалуйста, сгенерируй оригинальный и смешной мем, который способен рассмешить широкую русскую аудиторию. 
            Используй актуальные темы, популярные культурные отсылки или повседневные ситуации, с которыми могут столкнуться многие люди. 
            Мем должен быть остроумным, с неожиданным поворотом или шуткой, использующей игровой подход к словам или ситуации. 
            Ты можешь использовать известные шаблоны мемов. 
            Цель — вызвать положительные эмоции и смех у читателей. Создай краткое описание мема на тему: %s. Опиши основные элементы, цвета и настроение.`, userPrompt),
			},
		},
	}

	// Отправляем запрос
	s.logger.Debug("Creating HTTP request to GPT service")
	response, err := s.sendGPTRequest(ctx, iamToken, request)
	if err != nil {
		s.logger.Error("Failed to generate enhanced prompt: %v, using original prompt", err)
		return userPrompt, nil
	}

	// Проверяем наличие ответа
	if len(response.Result.Alternatives) == 0 {
		s.logger.Error("No alternatives in response, using original prompt")
		return userPrompt, nil
	}

	// Ограничиваем длину промпта
	prompt := truncateText(response.Result.Alternatives[0].Message.Text, 400)

	return prompt, nil
}

// sendGPTRequest отправляет запрос к Yandex GPT API и обрабатывает ответ
// Параметры:
// - ctx: контекст для отмены операции
// - iamToken: токен для аутентификации в API
// - request: структура запроса с промптом и настройками
// Возвращает:
// - *GPTResponse: структуру с сгенерированным текстом
// - error: ошибку в случае проблем с API или обработкой ответа
func (s *YandexGPTServiceImpl) sendGPTRequest(ctx context.Context, iamToken string, request GPTRequest) (*GPTResponse, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	s.logger.Debug("Sending request to GPT service")
	req, err := http.NewRequestWithContext(ctx, "POST", gptCompletionURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+iamToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-folder-id", s.config.YandexArtFolderID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Debug("Got response from GPT service, status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		// Пытаемся прочитать тело ошибки
		var errResponse GPTErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResponse); err == nil {
			s.logger.Error("GPT service returned non-200 status code: %d, body: %+v", resp.StatusCode, errResponse)
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response GPTResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &response, nil
}

// Структуры данных для работы с Yandex GPT API

// GPTRequest описывает формат запроса к API
// ModelUri: путь к модели в формате gpt://{folder_id}/{model_name}
// CompletionOptions: настройки генерации текста
// Messages: массив сообщений для контекста и запроса
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

// GPTErrorResponse описывает структуру ошибки от API
// Содержит:
// - GrpcCode: код ошибки gRPC
// - HttpCode: HTTP код ответа
// - Message: текстовое описание ошибки
// - HttpStatus: статус HTTP ответа
// - Details: дополнительная информация об ошибке
type GPTErrorResponse struct {
	Error struct {
		GrpcCode   int      `json:"grpcCode"`   // Код ошибки gRPC
		HttpCode   int      `json:"httpCode"`   // HTTP статус
		Message    string   `json:"message"`    // Описание ошибки
		HttpStatus string   `json:"httpStatus"` // Текстовый HTTP статус
		Details    []string `json:"details"`    // Детали ошибки
	} `json:"error"`
}

// truncateText обрезает текст до указанной длины, сохраняя целые предложения
// Алгоритм:
// 1. Проверяет, не превышает ли текст максимальную длину
// 2. Ищет последнюю точку перед максимальной длиной
// 3. Обрезает текст по найденной точке или по максимальной длине
// Параметры:
// - text: исходный текст для обработки
// - maxLength: максимально допустимая длина
// Возвращает:
// - обработанный текст, не превышающий maxLength и заканчивающийся полным предложением
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
