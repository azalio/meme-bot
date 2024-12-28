// Package config содержит конфигурацию приложения
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/joho/godotenv"
)

// Config представляет структуру конфигурации приложения
type Config struct {
	// Токен для Telegram бота
	TelegramToken string
	// OAuth токен для Yandex Cloud
	YandexOAuthToken string
	// IAM токен для Yandex Cloud
	YandexIAMToken string
	// ID папки в Yandex Cloud для ART
	YandexArtFolderID string
	// MEME_DEBUG включение дебаг уровня
	MemeDebug string
}

// New создает новый экземпляр конфигурации
// Загружает переменные окружения из указанного файла
// Если файл не указан, использует .env в текущей директории
// envFile - путь к файлу конфигурации (по умолчанию ".env")
func New(envFile string, logger *logger.Logger) (*Config, error) {
	// Если путь к файлу не указан, используем текущую директорию и файл ".env"
	if envFile == "" {
		envFile = ".env"
	}

	// Пытаемся загрузить указанный файл
	if err := godotenv.Load(envFile); err != nil {
		// Если файл не найден, логируем это, но продолжаем работу
		logger.Warn(context.Background(), "Error loading .env file", map[string]interface{}{
			"error": err,
			"path":  envFile,
		})
	}

	// Получаем необходимые переменные окружения
	config := &Config{
		TelegramToken:     os.Getenv("TELEGRAM_BOT_TOKEN"),
		YandexOAuthToken:  os.Getenv("YANDEX_OAUTH_TOKEN"),
		YandexIAMToken:    os.Getenv("YANDEX_IAM_TOKEN"),
		YandexArtFolderID: os.Getenv("YANDEX_ART_FOLDER_ID"),
		MemeDebug:         os.Getenv("MEME_DEBUG"),
	}

	// Проверяем наличие обязательных переменных
	if config.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}
	if config.YandexOAuthToken == "" {
		return nil, fmt.Errorf("YANDEX_OAUTH_TOKEN not set")
	}
	if config.YandexArtFolderID == "" {
		return nil, fmt.Errorf("YANDEX_ART_FOLDER_ID not set")
	}

	return config, nil
}
