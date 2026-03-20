package sharedcfg

import (
	"fmt"
	"strings"

	"github.com/jianwushu/Secs4go/secs4go"
)

func NormalizeAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", fmt.Errorf("addr is required")
	}
	return address, nil
}

func NormalizeItemAEncoding(encoding, defaultEncoding string) string {
	encoding = strings.TrimSpace(encoding)
	if encoding == "" {
		return defaultEncoding
	}
	return encoding
}

func NormalizeLogLevel(level string) (string, error) {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "debug", "info", "warn", "error":
		return level, nil
	default:
		return "", fmt.Errorf("log-level must be one of: debug, info, warn, error")
	}
}

func ParseLogLevel(level string) secs4go.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return secs4go.LogLevelDebug
	case "warn":
		return secs4go.LogLevelWarn
	case "error":
		return secs4go.LogLevelError
	default:
		return secs4go.LogLevelInfo
	}
}
