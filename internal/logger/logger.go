package logger

import (
	"log"
	"os"
)

// Color codes for terminal output
const (
	ColorRed    = "\033[0;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[0;33m"
	ColorBlue   = "\033[0;34m"
	ColorReset  = "\033[0m"
)

// Logger defines custom logging levels
type Logger struct {
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
	WarnLogger  *log.Logger
}

// New initializes a new logger with color output
func New() *Logger {
	return &Logger{
		InfoLogger:  log.New(os.Stdout, ColorGreen+"[INFO] "+ColorReset, log.Ldate|log.Ltime),
		ErrorLogger: log.New(os.Stderr, ColorRed+"[ERROR] "+ColorReset, log.Ldate|log.Ltime),
		WarnLogger:  log.New(os.Stdout, ColorYellow+"[WARN] "+ColorReset, log.Ldate|log.Ltime),
	}
}

// Info logs an informational message
func (l *Logger) Info(format string, v ...interface{}) {
	l.InfoLogger.Printf(format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.ErrorLogger.Printf(format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.WarnLogger.Printf(format, v...)
}
