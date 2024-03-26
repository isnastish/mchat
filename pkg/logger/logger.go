package logger

import (
	"fmt"
	"github.com/rs/zerolog"
	"os"
	"strings"
	"time"
)

type Logger struct {
	level string
	zerolog.Logger
}

func setLogLevel(logLevel string) error {
	var logLevelMap = map[string]zerolog.Level{
		"debug":    zerolog.DebugLevel,
		"info":     zerolog.InfoLevel,
		"warning":  zerolog.WarnLevel,
		"error":    zerolog.ErrorLevel,
		"fatal":    zerolog.FatalLevel,
		"panic":    zerolog.PanicLevel,
		"disabled": zerolog.Disabled,
		"trace":    zerolog.TraceLevel,
	}

	level, exists := logLevelMap[logLevel]
	if exists {
		zerolog.SetGlobalLevel(level)
		return nil
	}
	return fmt.Errorf("undefined log level: %s", logLevel)
}

func NewLogger(logLevel string) Logger {
	zerolog.TimeFieldFormat = time.RFC822

	logLevel = strings.ToLower(logLevel)
	if err := setLogLevel(logLevel); err != nil {
		fmt.Printf("failed to set global log level: %v", err.Error())
	} else {
		setLogLevel("debug")
	}

	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC822}
	output.FormatLevel = func(l interface{}) string {
		return strings.ToUpper(fmt.Sprintf("|%s|", l))
	}
	output.FormatMessage = func(msg interface{}) string {
		return fmt.Sprintf("Msg: %v", msg)
	}
	output.FormatFieldName = func(name interface{}) string {
		return fmt.Sprintf("%s: ", name)
	}

	logger := Logger{
		level:  logLevel,
		Logger: zerolog.New(output).With().Timestamp().Logger(),
	}

	return logger
}
