package logger_test

import (
	"context"
	"os"
	"testing"

	"github.com/azalio/meme-bot/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestLogger_Fatal(t *testing.T) {
	// Создаем логгер
	log, err := logger.New(logger.Config{
		Level:   logger.FatalLevel,
		Service: "test-service",
	})
	assert.NoError(t, err)

	// Перенаправляем stderr для захвата вывода
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	// Вызываем Fatal
	log.Fatal(context.Background(), "Test fatal message", map[string]interface{}{
		"fatal_key": "fatal_value",
	})

	// Читаем захваченный вывод
	w.Close()
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Проверяем, что вывод содержит ожидаемые данные
	assert.Contains(t, output, `"level":"FATAL"`)
	assert.Contains(t, output, `"message":"Test fatal message"`)
	assert.Contains(t, output, `"fatal_key":"fatal_value"`)
	assert.Contains(t, output, `"service":"test-service"`)
}
