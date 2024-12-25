// Package service содержит интерфейсы и реализацию бизнес-логики
package service

import (
    "context"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// YandexAuthService определяет интерфейс для работы с аутентификацией Yandex
type YandexAuthService interface {
    // GetIAMToken получает или обновляет IAM токен
    GetIAMToken(ctx context.Context) (string, error)
    // RefreshIAMToken обновляет IAM токен
    RefreshIAMToken(ctx context.Context, oauthToken string) (string, error)
}

// YandexGPTService определяет интерфейс для работы с Yandex GPT
type YandexGPTService interface {
    // GenerateImagePrompt генерирует промпт для создания изображения
    GenerateImagePrompt(ctx context.Context, userPrompt string) (string, error)
}

// ImageGenerator определяет интерфейс для сервисов генерации изображений.
// Может быть реализован различными провайдерами (Yandex Art, Stable Diffusion, DALL-E и т.д.)
type ImageGenerator interface {
    // GenerateImage генерирует изображение на основе текстового промпта
    // ctx - контекст выполнения
    // promptText - текстовое описание желаемого изображения
    // Возвращает сгенерированное изображение в виде []byte и ошибку, если она возникла
    GenerateImage(ctx context.Context, promptText string) ([]byte, error)
}

// BotService определяет интерфейс для работы с телеграм ботом
type BotService interface {
    // GetUpdatesChan возвращает канал для получения обновлений от Telegram
    GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
    // HandleCommand обрабатывает команды бота
    HandleCommand(ctx context.Context, command string, args string) ([]byte, error)
    // SendMessage отправляет текстовое сообщение
    SendMessage(ctx context.Context, chatID int64, message string) error
    // SendPhoto отправляет фото
    SendPhoto(ctx context.Context, chatID int64, photo []byte) error
    // Stop останавливает работу бота
    Stop()
}