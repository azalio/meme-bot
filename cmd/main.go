// Package main представляет собой точку входа в приложение Telegram бота для генерации мемов.
// Бот использует чистую архитектуру, паттерны проектирования и лучшие практики Go
// для обеспечения надежности, масштабируемости и удобства поддержки.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"flag"
	"syscall"
	"time"

	"github.com/azalio/meme-bot/internal/config"
	"github.com/azalio/meme-bot/internal/otel/metrics"
	"github.com/azalio/meme-bot/internal/service"
	"github.com/azalio/meme-bot/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Константы для настройки таймаутов и лимитов
const (
	shutdownTimeout = 30 * time.Second
	commandTimeout  = 5 * time.Minute
	workerPoolSize  = 10
)

// App представляет основную структуру приложения
// Application State Pattern: Хранение состояния приложения в единой структуре
type App struct {
	bot     *service.BotServiceImpl
	log     *logger.Logger
	metrics *metrics.MetricProvider
	wg      sync.WaitGroup
}

// newApp создает новый экземпляр приложения
// Factory Pattern: Создание сложного объекта через фабричный метод
func newApp() (*App, error) {
	// Инициализируем логгер
	logLevel := logger.InfoLevel

	log, err := logger.New(logger.Config{
		Level:     logLevel,
		Service:   "meme-bot",
		Env:       os.Getenv("ENVIRONMENT"),
		GitCommit: os.Getenv("GIT_COMMIT"),
	})

	// Добавляем обработку аргументов командной строки
	envFile := flag.String("config", "", "Path to the .env file")
	flag.Parse()

	// Загружаем конфигурацию
	// Так как конфигуарция тоже нуждается в логгировании,
	// но дебаг уровень выставлен в конфигуарции,
	// то сначала создаем логгер, потом уже устанавливаем дебаг уровень.
	cfg, err := config.New(*envFile, log)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Проверяем что включен DEBUG уровень логгирования
	if cfg.MemeDebug == "1" {
		logLevel = logger.DebugLevel
		log.SetLevel(logLevel)
	}

	log.Debug(context.Background(), "Configuration loaded successfully", nil)

	log.Info(context.Background(), "Get logger level", map[string]interface{}{
		"logger level": log.GetLevel().String(),
	})

	log.Info(context.Background(), log.GetLevel().String(), nil)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	log.Debug(context.Background(), "Logger initialized successfully", nil)

	// Инициализируем метрики
	mp, err := metrics.InitMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}
	log.Debug(context.Background(), "Metrics initialized successfully", nil)

	// Инициализируем сервисы
	// Builder Pattern: Пошаговое создание сложного объекта
	authService := service.NewYandexAuthService(cfg, log)
	log.Debug(context.Background(), "Auth service initialized successfully", nil)

	gptService := service.NewYandexGPTService(cfg, log, authService)

	botService, err := service.NewBotService(cfg, log, authService, gptService)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot service: %w", err)
	}
	log.Debug(context.Background(), "Bot service initialized successfully", nil)

	return &App{
		bot:     botService,
		log:     log,
		metrics: mp,
	}, nil
}

// startHealthServer запускает HTTP сервер для health checks
// Health Check Pattern: Отдельный эндпоинт для проверки здоровья сервиса
func (a *App) startHealthServer(ctx context.Context) {

	a.log.Debug(ctx, "Starting health server", map[string]interface{}{
		"port": 8081,
	})

	mux := http.NewServeMux()

	// Liveness probe
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Readiness probe
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Здесь можно добавить проверки готовности сервисов
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.Error(ctx, "Health server failed", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Graceful shutdown для health сервера
	go func() {
		<-ctx.Done()
		a.log.Debug(ctx, "Shutting down health server", nil)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			a.log.Error(ctx, "Health server shutdown failed", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()
}

// run запускает основной цикл приложения
// Command Pattern: Инкапсуляция всех операций по запуску приложения
func (a *App) run(ctx context.Context) error {
	// run запускает основной цикл приложения
	// Command Pattern: Инкапсуляция всех операций по запуску приложения
	// Запускаем health checks

	a.log.Debug(ctx, "Starting health server", nil)
	a.startHealthServer(ctx)

	// Запускаем сервер метрик
	a.log.Debug(ctx, "Starting metrics server", nil)
	metrics.StartMetricsServer()

	// Запускаем обработчик обновлений
	a.log.Debug(ctx, "Starting update handler", nil)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.handleUpdates(ctx)
	}()

	return nil
}

// shutdown выполняет корректное завершение работы приложения
// Graceful Shutdown Pattern: Корректное завершение всех компонентов
func (a *App) shutdown(ctx context.Context) {
	a.log.Info(ctx, "Starting graceful shutdown", nil)

	// Останавливаем бота
	a.log.Info(ctx, "Stopping bot", nil)
	a.bot.Stop()

	// Ожидаем завершения всех горутин
	a.log.Info(ctx, "Waiting for goroutines to complete", nil)
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	// Ожидаем завершения или таймаута
	select {
	case <-done:
		a.log.Info(ctx, "All goroutines completed successfully", nil)
	case <-ctx.Done():
		a.log.Error(ctx, "Shutdown timed out", nil)
	}

	// Останавливаем метрики
	if err := a.metrics.Shutdown(context.Background()); err != nil {
		a.log.Error(ctx, "Metrics shutdown failed", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// handleUpdates обрабатывает входящие сообщения от Telegram
// Worker Pool Pattern: Ограничение количества одновременных обработчиков
// Метод использует пул горутин для обработки команд, чтобы избежать перегрузки системы.
func (a *App) handleUpdates(ctx context.Context) {
	// Логируем начало работы обработчика обновлений
	a.log.Info(ctx, "Starting update handler", nil)
	// Логируем завершение работы обработчика обновлений при выходе из функции
	defer func() {
		a.log.Info(ctx, "Update handler stopped", nil)
	}()

	// Создаем конфигурацию для получения обновлений от Telegram
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30 // Таймаут для получения обновлений

	// Получаем канал обновлений от Telegram бота
	updates := a.bot.GetUpdatesChan(updateConfig)

	// Создаем канал для передачи ошибок, возникающих при обработке команд
	errorChan := make(chan error, 1)

	// Создаем пул горутин для ограничения количества одновременных обработчиков
	workerPool := make(chan struct{}, workerPoolSize)

	// Основной цикл обработки обновлений
	for {
		select {
		case <-ctx.Done():
			// Если контекст завершен, останавливаем обработчик обновлений
			a.log.Info(ctx, "Stopping update handler", nil)
			return
		case err := <-errorChan:
			// Если произошла ошибка при обработке команда, логируем её
			a.log.Error(ctx, "Error handling command", map[string]interface{}{
				"error": err.Error(),
			})
		case update, ok := <-updates:
			// Получаем обновление из канала
			if !ok {
				// Если канал обновлений закрыт, завершаем работу обработчика
				a.log.Info(ctx, "Update channel closed", nil)
				return
			}

			// Если обновление не содержит сообщения, пропускаем его
			if update.Message == nil {
				continue
			}

			// Логируем полученное сообщение
			a.log.Info(ctx, "Received message", map[string]interface{}{
				"user":    update.Message.From.UserName,
				"message": update.Message.Text,
			})

			// Если сообщение является командой, обрабатываем её
			if update.Message.IsCommand() {
				// Увеличиваем счетчик WaitGroup для отслеживания активных горутин
				a.wg.Add(1)
				go func(update tgbotapi.Update) {
					// Занимаем слот в пуле горутин
					// Горутина занимает слот в пуле, отправляя пустую структуру в канал.
					// Если все слоты заняты, выполнение блокируется до освобождения одного из них.
					workerPool <- struct{}{}
					// Освобождаем слот в пуле горутин при завершении обработки
					// После завершения выполнения задачи горутина освобождает слот, читая из канала.
					// Это позволяет другим горутинам занять освободившийся слот.
					defer func() { <-workerPool }()
					// Уменьшаем счетчик WaitGroup при завершении обработки
					defer a.wg.Done()

					// Извлекаем команду и аргументы из сообщения
					command := update.Message.Command()
					args := strings.TrimSpace(update.Message.CommandArguments())

					// Создаем контекст с таймаутом для обработки команды
					cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
					// Отменяем контекст при завершении обработки
					defer cancel()

					// Обрабатываем команду и передаем ошибку в канал, если она возникла
					if err := a.handleCommand(cmdCtx, update, command, args); err != nil {
						errorChan <- fmt.Errorf("command %s failed: %w", command, err)
					}
				}(update)
			}
		}
	}
}

// handleCommand обрабатывает команды бота
// Strategy Pattern: Выбор стратегии обработки в зависимости от команды
func (a *App) handleCommand(ctx context.Context, update tgbotapi.Update, command, args string) error {
	a.log.Info(ctx, "Processing command", map[string]interface{}{
		"command": command,
		"args":    args,
		"user":    update.Message.From.UserName,
		"chat_id": update.Message.Chat.ID,
	})

	switch command {
	case "meme":
		return a.handleMemeCommand(ctx, update, args)
	case "help":
		return a.handleHelpCommand(ctx, update)
	case "start":
		return a.handleStartCommand(ctx, update)
	default:
		return a.handleUnknownCommand(ctx, update)
	}
}

// handleMemeCommand обрабатывает команду генерации мема
// Template Method Pattern: Определяет скелет алгоритма генерации мема
func (a *App) handleMemeCommand(ctx context.Context, update tgbotapi.Update, args string) error {
	// Metrics Pattern: Увеличиваем счетчик использования команды
	metrics.CommandCounter.Inc("meme")

	// Step 1: Отправляем сообщение о начале генерации
	processingMsg, err := a.bot.SendMessage(ctx, update.Message.Chat.ID, "Генерирую мем, пожалуйста подождите...")
	if err != nil {
		a.log.Error(ctx, "Failed to send start message", map[string]interface{}{
			"error":     err.Error(),
			"chat_id":  update.Message.Chat.ID,
			"user":     update.Message.From.UserName,
			"command":  "meme",
			"function": "handleMemeCommand",
		})
		return fmt.Errorf("failed to send start message: %w", err)
	}

	// Step 2: Засекаем время для метрик
	startTime := time.Now()
	defer func() {
		// Metrics Pattern: Записываем время генерации мема
		metrics.GenerationDuration.Observe(time.Since(startTime).Seconds())
	}()

	// Step 3: Генерируем мем
	imageData, err, caption := a.bot.HandleCommand(ctx, "meme", args)
	if err != nil {
		// Metrics Pattern: Увеличиваем счетчик ошибок
		metrics.ErrorCounter.Inc("meme_generation")

		errMsg := fmt.Sprintf("Ошибка генерации мема: %v", err)
		if _, sendErr := a.bot.SendMessage(ctx, update.Message.Chat.ID, errMsg); sendErr != nil {
			a.log.Error(ctx, "Failed to send error message", map[string]interface{}{
				"error":     sendErr.Error(),
				"orig_err":  err.Error(),
				"chat_id":   update.Message.Chat.ID,
				"user_name": update.Message.From.UserName,
				"command":   "meme",
				"function":  "handleMemeCommand",
				"prompt":    args,
			})
		}
		return fmt.Errorf("failed to generate image: %w", err)
	}

	// Step 4: Удаляем сообщение о генерации
	// Fail Gracefully Pattern: Продолжаем даже при ошибке удаления
	if err := a.bot.DeleteMessage(ctx, update.Message.Chat.ID, processingMsg.MessageID); err != nil {
		a.log.Error(ctx, "Failed to delete generation message", map[string]interface{}{
			"error":    err.Error(),
			"chat_id":  update.Message.Chat.ID,
			"msg_id":   processingMsg.MessageID,
			"command":  "meme",
			"function": "handleMemeCommand",
			"user":     update.Message.From.UserName,
		})
	}

	// Step 5: Отправляем сгенерированный мем
	if err := a.bot.SendPhoto(ctx, update.Message.Chat.ID, imageData, caption); err != nil {
		// Metrics Pattern: Увеличиваем счетчик ошибок отправки
		metrics.ErrorCounter.Inc("meme_sending")

		errMsg := fmt.Sprintf("Ошибка отправки изображения: %v", err)
		if _, sendErr := a.bot.SendMessage(ctx, update.Message.Chat.ID, errMsg); sendErr != nil {
			a.log.Error(ctx, "Failed to send photo error message", map[string]interface{}{
				"error":     sendErr.Error(),
				"orig_err":  err.Error(),
				"chat_id":   update.Message.Chat.ID,
				"user_name": update.Message.From.UserName,
			})
		}
		return fmt.Errorf("failed to send photo: %w", err)
	}

	// Step 6: Логируем успешное выполнение
	a.log.Info(ctx, "Meme generated and sent successfully", map[string]interface{}{
		"user":     update.Message.From.UserName,
		"chat_id":  update.Message.Chat.ID,
		"duration": time.Since(startTime).String(),
	})

	return nil
}

// handleHelpCommand обрабатывает команду помощи
func (a *App) handleHelpCommand(ctx context.Context, update tgbotapi.Update) error {
	metrics.CommandCounter.Inc("help")

	helpText := `Доступные команды:
/meme [текст] - Генерирует мем с опциональным описанием
/start - Запускает бота
/help - Показывает это сообщение`

	if _, err := a.bot.SendMessage(ctx, update.Message.Chat.ID, helpText); err != nil {
		metrics.ErrorCounter.Inc("help_message")
		a.log.Error(ctx, "Failed to send help message", map[string]interface{}{
			"error":   err.Error(),
			"chat_id": update.Message.Chat.ID,
			"user":    update.Message.From.UserName,
		})
		return fmt.Errorf("failed to send help message: %w", err)
	}

	return nil
}

// handleStartCommand обрабатывает команду начала работы с ботом
func (a *App) handleStartCommand(ctx context.Context, update tgbotapi.Update) error {
	metrics.CommandCounter.Inc("start")

	welcomeMsg := fmt.Sprintf(
		"Привет, %s! Я бот для генерации мемов. Используй /meme [текст] для создания мема. "+
			"Например: /meme красная шапочка",
		update.Message.From.UserName,
	)

	if _, err := a.bot.SendMessage(ctx, update.Message.Chat.ID, welcomeMsg); err != nil {
		metrics.ErrorCounter.Inc("start_message")
		a.log.Error(ctx, "Failed to send start message", map[string]interface{}{
			"error":   err.Error(),
			"chat_id": update.Message.Chat.ID,
			"user":    update.Message.From.UserName,
		})
		return fmt.Errorf("failed to send start message: %w", err)
	}

	return nil
}

// handleUnknownCommand обрабатывает неизвестные команды
func (a *App) handleUnknownCommand(ctx context.Context, update tgbotapi.Update) error {
	metrics.CommandCounter.Inc("unknown")

	if _, err := a.bot.SendMessage(ctx, update.Message.Chat.ID, "Я не знаю такой команды"); err != nil {
		metrics.ErrorCounter.Inc("unknown_command_message")
		a.log.Error(ctx, "Failed to send unknown command message", map[string]interface{}{
			"error":   err.Error(),
			"chat_id": update.Message.Chat.ID,
			"user":    update.Message.From.UserName,
		})
		return fmt.Errorf("failed to send unknown command message: %w", err)
	}

	return nil
}

// main - точка входа в приложение
func main() {
	// Создаем корневой контекст
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализируем приложение
	app, err := newApp()
	if err != nil {
		fmt.Printf("Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	app.log.Debug(ctx, "Application initialized successfully", nil)

	// Настраиваем обработку сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем приложение
	if err := app.run(ctx); err != nil {
		app.log.Fatal(ctx, "Failed to run application", map[string]interface{}{
			"error": err.Error(),
		})
	}

	app.log.Info(ctx, "Application started successfully", nil)

	// Ожидаем сигнал завершения
	<-sigChan
	app.log.Info(ctx, "Received shutdown signal", nil)

	// Создаем контекст с таймаутом для graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Выполняем graceful shutdown
	app.shutdown(shutdownCtx)
	app.log.Info(ctx, "Shutdown complete", nil)
}
