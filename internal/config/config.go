// Package config содержит конфигурацию приложения
package config

import (
	"fmt"
	"os"

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
// Загружает переменные окружения из .env файла
// Если файл не найден, пытается получить переменные из окружения
func New() (*Config, error) {
	// Пытаемся загрузить .env файл
	if err := godotenv.Load(); err != nil {
		// Если файл не найден, логируем это, но продолжаем работу
		// так как переменные могут быть установлены в окружении
		fmt.Printf("Warning: Error loading .env file: %v\n", err)
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
