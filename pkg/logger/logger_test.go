package logger_test

import (
	"context"
	"os"
	"testing"

	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/stretchr/testify/assert"
)

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

// func TestLogger_Fatal(t *testing.T) {
// 	// Создаем логгер
// 	log, err := logger.New(logger.Config{
// 		Level:   logger.FatalLevel,
// 		Service: "test-service",
// 	})
// 	assert.NoError(t, err)

// 	// Перенаправляем stderr для захвата вывода
// 	oldStderr := os.Stderr
// 	r, w, _ := os.Pipe()
// 	os.Stderr = w
// 	defer func() { os.Stderr = oldStderr }()

// 	// Вызываем Fatal
// 	log.Fatal(context.Background(), "Test fatal message", map[string]interface{}{
// 		"fatal_key": "fatal_value",
// 	})

// 	// Читаем захваченный вывод
// 	w.Close()
// 	buf := make([]byte, 1024)
// 	n, _ := r.Read(buf)
// 	output := string(buf[:n])

// 	// Проверяем, что вывод содержит ожидаемые данные
// 	assert.Contains(t, output, `"level":"FATAL"`)
// 	assert.Contains(t, output, `"message":"Test fatal message"`)
// 	assert.Contains(t, output, `"fatal_key":"fatal_value"`)
// 	assert.Contains(t, output, `"service":"test-service"`)
// }
