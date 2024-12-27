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
// Загружает переменные окружения из .env файла
// Если файл не найден, пытается получить переменные из окружения
// envPath - путь к директории, где находится .env файл (по умолчанию текущая директория)
// envName - имя .env файла (по умолчанию ".env")
func New(envPath, envName string, logger *logger.Logger) (*Config, error) {
	// Если путь не указан, используем текущую директорию
	if envPath == "" {
		envPath = "."
	}
	// Если имя файла не указано, используем ".env"
	if envName == "" {
		envName = ".env"
	}

	// Формируем полный путь к .env файлу
	envFile := filepath.Join(envPath, envName)

	// Пытаемся загрузить .env файл по указанному пути
	if err := godotenv.Load(envFile); err != nil {
		// Если файл не найден, логируем это и пробуем загрузить .env из корневой директории приложения
		logger.Warn(context.Background(), "Error loading .env file from specified path, trying root directory", map[string]interface{}{
			"error": err,
			"path":  envFile,
		})

		// Пытаемся загрузить .env из корневой директории приложения
		rootEnvFile := filepath.Join(".", envName)
		if err := godotenv.Load(rootEnvFile); err != nil {
			// Если файл не найден и в корневой директории, логируем это, но продолжаем работу
			// так как переменные могут быть установлены в окружении
			logger.Warn(context.Background(), "Error loading .env file from root directory", map[string]interface{}{
				"error": err,
				"path":  rootEnvFile,
			})
		}
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
