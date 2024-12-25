package service

import (
	"context"
	"fmt"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
)

// ImageGenerationService provides a unified interface for image generation
type ImageGenerationService struct {
	fusionBrain *FusionBrainServiceImpl
	yandexArt   *YandexArtServiceImpl
	logger      *logger.Logger
}

// NewImageGenerationService creates a new instance of ImageGenerationService
func NewImageGenerationService(
	cfg *config.Config,
	log *logger.Logger,
	auth YandexAuthService,
	gpt YandexGPTService,
) *ImageGenerationService {
	return &ImageGenerationService{
		fusionBrain: NewFusionBrainService(log),
		yandexArt:   NewYandexArtService(cfg, log, auth, gpt),
		logger:      log,
	}
}

// GenerateImage attempts to generate an image using available services
func (s *ImageGenerationService) GenerateImage(ctx context.Context, promptText string) ([]byte, error) {
	// First try FusionBrain
	if s.fusionBrain != nil {
		s.logger.Info("Attempting to generate image using FusionBrain")
		imageData, err := s.fusionBrain.GenerateImage(ctx, promptText)
		if err == nil {
			s.logger.Info("Successfully generated image using FusionBrain")
			return imageData, nil
		}
		s.logger.Error("FusionBrain generation failed: %v, falling back to YandexArt", err)
	}

	// Fallback to YandexArt
	s.logger.Info("Attempting to generate image using YandexArt")
	imageData, err := s.yandexArt.GenerateImage(ctx, promptText)
	if err != nil {
		s.logger.Error("YandexArt generation failed: %v", err)
		return nil, fmt.Errorf("all image generation services failed: %w", err)
	}

	s.logger.Info("Successfully generated image using YandexArt")
	return imageData, nil
}
