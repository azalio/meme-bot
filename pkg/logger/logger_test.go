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
