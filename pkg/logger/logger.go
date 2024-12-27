package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Level представляет уровень логирования
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String возвращает строковое представление уровня логирования
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger представляет собой структуру для логирования
type Logger struct {
	mu        sync.Mutex
	level     Level
	service   string
	env       string
	hostname  string
	gitCommit string
}

// LogEntry представляет структуру JSON-записи лога
type LogEntry struct {
	Level      string                 `json:"level"`
	Timestamp  string                 `json:"timestamp"`
	Message    string                 `json:"message"`
	Caller     string                 `json:"caller"`
	Service    string                 `json:"service"`
	Env        string                 `json:"env,omitempty"`
	Hostname   string                 `json:"hostname,omitempty"`
	GitCommit  string                 `json:"git_commit,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	SpanID     string                 `json:"span_id,omitempty"`
	Additional map[string]interface{} `json:"additional,omitempty"`
}

// Config представляет конфигурацию логгера
type Config struct {
	Level     Level
	Service   string
	Env       string
	GitCommit string
}

// New создает новый экземпляр логгера
func New(cfg Config) (*Logger, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return &Logger{
		level:     cfg.Level,
		service:   cfg.Service,
		env:       cfg.Env,
		hostname:  hostname,
		gitCommit: cfg.GitCommit,
	}, nil
}

// getCallerInfo возвращает имя файла и номер строки вызывающего кода
func getCallerInfo() string {
	_, file, line, ok := runtime.Caller(3) // 3 для пропуска дополнительного уровня стека
	if !ok {
		return "unknown:0"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

// writeLog записывает лог в JSON формате
func (l *Logger) writeLog(ctx context.Context, level Level, output *os.File, msg string, additional map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Level:      level.String(),
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Message:    msg,
		Caller:     getCallerInfo(),
		Service:    l.service,
		Env:        l.env,
		Hostname:   l.hostname,
		GitCommit:  l.gitCommit,
		Additional: additional,
	}

	// Добавляем информацию о трейсинге, если она есть в контексте
	if span := trace.SpanFromContext(ctx); span != nil {
		spanCtx := span.SpanContext()
		if spanCtx.HasTraceID() {
			entry.TraceID = spanCtx.TraceID().String()
		}
		if spanCtx.HasSpanID() {
			entry.SpanID = spanCtx.SpanID().String()
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(entry); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding log entry: %v\n", err)
	}
}

// With добавляет дополнительные поля к логу
func (l *Logger) With(fields map[string]interface{}) *Logger {
	newLogger := *l
	return &newLogger
}

// Debug логирует отладочное сообщение
func (l *Logger) Debug(ctx context.Context, msg string, fields map[string]interface{}) {
	l.writeLog(ctx, DebugLevel, os.Stdout, msg, fields)
}

// Info логирует информационное сообщение
func (l *Logger) Info(ctx context.Context, msg string, fields map[string]interface{}) {
	l.writeLog(ctx, InfoLevel, os.Stdout, msg, fields)
}

// Warn логирует предупреждающее сообщение
func (l *Logger) Warn(ctx context.Context, msg string, fields map[string]interface{}) {
	l.writeLog(ctx, WarnLevel, os.Stdout, msg, fields)
}

// Error логирует сообщение об ошибке
func (l *Logger) Error(ctx context.Context, msg string, fields map[string]interface{}) {
	l.writeLog(ctx, ErrorLevel, os.Stderr, msg, fields)
}

// Fatal логирует сообщение об ошибке и завершает программу
func (l *Logger) Fatal(ctx context.Context, msg string, fields map[string]interface{}) {
	l.writeLog(ctx, FatalLevel, os.Stderr, msg, fields)
	os.Exit(1)
}

// SetLevel устанавливает уровень логирования
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel возвращает текущий уровень логирования
func (l *Logger) GetLevel() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}
