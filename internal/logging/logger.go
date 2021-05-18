package logging

import (
	"os"
	"strings"

	"go.uber.org/zap"
)

var (
	// DefaultLogger is the default logger inside the gnet server.
	DefaultLogger Logger
	zapLogger     *zap.Logger
)

func init() {
	switch strings.ToLower(os.Getenv("GNET_LOGGING_MODE")) {
	case "prod":
		zapLogger, _ = zap.NewProduction()
	default:
		// Other values except "Prod" create the development logger for gnet server.
		zapLogger, _ = zap.NewDevelopment()
	}
	DefaultLogger = zapLogger.Sugar()
}

// Cleanup does something windup for logger, like closing, flushing, etc.
func Cleanup() {
	_ = zapLogger.Sync()
}

// Logger is used for logging formatted messages.
type Logger interface {
	// Debugf logs messages at DEBUG level.
	Debugf(format string, args ...interface{})
	// Infof logs messages at INFO level.
	Infof(format string, args ...interface{})
	// Warnf logs messages at WARN level.
	Warnf(format string, args ...interface{})
	// Errorf logs messages at ERROR level.
	Errorf(format string, args ...interface{})
	// Fatalf logs messages at FATAL level.
	Fatalf(format string, args ...interface{})
}
