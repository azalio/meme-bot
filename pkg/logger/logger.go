// Package logger предоставляет функционал для логирования
package logger

import (
    "log"
    "os"
    "sync"
)

// Logger представляет собой структуру для логирования
// Он использует стандартный пакет log, но может быть легко заменен
// на более продвинутые решения как zap или logrus
type Logger struct {
    infoLog  *log.Logger
    errorLog *log.Logger
    debugLog *log.Logger
    mu       sync.Mutex // мьютекс для безопасного доступа к логгеру
}

// New создает новый экземпляр логгера
func New() *Logger {
    return &Logger{
        // Создаем логгер для информационных сообщений
        // Они будут выводиться в stdout (консоль)
        infoLog: log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
        
        // Создаем логгер для ошибок
        // Они будут выводиться в stderr
        errorLog: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
        
        // Создаем логгер для отладочных сообщений
        // Они будут выводиться в stdout только если включен debug режим
        debugLog: log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
    }
}

// Info логирует информационное сообщение
func (l *Logger) Info(format string, v ...interface{}) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.infoLog.Printf(format, v...)
}

// Error логирует сообщение об ошибке
func (l *Logger) Error(format string, v ...interface{}) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.errorLog.Printf(format, v...)
}

// isDebugEnabled проверяет, включен ли режим отладки
func (l *Logger) isDebugEnabled() bool {
    return os.Getenv("MEME_DEBUG") == "1"
}

// Debug логирует отладочное сообщение если включен debug режим
func (l *Logger) Debug(format string, v ...interface{}) {
    if !l.isDebugEnabled() {
        return
    }
    l.mu.Lock()
    defer l.mu.Unlock()
    l.debugLog.Printf(format, v...)
}
