package service

import (
	"context"
	"fmt"
	"time"

	"github.com/azalio/meme-bot/internal/otel/metrics"
	"github.com/azalio/meme-bot/pkg/logger"
)

// PromptEnhancer предоставляет функциональность для улучшения промптов
type PromptEnhancer struct {
	logger     *logger.Logger
	gptService YandexGPTService
}

// NewPromptEnhancer создает новый экземпляр PromptEnhancer
func NewPromptEnhancer(log *logger.Logger, gpt YandexGPTService) *PromptEnhancer {
	return &PromptEnhancer{
		logger:     log,
		gptService: gpt,
	}
}

// EnhancePrompt улучшает исходный промпт с помощью GPT
func (p *PromptEnhancer) EnhancePrompt(ctx context.Context, originalPrompt string) (string, string, error) {
	startTime := time.Now()
	defer func() {
		metrics.PromptGenerationTime.Observe(time.Since(startTime).Seconds())
	}()
	p.logger.Debug(ctx, "Starting prompt enhancement", map[string]interface{}{
		"original_prompt": originalPrompt,
		"prompt_length":   len(originalPrompt),
	})
	enhancedPrompt, caption, err := p.gptService.GenerateImagePrompt(ctx, originalPrompt)
	if err != nil {
		p.logger.Error(ctx, "Failed to enhance prompt", map[string]interface{}{
			"error":           err.Error(),
			"original_prompt": originalPrompt,
		})
		return originalPrompt, "", fmt.Errorf("enhancing prompt: %w", err)
	}

	p.logger.Debug(ctx, "Successfully enhanced prompt", map[string]interface{}{
		"original_prompt": originalPrompt,
		"enhanced_prompt": enhancedPrompt,
		"caption":         caption,
		"original_length": len(originalPrompt),
		"enhanced_length": len(enhancedPrompt),
	})
	return enhancedPrompt, caption, nil
}
