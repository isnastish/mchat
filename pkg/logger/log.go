package log

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
)

type logger struct {
	level   string
	zerolog zerolog.Logger
}

func setupLogger(level string) *logger {
	zerolog.TimeFieldFormat = time.RFC850
	zerolog.TimestampFieldName = "timestamp"

	level = strings.ToLower(level)
	if err := setLogLevel(level); err != nil {
		fmt.Printf("failed to set global log level: %v", err)
	} else {
		setLogLevel("debug")
	}

	logger := &logger{
		level: level,
	}

	if runtime.GOOS == "windows" {
		// zerolog doesn't support colorable logging on windows, thus we need to use colorable package
		output := zerolog.ConsoleWriter{Out: colorable.NewColorableStderr()}
		logger.zerolog = zerolog.New(output).With().Timestamp().Logger()
	} else {
		logger.zerolog = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	return logger
}

func setLogLevel(level string) error {
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

	zerologLevel, exists := logLevelMap[level]
	if exists {
		zerolog.SetGlobalLevel(zerologLevel)
		globalLevel := zerolog.GlobalLevel()
		_ = globalLevel
		return nil
	}

	return fmt.Errorf("Undefined log level: %s", level)
}

func (l *logger) log(level zerolog.Level, format string, args ...any) {
	switch level {
	case zerolog.InfoLevel:
		l.zerolog.Info().Msgf(format, args...)
	case zerolog.DebugLevel:
		l.zerolog.Debug().Msgf(format, args...)
	case zerolog.WarnLevel:
		l.zerolog.Warn().Msgf(format, args...)
	case zerolog.ErrorLevel:
		l.zerolog.Error().Msgf(format, args...)
	case zerolog.FatalLevel:
		l.zerolog.Fatal().Msgf(format, args...)
	case zerolog.PanicLevel:
		l.zerolog.Panic().Msgf(format, args...)
	case zerolog.TraceLevel:
		l.zerolog.Trace().Msgf(format, args...)
	}
}

func (l *logger) Info(format string, args ...any) {
	l.log(zerolog.InfoLevel, format, args...)
}

func (l *logger) Debug(format string, args ...any) {
	l.log(zerolog.DebugLevel, format, args...)
}

func (l *logger) Warn(format string, args ...any) {
	l.log(zerolog.WarnLevel, format, args...)
}

func (l *logger) Error(format string, args ...any) {
	l.log(zerolog.ErrorLevel, format, args...)
}

func (l *logger) Fatal(format string, args ...any) {
	l.log(zerolog.PanicLevel, format, args...)
}

func (l *logger) Trace(format string, args ...any) {
	l.log(zerolog.TraceLevel, format, args...)
}

func (l *logger) Panic(format string, args ...any) {
	l.log(zerolog.PanicLevel, format, args...)
}

var Logger = setupLogger("debug")
