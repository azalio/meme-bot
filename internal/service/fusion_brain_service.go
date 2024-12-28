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
		log.Error(context.Background(), "Environment variables not set", map[string]interface{}{
			"missing_vars": []string{"FUSION_BRAIN_API_KEY", "FUSION_BRAIN_SECRET_KEY"},
		})
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
		log.Error(context.Background(), "Failed to get model ID", map[string]interface{}{
			"error": err.Error(),
		})
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

	s.logger.Info(ctx, "Starting FusionBrain image generation", map[string]interface{}{
		"prompt_text": promptText,
	})
	// Check service availability
	if available, err := s.checkAvailability(ctx); err != nil || !available {
		s.logger.Error(ctx, "Service availability check failed", map[string]interface{}{
			"error": err,
		})
		return nil, fmt.Errorf("service unavailable: %v", err)
	}

	// Start image generation
	uuid, err := s.startImageGeneration(ctx, promptText)
	if err != nil {
		s.logger.Error(ctx, "Failed to start image generation", map[string]interface{}{
			"error":       err.Error(),
			"prompt_text": promptText,
		})
		return nil, fmt.Errorf("starting image generation: %w", err)
	}

	// Wait for the image and get result
	imageData, err := s.waitForImageAndGet(ctx, uuid)
	if err != nil {
		s.logger.Error(ctx, "Failed to get generated image", map[string]interface{}{
			"error": err.Error(),
			"uuid":  uuid,
		})
		return nil, fmt.Errorf("waiting for image: %w", err)
	}

	s.logger.Info(ctx, "Successfully generated image", map[string]interface{}{
		"uuid": uuid,
	})

	return imageData, nil
}

func (s *FusionBrainServiceImpl) checkAvailability(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf(fusionBrainBaseURL+"key/api/v1/text2image/availability?model_id=%d", s.modelID), nil)
	if err != nil {
		s.logger.Error(ctx, "Failed to create availability check request", map[string]interface{}{
			"error":   err.Error(),
			"modelID": s.modelID,
		})
		return false, fmt.Errorf("creating request: %w", err)
	}

	s.addAuthHeaders(req)

	client := &http.Client{Timeout: 30 * time.Second}

	s.logger.Debug(ctx, "Checking FusionBrain service availability", map[string]interface{}{
		"modelID": s.modelID,
	})
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error(ctx, "Failed to make availability request", map[string]interface{}{
			"error": err.Error(),
		})
		return false, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Debug(ctx, "Received availability response", map[string]interface{}{
		"status_code": resp.StatusCode,
	})

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn(ctx, "Service returned non-OK status", map[string]interface{}{
			"status_code": resp.StatusCode,
		})
		return false, nil
	}

	var status struct {
		ModelStatus string `json:"model_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		s.logger.Error(ctx, "Failed to decode availability response", map[string]interface{}{
			"error": err.Error(),
		})
		return false, fmt.Errorf("decoding response: %w", err)
	}

	s.logger.Debug(ctx, "Retrieved model status", map[string]interface{}{
		"model_status": status.ModelStatus,
	})

	return status.ModelStatus != "DISABLED_BY_QUEUE", nil
}

func (s *FusionBrainServiceImpl) startImageGeneration(ctx context.Context, prompt string) (string, error) {
	startTime := time.Now()
	defer func() {
		metrics.APIResponseTime.Observe(time.Since(startTime).Seconds(), attribute.String("service", "fusion_brain"))
	}()
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
		s.logger.Error(ctx, "Failed to marshal generation parameters", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("marshalling params: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add model_id field
	if err := writer.WriteField("model_id", fmt.Sprintf("%d", s.modelID)); err != nil {
		s.logger.Error(ctx, "Failed to write model_id field", map[string]interface{}{
			"error":    err.Error(),
			"model_id": s.modelID,
		})
		return "", fmt.Errorf("writing model_id: %w", err)
	}

	// Add params field with JSON content type
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", "application/json")
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, "params"))
	part, err := writer.CreatePart(h)
	if err != nil {
		s.logger.Error(ctx, "Failed to create params field", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("creating params field: %w", err)
	}
	if _, err := part.Write(paramsJSON); err != nil {
		s.logger.Error(ctx, "Failed to write params", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("writing params: %w", err)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST",
		fusionBrainBaseURL+"key/api/v1/text2image/run", body)
	if err != nil {
		s.logger.Error(ctx, "Failed to create generation request", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("creating request: %w", err)
	}

	s.addAuthHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error(ctx, "Failed to make generation request", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Error(ctx, "Received unexpected status code", map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        string(body),
		})
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		s.logger.Error(ctx, "Failed to decode generation response", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if response.UUID == "" {
		s.logger.Error(ctx, "No UUID in response", nil)
		return "", fmt.Errorf("no UUID in response")
	}

	s.logger.Info(ctx, "Image generation started", map[string]interface{}{
		"uuid":   response.UUID,
		"prompt": prompt,
	})
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
			s.logger.Error(ctx, "Operation cancelled", map[string]interface{}{
				"error": ctx.Err().Error(),
				"uuid":  uuid,
			})
			return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-ticker.C:
			s.logger.Debug(ctx, "Checking operation status", map[string]interface{}{
				"attempt":      attempt + 1,
				"max_attempts": maxAttempts,
				"uuid":         uuid,
			})

			req, err := http.NewRequestWithContext(ctx, "GET",
				fusionBrainBaseURL+"key/api/v1/text2image/status/"+uuid, nil)
			if err != nil {
				s.logger.Error(ctx, "Failed to create status request", map[string]interface{}{
					"error": err.Error(),
					"uuid":  uuid,
				})
				return nil, fmt.Errorf("creating status request: %w", err)
			}

			s.addAuthHeaders(req)

			resp, err := client.Do(req)
			if err != nil {
				s.logger.Error(ctx, "Status request failed", map[string]interface{}{
					"error": err.Error(),
					"uuid":  uuid,
				})
				if ctx.Err() != nil {
					return nil, fmt.Errorf("operation cancelled during request: %w", ctx.Err())
				}
				continue
			}

			var response StatusResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			resp.Body.Close()
			if err != nil {
				s.logger.Error(ctx, "Failed to decode status response", map[string]interface{}{
					"error": err.Error(),
					"uuid":  uuid,
				})
				if ctx.Err() != nil {
					return nil, fmt.Errorf("operation cancelled during response reading: %w", ctx.Err())
				}
				continue
			}

			s.logger.Debug(ctx, "Received operation status", map[string]interface{}{
				"status": response.Status,
				"uuid":   uuid,
			})
			if response.Status == "DONE" {
				if len(response.Images) == 0 {
					s.logger.Error(ctx, "Operation completed but no images received", map[string]interface{}{
						"uuid": uuid,
					})
					return nil, fmt.Errorf("operation completed but no images received")
				}

				imageData, err := base64.StdEncoding.DecodeString(response.Images[0])
				if err != nil {
					s.logger.Error(ctx, "Failed to decode base64 image", map[string]interface{}{
						"error": err.Error(),
						"uuid":  uuid,
					})
					return nil, fmt.Errorf("decoding base64 image: %w", err)
				}

				s.logger.Info(ctx, "Image generation completed successfully", map[string]interface{}{
					"uuid": uuid,
				})
				return imageData, nil
			} else if response.Status == "FAIL" {
				s.logger.Error(ctx, "Generation failed", map[string]interface{}{
					"error": response.Error,
					"uuid":  uuid,
				})
				return nil, fmt.Errorf("generation failed: %s", response.Error)
			}

			s.logger.Debug(ctx, "Generation in progress", map[string]interface{}{
				"status": response.Status,
				"uuid":   uuid,
			})
		}
	}

	s.logger.Error(ctx, "Operation timed out", map[string]interface{}{
		"attempts": maxAttempts,
		"uuid":     uuid,
	})
	return nil, fmt.Errorf("operation timed out after %d attempts", maxAttempts)
}
