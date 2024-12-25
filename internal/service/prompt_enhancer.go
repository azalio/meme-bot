package service

import (
	"context"
	"fmt"

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
func (p *PromptEnhancer) EnhancePrompt(ctx context.Context, originalPrompt string) (string, error) {
	p.logger.Debug("Enhancing prompt: %s", originalPrompt)

	enhancedPrompt, err := p.gptService.GenerateImagePrompt(ctx, originalPrompt)
	if err != nil {
		p.logger.Error("Failed to enhance prompt: %v", err)
		return originalPrompt, fmt.Errorf("enhancing prompt: %w", err)
	}

	p.logger.Debug("Enhanced prompt: %s", enhancedPrompt)
	return enhancedPrompt, nil
}
