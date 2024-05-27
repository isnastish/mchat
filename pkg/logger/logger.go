package logger

import (
	"fmt"
	"github.com/rs/zerolog"
	"os"
	"strings"
	"time"
)

type Logger struct {
	level      string
	zeroLogger zerolog.Logger

	disabled bool
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

	output.FormatFieldName = func(name interface{}) string {
		return fmt.Sprintf("%s: ", name)
	}

	logger := Logger{
		level:      logLevel,
		zeroLogger: zerolog.New(output).With().Timestamp().Logger(),
	}

	return logger
}

func (l *Logger) log(level zerolog.Level, format string, args ...any) {
	if !l.disabled {
		switch level {
		case zerolog.InfoLevel:
			l.zeroLogger.Info().Msgf(format, args...)
		case zerolog.DebugLevel:
			l.zeroLogger.Info().Msgf(format, args...)
		case zerolog.WarnLevel:
			l.zeroLogger.Info().Msgf(format, args...)
		case zerolog.ErrorLevel:
			l.zeroLogger.Error().Msgf(format, args...)
		case zerolog.FatalLevel:
			l.zeroLogger.Fatal().Msgf(format, args...)
		case zerolog.PanicLevel:
			l.zeroLogger.Panic().Msgf(format, args...)
		case zerolog.TraceLevel:
			l.zeroLogger.Trace().Msgf(format, args...)
		}
	}
}

func (l *Logger) Info(format string, args ...any) {
	l.log(zerolog.InfoLevel, format, args...)
}

func (l *Logger) Debug(format string, args ...any) {
	l.log(zerolog.DebugLevel, format, args...)
}

func (l *Logger) Warn(format string, args ...any) {
	l.log(zerolog.WarnLevel, format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.log(zerolog.ErrorLevel, format, args...)
}

func (l *Logger) Fatal(format string, args ...any) {
	l.log(zerolog.PanicLevel, format, args...)
}

func (l *Logger) Trace(format string, args ...any) {
	l.log(zerolog.TraceLevel, format, args...)
}
