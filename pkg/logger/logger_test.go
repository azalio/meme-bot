package logger

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// NewTestLogger creates a logger for testing purposes
func NewTestLogger() *Logger {
	log, _ := New(Config{
		Level:   DebugLevel,
		Service: "test",
	})
	return log
}

func TestLogger(t *testing.T) {
	log := NewTestLogger()

	t.Run("Info", func(t *testing.T) {
		log.Info(context.Background(), "Test info message", map[string]interface{}{
			"key": "value",
		})
	})

	t.Run("Error", func(t *testing.T) {
		log.Error(context.Background(), "Test error message", map[string]interface{}{
			"error": "test error",
		})
	})

	t.Run("Debug", func(t *testing.T) {
		log.Debug(context.Background(), "Test debug message", nil)
	})

	t.Run("Fatal", func(t *testing.T) {
		// Redirect stdout to prevent exit
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		log.Fatal(context.Background(), "Test fatal message", nil)

		// Restore stdout
		os.Stdout = oldStdout
	})

	t.Run("Levels", func(t *testing.T) {
		assert.Equal(t, "DEBUG", DebugLevel.String())
		assert.Equal(t, "INFO", InfoLevel.String())
		assert.Equal(t, "WARN", WarnLevel.String())
		assert.Equal(t, "ERROR", ErrorLevel.String())
		assert.Equal(t, "FATAL", FatalLevel.String())
		assert.Equal(t, "UNKNOWN", Level(100).String())
	})

	t.Run("WithFields", func(t *testing.T) {
		loggerWithFields := log.With(map[string]interface{}{
			"field1": "value1",
		})
		assert.NotNil(t, loggerWithFields)
	})
}
