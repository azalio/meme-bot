package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Logger представляет собой структуру для логирования
type Logger struct {
	infoLog  *log.Logger
	errorLog *log.Logger
	debugLog *log.Logger
	mu       sync.Mutex // мьютекс для безопасного доступа к логгеру
}

// New создает новый экземпляр логгера
func New() *Logger {
	return &Logger{
		infoLog:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
		errorLog: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime),
		debugLog: log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime),
	}
}

// getCallerInfo возвращает имя файла и номер строки вызывающего кода
func getCallerInfo() string {
	_, file, line, ok := runtime.Caller(2) // 2 означает, что мы пропускаем два уровня стека вызовов
	if !ok {
		return "unknown:0"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

// Info логирует информационное сообщение
func (l *Logger) Info(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infoLog.Printf("%s: %s", getCallerInfo(), fmt.Sprintf(format, v...))
}

// Error логирует сообщение об ошибке
func (l *Logger) Error(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errorLog.Printf("%s: %s", getCallerInfo(), fmt.Sprintf(format, v...))
}

// Debug логирует отладочное сообщение если включен debug режим
func (l *Logger) Debug(format string, v ...interface{}) {
	if !l.isDebugEnabled() {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugLog.Printf("%s: %s", getCallerInfo(), fmt.Sprintf(format, v...))
}

// Fatal логирует сообщение об ошибке и завершает программу с кодом выхода 1
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errorLog.Printf("%s: %s", getCallerInfo(), fmt.Sprintf(format, v...))
	os.Exit(1)
}

// isDebugEnabled проверяет, включен ли режим отладки
func (l *Logger) isDebugEnabled() bool {
	return os.Getenv("MEME_DEBUG") == "1"
}
