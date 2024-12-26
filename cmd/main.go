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
	"github.com/azalio/meme-bot/internal/otel/metrics"
	"github.com/azalio/meme-bot/internal/service"
	"github.com/azalio/meme-bot/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// main - точка входа в программу.
func main() {
	// Создаем контекст с возможностью отмены. Это позволяет управлять жизненным циклом горутин.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Гарантируем, что контекст будет отменен при завершении программы.

	// Инициализируем логгер для записи сообщений.
	log := logger.New()

	// Загружаем конфигурацию приложения. Конфигурация может содержать токены, настройки и т.д.
	cfg, err := config.New()
	if err != nil {
		log.Fatal("Failed to load config: %v", err)
	}

	// Инициализируем метрики и экспортер Prometheus.
	mp, err := metrics.InitMetrics()
	if err != nil {
		log.Fatal("Failed to initialize metrics: %v", err)
	}
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Info("Error shutting down meter provider: %v", err)
		}
	}()

	// Запускаем сервер метрик.
	metrics.StartMetricsServer()

	// Инициализируем сервис аутентификации для работы с Yandex Cloud.
	authService := service.NewYandexAuthService(cfg, log)

	// Инициализируем сервис для работы с Yandex GPT.
	gptService := service.NewYandexGPTService(cfg, log, authService)

	// Инициализируем сервис Telegram бота.
	var botService *service.BotServiceImpl
	botService, err = service.NewBotService(cfg, log, authService, gptService)
	if err != nil {
		log.Error("Failed to create bot service: %v", err)
		os.Exit(1)
	}

	// Настраиваем канал для обработки сигналов завершения программы (например, Ctrl+C).
	sigChan := make(chan os.Signal, 1)
	shutdownComplete := make(chan struct{})
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// WaitGroup используется для ожидания завершения всех горутин перед завершением программы.
	var wg sync.WaitGroup

	// Запускаем обработку обновлений от Telegram в отдельной горутине.
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleUpdates(ctx, botService, log)
	}()

	// Ожидаем сигнал завершения программы.
	go func() {
		<-sigChan
		log.Info("Received shutdown signal, stopping gracefully...")
		cancel() // Отменяем контекст, чтобы остановить все горутины.

		// Создаем контекст с таймаутом для graceful shutdown.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		// Останавливаем сервис бота.
		botService.Stop()

		// Ожидаем завершения всех горутин.
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		// Ожидаем либо завершения всех горутин, либо истечения таймаута.
		select {
		case <-done:
			log.Info("All goroutines completed successfully")
		case <-shutdownCtx.Done():
			log.Error("Shutdown timed out")
		}

		// Сигнализируем о завершении shutdown.
		close(shutdownComplete)
	}()

	// Ожидаем завершения всех операций перед выходом из программы.
	<-shutdownComplete
	log.Info("Shutdown complete")
}

// handleUpdates обрабатывает входящие обновления от Telegram.
func handleUpdates(ctx context.Context, bot *service.BotServiceImpl, log *logger.Logger) {
	// Настраиваем параметры получения обновлений.
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	// Получаем канал обновлений от Telegram.
	updates := bot.GetUpdatesChan(updateConfig)

	// Канал для асинхронной обработки ошибок.
	errorChan := make(chan error, 1)

	// WaitGroup для отслеживания активных горутин.
	var wg sync.WaitGroup

	// Пул горутин: ограничиваем количество одновременно выполняемых горутин.
	workerPool := make(chan struct{}, 10)

	// Основной цикл обработки обновлений.
	for {
		select {
		case <-ctx.Done():
			// Если контекст отменен, завершаем обработку обновлений.
			log.Info("Stopping update handler")
			wg.Wait() // Ожидаем завершения всех горутин.
			return
		case err := <-errorChan:
			// Логируем ошибки, возникшие при обработке команд.
			log.Error("Error handling command: %v", err)
		case update, ok := <-updates:
			if !ok {
				// Если канал обновлений закрыт, завершаем обработку.
				log.Info("Update channel closed")
				return
			}

			if update.Message == nil {
				// Пропускаем обновления без сообщений.
				continue
			}

			// Логируем полученное сообщение.
			log.Info("[%s] %s", update.Message.From.UserName, update.Message.Text)

			if update.Message.IsCommand() {
				// Если сообщение является командой, запускаем обработку в отдельной горутине.
				wg.Add(1)
				go func() {
					workerPool <- struct{}{}        // Занимаем слот в пуле горутин.
					defer func() { <-workerPool }() // Освобождаем слот после завершения.
					defer wg.Done()

					command := update.Message.Command()
					args := strings.TrimSpace(update.Message.CommandArguments())

					// Создаем контекст с таймаутом для обработки команды.
					cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
					defer cancel()

					// Обрабатываем команду.
					switch command {
					case "meme", "help", "start":
						if err := handleCommand(cmdCtx, bot, update, command, args, log); err != nil {
							errorChan <- fmt.Errorf("command %s failed: %w", command, err)
						}
					default:
						// Если команда неизвестна, отправляем сообщение об ошибке.
						if _, err := bot.SendMessage(cmdCtx, update.Message.Chat.ID, "Я не знаю такой команды"); err != nil {
							errorChan <- fmt.Errorf("failed to send unknown command message: %w", err)
						}
					}
				}()
			}
		}
	}
}

// handleCommand обрабатывает конкретные команды бота.
func handleCommand(ctx context.Context, bot *service.BotServiceImpl, update tgbotapi.Update, command, args string, log *logger.Logger) error {
	switch command {
	case "meme":
		// Отправляем сообщение о начале генерации мема.
		processingMsg, err := bot.SendMessage(ctx, update.Message.Chat.ID, "Генерирую мем, пожалуйста подождите...")
		if err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}

		// Генерируем мем.
		imageData, err := bot.HandleCommand(ctx, command, args)
		if err != nil {
			// В случае ошибки отправляем сообщение об ошибке.
			errMsg := fmt.Sprintf("Ошибка генерации мема: %v", err)
			if _, sendErr := bot.SendMessage(ctx, update.Message.Chat.ID, errMsg); sendErr != nil {
				return fmt.Errorf("failed to send error message: %w", sendErr)
			}
			return fmt.Errorf("failed to generate image: %w", err)
		}

		// Удаляем сообщение о генерации.
		if err := bot.DeleteMessage(ctx, update.Message.Chat.ID, processingMsg.MessageID); err != nil {
			fmt.Printf("Failed to delete generation message: %v", err)
		}

		// Отправляем сгенерированный мем.
		if err := bot.SendPhoto(ctx, update.Message.Chat.ID, imageData); err != nil {
			errMsg := fmt.Sprintf("Ошибка отправки изображения: %v", err)
			if _, sendErr := bot.SendMessage(ctx, update.Message.Chat.ID, errMsg); sendErr != nil {
				return fmt.Errorf("failed to send error message: %w", sendErr)
			}
			return fmt.Errorf("failed to send photo: %w", err)
		}

	case "help":
		// Отправляем сообщение с помощью по команде /help.
		if _, err := bot.SendMessage(ctx, update.Message.Chat.ID, `Доступные команды:
/meme [текст] - Генерирует мем с опциональным описанием
/start - Запускает бота
/help - Показывает это сообщение`); err != nil {
			return fmt.Errorf("failed to send help message: %w", err)
		}

	case "start":
		// Отправляем приветственное сообщение по команде /start.
		if _, err := bot.SendMessage(ctx, update.Message.Chat.ID,
			fmt.Sprintf("Привет, %s! Я бот для генерации мемов. Используй /meme [текст] для создания мема. Например: /meme красная шапочка",
				update.Message.From.UserName)); err != nil {
			return fmt.Errorf("failed to send start message: %w", err)
		}
	}
	return nil
}
