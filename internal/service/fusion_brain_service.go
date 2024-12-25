package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"time"

	"github.com/azalio/meme-bot/pkg/logger"
)

const (
	fusionBrainBaseURL = "https://api-key.fusionbrain.ai/"
)

// FusionBrainServiceImpl implements image generation using FusionBrain API
type FusionBrainServiceImpl struct {
	logger    *logger.Logger
	apiKey    string
	secretKey string
	modelID   int
}

// NewFusionBrainService creates a new instance of FusionBrainService
func NewFusionBrainService(log *logger.Logger) *FusionBrainServiceImpl {
	apiKey := os.Getenv("FUSION_BRAIN_API_KEY")
	secretKey := os.Getenv("FUSION_BRAIN_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		log.Error("FUSION_BRAIN_API_KEY or FUSION_BRAIN_SECRET_KEY not set")
		return nil
	}

	service := &FusionBrainServiceImpl{
		logger:    log,
		apiKey:    apiKey,
		secretKey: secretKey,
	}

	// Get model ID during initialization
	modelID, err := service.getModel()
	if err != nil {
		log.Error("Failed to get model ID: %v", err)
		return nil
	}

	service.modelID = modelID
	return service
}

type FusionBrainModel struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Version float64 `json:"version"`
	Type    string  `json:"type"`
}

type GenerateParams struct {
	Query string `json:"query"`
}

type GenerateRequest struct {
	Type           string         `json:"type"`
	NumImages      int            `json:"numImages"`
	Width          int            `json:"width"`
	Height         int            `json:"height"`
	GenerateParams GenerateParams `json:"generateParams"`
}

type GenerateResponse struct {
	UUID   string `json:"uuid"`
	Status string `json:"status"`
}

type StatusResponse struct {
	UUID       string   `json:"uuid"`
	Status     string   `json:"status"`
	Images     []string `json:"images"`
	Error      string   `json:"errorDescription,omitempty"`
	IsCensored bool     `json:"censored"`
}

// getModel retrieves the available model ID
func (s *FusionBrainServiceImpl) getModel() (int, error) {
	req, err := http.NewRequest("GET", fusionBrainBaseURL+"key/api/v1/models", nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	s.addAuthHeaders(req)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var models []FusionBrainModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	if len(models) == 0 {
		return 0, fmt.Errorf("no models available")
	}

	return models[0].ID, nil
}

// addAuthHeaders adds the required authentication headers to the request
func (s *FusionBrainServiceImpl) addAuthHeaders(req *http.Request) {
	req.Header.Set("X-Key", "Key "+s.apiKey)
	req.Header.Set("X-Secret", "Secret "+s.secretKey)
}

// GenerateImage generates an image using FusionBrain API
func (s *FusionBrainServiceImpl) GenerateImage(ctx context.Context, promptText string) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("FusionBrain service not initialized")
	}

	s.logger.Info("Starting FusionBrain image generation")
	s.logger.Debug("Prompt text: %s", promptText)

	// Check service availability
	if available, err := s.checkAvailability(ctx); err != nil || !available {
		return nil, fmt.Errorf("service unavailable: %v", err)
	}

	// Start image generation
	uuid, err := s.startImageGeneration(ctx, promptText)
	if err != nil {
		return nil, fmt.Errorf("starting image generation: %w", err)
	}

	// Wait for the image and get result
	imageData, err := s.waitForImageAndGet(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("waiting for image: %w", err)
	}

	return imageData, nil
}

func (s *FusionBrainServiceImpl) checkAvailability(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf(fusionBrainBaseURL+"key/api/v1/text2image/availability?model_id=%d", s.modelID), nil)
	if err != nil {
		s.logger.Error("checkAvailability in FusionBrainService failed, err is: %s", err)
		return false, fmt.Errorf("creating request: %w", err)
	}

	s.addAuthHeaders(req)

	client := &http.Client{Timeout: 30 * time.Second}

	s.logger.Debug("Start request in FusionBrain from checkAvailability")
	resp, err := client.Do(req)
	s.logger.Debug("responce: %v", resp)

	if err != nil {
		return false, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var status struct {
		ModelStatus string `json:"model_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return false, fmt.Errorf("decoding response: %w", err)
	}

	return status.ModelStatus != "DISABLED_BY_QUEUE", nil
}

func (s *FusionBrainServiceImpl) startImageGeneration(ctx context.Context, prompt string) (string, error) {
	params := GenerateRequest{
		Type:      "GENERATE",
		NumImages: 1,
		Width:     1024,
		Height:    1024,
		GenerateParams: GenerateParams{
			Query: prompt,
		},
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshalling params: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add model_id field
	if err := writer.WriteField("model_id", fmt.Sprintf("%d", s.modelID)); err != nil {
		return "", fmt.Errorf("writing model_id: %w", err)
	}

	// Add params field with JSON content type
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", "application/json")
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, "params"))
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("creating params field: %w", err)
	}
	if _, err := part.Write(paramsJSON); err != nil {
		return "", fmt.Errorf("writing params: %w", err)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST",
		fusionBrainBaseURL+"key/api/v1/text2image/run", body)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	s.addAuthHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if response.UUID == "" {
		return "", fmt.Errorf("no UUID in response")
	}

	s.logger.Info("Image generation started")
	s.logger.Debug("UUID: %s", response.UUID)
	return response.UUID, nil
}

func (s *FusionBrainServiceImpl) waitForImageAndGet(ctx context.Context, uuid string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	maxAttempts := 60 // 10 minutes with 10-second intervals
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			s.logger.Error("Operation cancelled by context: %v", ctx.Err())
			return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-ticker.C:
			s.logger.Debug("Checking operation status, attempt %d/%d", attempt+1, maxAttempts)

			req, err := http.NewRequestWithContext(ctx, "GET",
				fusionBrainBaseURL+"key/api/v1/text2image/status/"+uuid, nil)
			if err != nil {
				s.logger.Error("Failed to create request: %v", err)
				return nil, fmt.Errorf("creating status request: %w", err)
			}

			s.addAuthHeaders(req)

			resp, err := client.Do(req)
			if err != nil {
				s.logger.Error("Request failed: %v", err)
				if ctx.Err() != nil {
					return nil, fmt.Errorf("operation cancelled during request: %w", ctx.Err())
				}
				continue
			}

			var response StatusResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			resp.Body.Close()
			if err != nil {
				s.logger.Error("Failed to decode response: %v", err)
				if ctx.Err() != nil {
					return nil, fmt.Errorf("operation cancelled during response reading: %w", ctx.Err())
				}
				continue
			}

			s.logger.Debug("Operation status: %s", response.Status)

			if response.Status == "DONE" {
				if len(response.Images) == 0 {
					s.logger.Error("Operation completed but no images received")
					return nil, fmt.Errorf("operation completed but no images received")
				}

				imageData, err := base64.StdEncoding.DecodeString(response.Images[0])
				if err != nil {
					s.logger.Error("Failed to decode base64 image: %v", err)
					return nil, fmt.Errorf("decoding base64 image: %w", err)
				}

				s.logger.Info("Image generation completed successfully")
				return imageData, nil
			} else if response.Status == "FAIL" {
				s.logger.Error("Generation failed: %s", response.Error)
				return nil, fmt.Errorf("generation failed: %s", response.Error)
			}

			// If status is not DONE or FAIL, continue waiting
			s.logger.Debug("Generation in progress, status: %s", response.Status)
		}
	}

	s.logger.Error("Operation timed out after %d attempts", maxAttempts)
	return nil, fmt.Errorf("operation timed out after %d attempts", maxAttempts)
}
