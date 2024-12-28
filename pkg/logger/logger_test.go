package logger_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// Переменная для перехвата os.Exit
var osExit = os.Exit

// NewTestLogger создает логгер для тестирования
func NewTestLogger() *logger.Logger {
	log, _ := logger.New(logger.Config{
		Level:   logger.DebugLevel,
		Service: "test",
	})
	return log
}

func TestLogger_Info(t *testing.T) {
	// Создаем логгер для тестов через конструктор
	log, err := logger.New(logger.Config{
		Level:   logger.InfoLevel,
		Service: "test-service",
	})
	assert.NoError(t, err)

	// Перенаправляем stdout для захвата вывода
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Выполняем логирование
	log.Info(context.Background(), "Test info message", map[string]interface{}{
		"key": "value",
	})

	// Восстанавливаем stdout
	w.Close()
	os.Stdout = oldStdout

	// Читаем захваченный вывод
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Проверяем, что вывод содержит ожидаемые данные
	assert.Contains(t, output, `"level":"INFO"`)
	assert.Contains(t, output, `"message":"Test info message"`)
	assert.Contains(t, output, `"key":"value"`)
	assert.Contains(t, output, `"service":"test-service"`)
}

func TestLogger_Levels(t *testing.T) {
	assert.Equal(t, "DEBUG", logger.DebugLevel.String())
	assert.Equal(t, "INFO", logger.InfoLevel.String())
	assert.Equal(t, "WARN", logger.WarnLevel.String())
	assert.Equal(t, "ERROR", logger.ErrorLevel.String())
	assert.Equal(t, "FATAL", logger.FatalLevel.String())
	assert.Equal(t, "UNKNOWN", logger.Level(100).String())
}

func TestLogger_Debug(t *testing.T) {
	log, err := logger.New(logger.Config{
		Level:   logger.DebugLevel,
		Service: "test-service",
	})
	assert.NoError(t, err)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	log.Debug(context.Background(), "Test debug message", map[string]interface{}{
		"debug_key": "debug_value",
	})

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, `"level":"DEBUG"`)
	assert.Contains(t, output, `"message":"Test debug message"`)
	assert.Contains(t, output, `"debug_key":"debug_value"`)
}

func TestLogger_Error(t *testing.T) {
	log, err := logger.New(logger.Config{
		Level:   logger.ErrorLevel,
		Service: "test-service",
	})
	assert.NoError(t, err)

	oldStdout := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	log.Error(context.Background(), "Test error message", map[string]interface{}{
		"error_key": "error_value",
	})

	w.Close()
	os.Stderr = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, `"level":"ERROR"`)
	assert.Contains(t, output, `"message":"Test error message"`)
	assert.Contains(t, output, `"error_key":"error_value"`)
}

func TestLogger_Warn(t *testing.T) {
	log, err := logger.New(logger.Config{
		Level:   logger.WarnLevel,
		Service: "test-service",
	})
	assert.NoError(t, err)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	log.Warn(context.Background(), "Test warn message", map[string]interface{}{
		"warn_key": "warn_value",
	})

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, `"level":"WARN"`)
	assert.Contains(t, output, `"message":"Test warn message"`)
	assert.Contains(t, output, `"warn_key":"warn_value"`)
}

func TestLogger_Fatal(t *testing.T) {
	log, err := logger.New(logger.Config{
		Level:   logger.FatalLevel,
		Service: "test-service",
	})
	assert.NoError(t, err)

	oldStdout := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Используем канал для захвата кода выхода
	exitChan := make(chan int, 1)
	defer close(exitChan)

	// Перехватываем os.Exit
	oldOsExit := osExit
	defer func() { osExit = oldOsExit }()
	osExit = func(code int) {
		exitChan <- code
		panic(fmt.Sprintf("exit %d", code)) // Имитируем os.Exit через панику
	}

	// Обрабатываем панику, чтобы тест не завершился
	defer func() {
		if r := recover(); r != nil {
			// Проверяем, что паника была вызвана нашим моком
			assert.Contains(t, r.(string), "exit 1")
		}
	}()

	log.Fatal(context.Background(), "Test fatal message", map[string]interface{}{
		"fatal_key": "fatal_value",
	})

	w.Close()
	os.Stderr = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, `"level":"FATAL"`)
	assert.Contains(t, output, `"message":"Test fatal message"`)
	assert.Contains(t, output, `"fatal_key":"fatal_value"`)

	// Проверяем код выхода
	select {
	case code := <-exitChan:
		assert.Equal(t, 1, code)
	default:
		t.Fatal("os.Exit не был вызван")
	}
}
