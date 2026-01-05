package secs4go

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger 日志接口
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// ============================================================
// 默认Logger实现
// ============================================================

type defaultLogger struct {
	logger *log.Logger
	level  LogLevel
}

// NewDefaultLogger 创建默认logger (输出到stdout)
func NewDefaultLogger() Logger {
	return &defaultLogger{
		logger: log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds),
		level:  LogLevelInfo,
	}
}

// NewLoggerWithLevel 创建指定级别的logger
func NewLoggerWithLevel(level LogLevel) Logger {
	return &defaultLogger{
		logger: log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds),
		level:  level,
	}
}

func (l *defaultLogger) Debug(format string, args ...interface{}) {
	if l.level <= LogLevelDebug {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

func (l *defaultLogger) Info(format string, args ...interface{}) {
	if l.level <= LogLevelInfo {
		l.logger.Printf("[INFO] "+format, args...)
	}
}

func (l *defaultLogger) Warn(format string, args ...interface{}) {
	if l.level <= LogLevelWarn {
		l.logger.Printf("[WARN] "+format, args...)
	}
}

func (l *defaultLogger) Error(format string, args ...interface{}) {
	if l.level <= LogLevelError {
		l.logger.Printf("[ERROR] "+format, args...)
	}
}

// ============================================================
// 文件输出Logger实现
// ============================================================

type fileLogger struct {
	logger     *log.Logger
	level      LogLevel
	file       *os.File
	currentHour int
	mu         sync.Mutex
	logDir     string
}

// NewFileLogger 创建设备名称指定的文件logger
func NewFileLogger(deviceName string) Logger {
	return NewFileLoggerWithLevel(deviceName, LogLevelInfo)
}

// NewFileLoggerWithLevel 创建设备名称指定的文件logger（带级别）
func NewFileLoggerWithLevel(deviceName string, level LogLevel) Logger {
	// 清理设备名称中的非法字符
	cleanDeviceName := sanitizeDeviceName(deviceName)

	// 创建logs目录结构
	logDir := filepath.Join("logs", cleanDeviceName)
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		// 如果创建目录失败，回退到默认logger
		return NewLoggerWithLevel(level)
	}

	// 创建当前时间的日志文件
	currentTime := time.Now()
	logFileName := fmt.Sprintf("%s.log", currentTime.Format("20060102_15"))
	logFilePath := filepath.Join(logDir, logFileName)

	// 打开日志文件（追加模式）
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// 如果打开文件失败，回退到默认logger
		return NewLoggerWithLevel(level)
	}

	// 创建多writer：stdout + 文件
	multiWriter := io.MultiWriter(os.Stdout, file)

	logger := log.New(multiWriter, "", log.LstdFlags|log.Lmicroseconds)

	return &fileLogger{
		logger:       logger,
		level:        level,
		file:         file,
		currentHour:  currentTime.Hour(),
		logDir:       logDir,
	}
}

// sanitizeDeviceName 清理设备名称中的非法字符
func sanitizeDeviceName(deviceName string) string {
	invalidChars := `\/:*?"<>|`
	cleanName := deviceName
	for _, char := range invalidChars {
		cleanName = strings.ReplaceAll(cleanName, string(char), "_")
	}
	cleanName = strings.TrimSpace(cleanName)
	if cleanName == "" || cleanName == "." {
		cleanName = "Device"
	}
	return cleanName
}

func (l *fileLogger) Debug(format string, args ...interface{}) {
	if l.level <= LogLevelDebug {
		l.checkAndSwitchFile()
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

func (l *fileLogger) Info(format string, args ...interface{}) {
	if l.level <= LogLevelInfo {
		l.checkAndSwitchFile()
		l.logger.Printf("[INFO] "+format, args...)
	}
}

func (l *fileLogger) Warn(format string, args ...interface{}) {
	if l.level <= LogLevelWarn {
		l.checkAndSwitchFile()
		l.logger.Printf("[WARN] "+format, args...)
	}
}

func (l *fileLogger) Error(format string, args ...interface{}) {
	if l.level <= LogLevelError {
		l.checkAndSwitchFile()
		l.logger.Printf("[ERROR] "+format, args...)
	}
}

// checkAndSwitchFile 检查是否需要切换日志文件（按小时）
func (l *fileLogger) checkAndSwitchFile() {
	l.mu.Lock()
	defer l.mu.Unlock()

	currentHour := time.Now().Hour()
	if currentHour == l.currentHour {
		return
	}

	// 切换日志文件
	if l.file != nil {
		l.file.Close()
	}

	currentTime := time.Now()
	logFileName := fmt.Sprintf("%s.log", currentTime.Format("20060102_15"))
	logFilePath := filepath.Join(l.logDir, logFileName)

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// 如果打开文件失败，回退到仅输出到 stdout
		l.logger = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
		l.file = nil
		l.currentHour = currentHour
		return
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	l.logger = log.New(multiWriter, "", log.LstdFlags|log.Lmicroseconds)
	l.file = file
	l.currentHour = currentHour
}

// Close 关闭文件logger
func (l *fileLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// ============================================================
// 静默Logger(用于测试)
// ============================================================

type silentLogger struct{}

// NewSilentLogger 创建静默logger
func NewSilentLogger() Logger {
	return &silentLogger{}
}

func (l *silentLogger) Debug(format string, args ...interface{}) {}
func (l *silentLogger) Info(format string, args ...interface{})  {}
func (l *silentLogger) Warn(format string, args ...interface{})  {}
func (l *silentLogger) Error(format string, args ...interface{}) {}

// ============================================================
// 自定义Logger适配器
// ============================================================

// LoggerFunc 函数式logger
type LoggerFunc func(level string, format string, args ...interface{})

type funcLogger struct {
	fn LoggerFunc
}

// NewFuncLogger 创建函数式logger
func NewFuncLogger(fn LoggerFunc) Logger {
	return &funcLogger{fn: fn}
}

func (l *funcLogger) Debug(format string, args ...interface{}) {
	l.fn("DEBUG", format, args...)
}

func (l *funcLogger) Info(format string, args ...interface{}) {
	l.fn("INFO", format, args...)
}

func (l *funcLogger) Warn(format string, args ...interface{}) {
	l.fn("WARN", format, args...)
}

func (l *funcLogger) Error(format string, args ...interface{}) {
	l.fn("ERROR", format, args...)
}
