package service

import (
	"context"
)

// ImageGenerator defines the interface for image generation services
type ImageGenerator interface {
	GenerateImage(ctx context.Context, promptText string) ([]byte, error)
}
