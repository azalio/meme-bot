package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/azalio/meme-bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Создаем корневой контекст приложения с возможностью отмены
	// Этот контекст будет использоваться для graceful shutdown и отмены всех операций
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Гарантируем отмену контекста при выходе из main

	// Инициализируем логгер для централизованного логирования
	// Логгер поддерживает уровни INFO, ERROR и DEBUG (если включен)
	log := logger.New()

	// Загружаем конфигурацию приложения из переменных окружения
	// Конфигурация содержит токены, идентификаторы и другие настройки
	cfg, err := config.New()
	if err != nil {
		log.Error("Failed to load config: %v", err)
		os.Exit(1)
	}

	// Инициализируем сервисы в правильном порядке зависимостей:
	// 1. AuthService - базовый сервис для аутентификации в Yandex Cloud
	authService := service.NewYandexAuthService(cfg, log)

	// 2. GPTService - сервис для работы с YandexGPT, зависит от AuthService
	gptService := service.NewYandexGPTService(cfg, log, authService)

	// 3. ArtService - сервис генерации изображений, зависит от AuthService и GPTService
	artService := service.NewYandexArtService(cfg, log, authService, gptService)

	// 4. BotService - основной сервис Telegram бота, зависит от ArtService
	var botService service.BotService
	botService, err = service.NewBotService(cfg, log, artService)
	if err != nil {
		log.Error("Failed to create bot service: %v", err)
		os.Exit(1)
	}

	// Настраиваем механизм graceful shutdown:
	// 1. sigChan - канал для получения сигналов операционной системы (Ctrl+C, kill)
	sigChan := make(chan os.Signal, 1)
	// 2. shutdownComplete - канал для синхронизации завершения работы
	shutdownComplete := make(chan struct{})
	// Подписываемся на сигналы SIGINT (Ctrl+C) и SIGTERM (kill)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// WaitGroup для отслеживания активных горутин
	// Позволяет дождаться завершения всех операций перед выключением
	var wg sync.WaitGroup

	// Запускаем цикл обработки сообщений в отдельной горутине
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleUpdates(ctx, botService, log)
	}()

	// Ожидаем сигнал остановки
	go func() {
		<-sigChan
		log.Info("Received shutdown signal, stopping gracefully...")
		cancel()

		// Создаем таймаут для graceful shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		// Останавливаем бот
		botService.Stop()

		// Ожидаем завершения всех горутин или таймаута
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Info("All goroutines completed successfully")
		case <-shutdownCtx.Done():
			log.Error("Shutdown timed out")
		}

		close(shutdownComplete)
	}()

	<-shutdownComplete
	log.Info("Shutdown complete")
}

// handleUpdates обрабатывает входящие сообщения от Telegram
// handleUpdates обрабатывает входящие сообщения от Telegram
// Параметры:
// - ctx: контекст для отмены операций
// - bot: сервис бота для взаимодействия с Telegram API
// - log: логгер для записи событий и ошибок
func handleUpdates(ctx context.Context, bot service.BotService, log *logger.Logger) {
    // Настраиваем параметры получения обновлений
    updateConfig := tgbotapi.NewUpdate(0) // 0 означает получение всех новых сообщений
    updateConfig.Timeout = 30 // таймаут long-polling в секундах

    // Получаем канал обновлений от Telegram
    updates := bot.GetUpdatesChan(updateConfig)
    // Канал для асинхронной обработки ошибок
    errorChan := make(chan error, 1)

    for {
        select {
        case <-ctx.Done():
            log.Info("Stopping update handler")
            return
        case err := <-errorChan:
            log.Error("Error handling command: %v", err)
        case update, ok := <-updates:
            if !ok {
                log.Info("Update channel closed")
                return
            }

            if update.Message == nil {
                continue
            }

            log.Info("[%s] %s", update.Message.From.UserName, update.Message.Text)

            if update.Message.IsCommand() {
                command := update.Message.Command()
                args := strings.TrimSpace(update.Message.CommandArguments())

                go func() {
                    cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
                    defer cancel()

                    switch command {
                    case "meme", "help", "start":
                        if err := handleCommand(cmdCtx, bot, update, command, args); err != nil {
                            errorChan <- fmt.Errorf("command %s failed: %w", command, err)
                        }
                    default:
                        if err := bot.SendMessage(cmdCtx, update.Message.Chat.ID, "Я не знаю такой команды"); err != nil {
                            errorChan <- fmt.Errorf("failed to send unknown command message: %w", err)
                        }
                    }
                }()
            }
        }
    }
}

// handleCommand обрабатывает отдельные команды бота
// handleCommand обрабатывает команды бота
// Параметры:
// - ctx: контекст с таймаутом для ограничения времени выполнения
// - bot: сервис бота для отправки сообщений
// - update: структура с информацией о сообщении
// - command: название команды (meme, help, start)
// - args: аргументы команды (текст после команды)
func handleCommand(ctx context.Context, bot service.BotService, update tgbotapi.Update, command, args string) error {
	switch command {
	case "meme":
		// Отправляем сообщение о начале генерации
		if err := bot.SendMessage(ctx, update.Message.Chat.ID, "Генерирую мем, пожалуйста подождите..."); err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}

		imageData, err := bot.HandleCommand(ctx, command, args)
		if err != nil {
			errMsg := fmt.Sprintf("Ошибка генерации мема: %v", err)
			if sendErr := bot.SendMessage(ctx, update.Message.Chat.ID, errMsg); sendErr != nil {
				return fmt.Errorf("failed to send error message: %w", sendErr)
			}
			return fmt.Errorf("failed to generate image: %w", err)
		}

		if err := bot.SendPhoto(ctx, update.Message.Chat.ID, imageData); err != nil {
			errMsg := fmt.Sprintf("Ошибка отправки изображения: %v", err)
			if sendErr := bot.SendMessage(ctx, update.Message.Chat.ID, errMsg); sendErr != nil {
				return fmt.Errorf("failed to send error message: %w", sendErr)
			}
			return fmt.Errorf("failed to send photo: %w", err)
		}

	case "help":
		if err := bot.SendMessage(ctx, update.Message.Chat.ID, `Доступные команды:
/meme [текст] - Генерирует мем с опциональным описанием
/start - Запускает бота
/help - Показывает это сообщение`); err != nil {
			return fmt.Errorf("failed to send help message: %w", err)
		}

	case "start":
		if err := bot.SendMessage(ctx, update.Message.Chat.ID, 
			fmt.Sprintf("Привет, %s! Я бот для генерации мемов. Используй /meme [текст] для создания мема. Например: /meme красная шапочка", 
			update.Message.From.UserName)); err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	}
	return nil
}