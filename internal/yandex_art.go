package internal

import (
	"fmt"
	"net/http"
	"os"
	"time"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log"
)

type YandexARTRequest struct {
	ModelUri          string            `json:"modelUri"`
	GenerationOptions GenerationOptions  `json:"generationOptions"`
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

// createPrompt создает запрос для генерации изображения с проверкой переменных окружения
func createPrompt() (*YandexARTRequest, error) {
	folderID := os.Getenv("YANDEX_ART_FOLDER_ID")
	if folderID == "" {
		return nil, fmt.Errorf("YANDEX_ART_FOLDER_ID environment variable not set")
	}

	// Логируем значение для проверки
	log.Printf("Using folder ID: %s", folderID)

	return &YandexARTRequest{
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
				Text:   "нарисуй смешного кота в стиле мема, яркие цвета, четкое изображение, высокое качество",
			},
		},
	}, nil
}

const imageGenerationURL = "https://llm.api.cloud.yandex.net/foundationModels/v1/imageGenerationAsync"
const operationURLBase = "https://llm.api.cloud.yandex.net:443/operations/"

func GenerateImageFromYandexART() ([]byte, error) {
	iamToken := os.Getenv("YANDEX_IAM_TOKEN")
	if iamToken == "" {
		return nil, fmt.Errorf("YANDEX_IAM_TOKEN environment variable not set")
	}

	// Создаем промпт с проверкой переменных окружения
	prompt, err := createPrompt()
	if err != nil {
		return nil, fmt.Errorf("creating prompt: %w", err)
	}

	// 1. Start async generation
	operationID, err := startImageGeneration(iamToken, prompt)
	if err != nil {
		return nil, fmt.Errorf("starting image generation: %w", err)
	}

	// 2. Wait for the operation to complete and get the image
	imageData, err := waitForImageAndGet(iamToken, operationID)
	if err != nil {
		return nil, fmt.Errorf("waiting for image: %w", err)
	}

	return imageData, nil
}

func startImageGeneration(iamToken string, prompt *YandexARTRequest) (string, error) {
	requestBody, err := json.Marshal(prompt)
	if err != nil {
		return "", fmt.Errorf("marshalling request: %w", err)
	}

	// Log the request body for debugging
	log.Printf("Request body: %s", string(requestBody))

	req, err := http.NewRequest("POST", imageGenerationURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+iamToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read and log the error response
		var errResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResponse); err == nil {
			log.Printf("Error response: %v", errResponse)
		}
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var operation YandexARTOperation
	if err := json.NewDecoder(resp.Body).Decode(&operation); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if operation.ID == "" {
		return "", fmt.Errorf("no operation ID in response")
	}

	return operation.ID, nil
}

func waitForImageAndGet(iamToken, operationID string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	maxAttempts := 180 // 30 minutes with 10-second intervals

	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := http.NewRequest("GET", operationURLBase+operationID, nil)
		if err != nil {
			return nil, fmt.Errorf("creating status request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+iamToken)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error checking status (attempt %d): %v", attempt, err)
			time.Sleep(10 * time.Second)
			continue
		}

		var operation YandexARTOperation
		if err := json.NewDecoder(resp.Body).Decode(&operation); err != nil {
			resp.Body.Close()
			log.Printf("Error decoding status response (attempt %d): %v", attempt, err)
			time.Sleep(10 * time.Second)
			continue
		}
		resp.Body.Close()

		if operation.Done {
			if operation.Response.Image == "" {
				return nil, fmt.Errorf("operation completed but no image data received")
			}

			imageData, err := base64.StdEncoding.DecodeString(operation.Response.Image)
			if err != nil {
				return nil, fmt.Errorf("decoding base64 image: %w", err)
			}

			return imageData, nil
		}

		time.Sleep(10 * time.Second)
	}

	return nil, fmt.Errorf("operation timed out after %d attempts", maxAttempts)
}
