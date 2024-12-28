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
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		log.Info(context.Background(), "Test info message", map[string]interface{}{
			"key": "value",
		})

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		assert.Contains(t, output, `"level":"INFO"`)
		assert.Contains(t, output, `"message":"Test info message"`)
		assert.Contains(t, output, `"key":"value"`)
	})

	t.Run("Error", func(t *testing.T) {
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		log.Error(context.Background(), "Test error message", map[string]interface{}{
			"error": "test error",
		})

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		assert.Contains(t, output, `"level":"ERROR"`)
		assert.Contains(t, output, `"message":"Test error message"`)
		assert.Contains(t, output, `"error":"test error"`)
	})

	t.Run("Debug", func(t *testing.T) {
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		log.Debug(context.Background(), "Test debug message", nil)

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		assert.Contains(t, output, `"level":"DEBUG"`)
		assert.Contains(t, output, `"message":"Test debug message"`)
	})

	t.Run("Fatal", func(t *testing.T) {
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Use a fake os.Exit to prevent actual program exit
		oldExit := osExit
		defer func() { osExit = oldExit }()
		var exitCode int
		osExit = func(code int) { exitCode = code }

		log.Fatal(context.Background(), "Test fatal message", nil)

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		assert.Contains(t, output, `"level":"FATAL"`)
		assert.Contains(t, output, `"message":"Test fatal message"`)
		assert.Equal(t, 1, exitCode)
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

		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		loggerWithFields.Info(context.Background(), "Test with fields", nil)

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		assert.Contains(t, output, `"field1":"value1"`)
	})
}

// Mock os.Exit for testing
var osExit = os.Exit
