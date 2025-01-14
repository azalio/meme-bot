package service

import (
	"context"
	"fmt"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/internal/otel/metrics"
	"github.com/azalio/meme-bot/pkg/logger"
)

// ImageGenerationService provides a unified interface for image generation
type ImageGenerationService struct {
	fusionBrain  *FusionBrainServiceImpl
	yandexArt    *YandexArtServiceImpl
	cloudflareAI *CloudflareAIServiceImpl
	logger       *logger.Logger
}

// NewImageGenerationService creates a new instance of ImageGenerationService
func NewImageGenerationService(
	cfg *config.Config,
	log *logger.Logger,
	auth YandexAuthService,
	gpt YandexGPTService,
) *ImageGenerationService {
	return &ImageGenerationService{
		fusionBrain:  NewFusionBrainService(log),
		yandexArt:    NewYandexArtService(cfg, log, auth, gpt),
		cloudflareAI: NewCloudflareAIService(log),
		logger:       log,
	}
}

// GenerateImage attempts to generate an image using available services
func (s *ImageGenerationService) GenerateImage(ctx context.Context, promptText string) ([]byte, error) {
	// Создаем каналы для получения результатов и ошибок
	resultChan := make(chan []byte)
	errorChan := make(chan error)

	// Запускаем генерацию изображений в параллельных горутинах
	go func() {
		if s.fusionBrain != nil {
			s.logger.Info(ctx, "Attempting FusionBrain image generation", map[string]interface{}{
				"prompt_length": len(promptText),
			})

			imageData, err := s.fusionBrain.GenerateImage(ctx, promptText)
			if err == nil {
				s.logger.Info(ctx, "Successfully generated image with FusionBrain", map[string]interface{}{
					"image_size": len(imageData),
				})
				metrics.FusionBrainSuccessCounter.Inc("success")
				resultChan <- imageData
				return
			}
			s.logger.Error(ctx, "FusionBrain generation failed", map[string]interface{}{
				"error": err.Error(),
			})
			metrics.FusionBrainFailureCounter.Inc("failure")
		}
		errorChan <- fmt.Errorf("FusionBrain generation failed")
	}()

	go func() {
		s.logger.Info(ctx, "Attempting YandexArt image generation", map[string]interface{}{
			"prompt_length": len(promptText),
		})
		imageData, err := s.yandexArt.GenerateImage(ctx, promptText)
		if err == nil {
			s.logger.Info(ctx, "Successfully generated image with YandexArt", map[string]interface{}{
				"image_size": len(imageData),
			})
			metrics.YandexArtSuccessCounter.Inc("success")
			resultChan <- imageData
			return
		}
		s.logger.Error(ctx, "YandexArt generation failed", map[string]interface{}{
			"error": err.Error(),
		})
		metrics.YandexArtFailureCounter.Inc("failure")
		errorChan <- fmt.Errorf("YandexArt generation failed")
	}()

	// Добавляем третью горутину для Cloudflare AI
	go func() {
		if s.cloudflareAI != nil {
			s.logger.Info(ctx, "Attempting Cloudflare AI image generation", map[string]interface{}{
				"prompt_length": len(promptText),
			})

			imageData, err := s.cloudflareAI.GenerateImage(ctx, promptText)
			if err == nil {
				s.logger.Info(ctx, "Successfully generated image with Cloudflare AI", map[string]interface{}{
					"image_size": len(imageData),
				})
				metrics.CloudflareAISuccessCounter.Inc("success")
				resultChan <- imageData
				return
			}
			s.logger.Error(ctx, "Cloudflare AI generation failed", map[string]interface{}{
				"error": err.Error(),
			})
			metrics.CloudflareAIFailureCounter.Inc("failure")
		}
		errorChan <- fmt.Errorf("Cloudflare AI generation failed")
	}()

	// Ожидаем первый успешный результат или все ошибки
	var errors []error

	for i := 0; i < 3; i++ {
		select {
		case imageData := <-resultChan:
			return imageData, nil
		case err := <-errorChan:
			errors = append(errors, err)
			if len(errors) == 2 {
				return nil, fmt.Errorf("all image generation services failed: %w", errors[0])
			}
		}
	}

	return nil, fmt.Errorf("unexpected error: no results received")
}
