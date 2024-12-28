package logger

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// NewTestLogger создает логгер для тестирования
func NewTestLogger() *Logger {
	log, _ := New(Config{
		Level:   DebugLevel,
		Service: "test",
	})
	return log
}

func TestLogger_Info(t *testing.T) {
	// Создаем логгер для тестов
	log := &Logger{
		level:   InfoLevel,
		service: "test-service",
	}

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
	assert.Equal(t, "DEBUG", DebugLevel.String())
	assert.Equal(t, "INFO", InfoLevel.String())
	assert.Equal(t, "WARN", WarnLevel.String())
	assert.Equal(t, "ERROR", ErrorLevel.String())
	assert.Equal(t, "FATAL", FatalLevel.String())
	assert.Equal(t, "UNKNOWN", Level(100).String())
}
