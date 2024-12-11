package internal

import (
	"fmt"
	"net/http"
	"os"
	"bytes"
	"encoding/json"
	"log"
	"strings"
)

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
			InputTextTokens   string `json:"inputTextTokens"`
			CompletionTokens string `json:"completionTokens"`
			TotalTokens      string `json:"totalTokens"`
		} `json:"usage"`
	} `json:"result"`
}

const gptCompletionURL = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"

// truncateText обрезает текст до указанной длины, сохраняя целые предложения
func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	// Находим последнюю точку перед maxLength
	lastDot := strings.LastIndex(text[:maxLength], ".")
	if lastDot == -1 {
		return text[:maxLength]
	}

	return text[:lastDot+1]
}

func GenerateImagePrompt(userPrompt string) (string, error) {
	iamToken := os.Getenv("YANDEX_IAM_TOKEN")
	folderID := os.Getenv("YANDEX_ART_FOLDER_ID")
	if iamToken == "" || folderID == "" {
		return "", fmt.Errorf("YANDEX_IAM_TOKEN or YANDEX_ART_FOLDER_ID not set")
	}

	// Создаем запрос к GPT
	request := GPTRequest{
		ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", folderID),
		CompletionOptions: CompletionOptions{
			Stream:      false,
			Temperature: 0.6, // Умеренная креативность
			MaxTokens:   "200", // Ограничиваем длину ответа
		},
		Messages: []GPTMessage{
			{
				Role: "system",
				Text: "Создавай короткие и четкие описания для иллюстраций. Используй не более 2-3 предложений.",
			},
			{
				Role: "user",
				Text: fmt.Sprintf("Создай краткое описание весёлой картинки на тему: %s. Опиши основные элементы, цвета и настроение.", userPrompt),
			},
		},
	}

	// Отправляем запрос
	response, err := sendGPTRequest(iamToken, folderID, request)
	if err != nil {
		return "", fmt.Errorf("sending GPT request: %w", err)
	}

	// Проверяем наличие ответа
	if len(response.Result.Alternatives) == 0 {
		return "", fmt.Errorf("no alternatives in response")
	}

	// Ограничиваем длину промпта до 400 символов (с запасом)
	prompt := truncateText(response.Result.Alternatives[0].Message.Text, 400)

	return prompt, nil
}

func sendGPTRequest(iamToken, folderID string, request GPTRequest) (*GPTResponse, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	// Log the request body for debugging
	log.Printf("GPT Request body: %s", string(requestBody))

	req, err := http.NewRequest("POST", gptCompletionURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+iamToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-folder-id", folderID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read and log the error response
		var errResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResponse); err == nil {
			log.Printf("GPT Error response: %v", errResponse)
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response GPTResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &response, nil
}
