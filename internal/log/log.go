// Package log provides a unified logging entry for the backend.
// It wraps log/slog with lumberjack file rotation and exposes leveled
// helpers (Info/Warn/Error/Panic/Fatal and their formatted variants)
// so that other packages use a single consistent logging system.
package log

import (
	"fmt"
	"github.com/listen808/citebase/internal/utils/envname"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger    *slog.Logger
	logWriter io.Writer
)

// parseLogLevel parses a log level string (debug, info, warn, error).
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func init() {
	// Default: stderr only. Call InitFileLogger() at startup to enable file rotation + console output.
	// Log level can be configured via AIWIKI_LOG_LEVEL env var (debug/info/warn/error).
	// P1-5：生产模式输出 JSON——TextHandler 的自由文本进 ELK/Loki 需要人肉写解析规则。
	level := parseLogLevel(envname.Getenv("CITEBASE_LOG_LEVEL", "AIWIKI_LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}
	if isJSONMode() {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, opts))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	logWriter = os.Stderr
}

// isJSONMode 生产环境默认 JSON 日志；AIWIKI_LOG_FORMAT=text 可强制回退人类可读格式。
func isJSONMode() bool {
	if f := envname.Getenv("CITEBASE_LOG_FORMAT", "AIWIKI_LOG_FORMAT"); f != "" {
		return f == "json"
	}
	return os.Getenv("GIN_MODE") == "release"
}

// InitFileLogger initializes structured logging with file rotation.
// Logs are written to both stdout and a rotating log file at data/logs/citebase.log.
// Rotation: 100MB per file, 7 backups, 30 days retention, gzip compression.
func InitFileLogger() {
	logDir := filepath.Join("data", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Fall back to stderr if log directory cannot be created.
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: parseLogLevel(envname.Getenv("CITEBASE_LOG_LEVEL", "AIWIKI_LOG_LEVEL")),
		}))
		logWriter = os.Stderr
		Errorf("Failed to create log directory %s: %v, falling back to stderr", logDir, err)
		return
	}

	fileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "citebase.log"),
		MaxSize:    100, // megabytes
		MaxBackups: 7,
		MaxAge:     30, // days
		Compress:   true,
	}

	// Write to both console and file.
	logWriter = io.MultiWriter(os.Stdout, fileWriter)

	// P1-5：与 init 保持一致——生产 JSON，开发可读
	opts := &slog.HandlerOptions{Level: parseLogLevel(envname.Getenv("CITEBASE_LOG_LEVEL", "AIWIKI_LOG_LEVEL"))}
	if isJSONMode() {
		logger = slog.New(slog.NewJSONHandler(logWriter, opts))
	} else {
		logger = slog.New(slog.NewTextHandler(logWriter, opts))
	}

	Infof("[Logger] Initialized: file=%s, maxSize=%dMB, maxBackups=%d, maxAge=%dd, compress=%v",
		fileWriter.Filename, fileWriter.MaxSize, fileWriter.MaxBackups, fileWriter.MaxAge, fileWriter.Compress)
}

// Writer returns the current log writer for use by other subsystems (e.g. GORM).
func Writer() io.Writer {
	if logWriter == nil {
		return os.Stderr
	}
	return logWriter
}

// StdLogger returns a standard *log.Logger backed by the unified writer,
// suitable for GORM's logger or other libraries that require *log.Logger.
func StdLogger(prefix string, flag int) *stdLogger {
	return &stdLogger{
		prefix: prefix,
		flag:   flag,
	}
}

// stdLogger wraps the unified writer to satisfy io.Writer for standard log compatibility.
type stdLogger struct {
	prefix string
	flag   int
}

func (l *stdLogger) Write(p []byte) (n int, err error) {
	msg := string(p)
	// Remove trailing newline to avoid double newlines in slog output.
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	logger.Info(l.prefix + msg)
	return len(p), nil
}

func Info(msg string, args ...any) {
	logger.Info(msg, args...)
}

func Infof(format string, args ...any) {
	logger.Info(fmt.Sprintf(format, args...))
}

func Warn(msg string, args ...any) {
	logger.Warn(msg, args...)
}

func Warnf(format string, args ...any) {
	logger.Warn(fmt.Sprintf(format, args...))
}

func Error(msg string, args ...any) {
	logger.Error(msg, args...)
}

func Errorf(format string, args ...any) {
	logger.Error(fmt.Sprintf(format, args...))
}

func Panic(msg string, args ...any) {
	logger.Error(msg, args...)
	panic(msg)
}

func Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logger.Error(msg)
	panic(msg)
}

func Fatal(msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}

func Fatalf(format string, args ...any) {
	logger.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}
