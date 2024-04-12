package config

import (
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-kit/log/term"
)

var GlobalLogger log.Logger

func ConfigureLog(logLevel, logFormat string) log.Logger {
	var logger log.Logger

	w := log.NewSyncWriter(os.Stdout)

	if logFormat == "json" {
		logger = term.NewLogger(w, log.NewJSONLogger, logColorFunc)
	} else {
		logger = term.NewLogger(w, log.NewLogfmtLogger, logColorFunc)
	}

	logger = level.NewFilter(logger, level.Allow(level.ParseDefault(logLevel, level.DebugValue())))
	logger = log.WithSuffix(logger, "caller", log.DefaultCaller)

	GlobalLogger = log.LoggerFunc(func(keyvals ...interface{}) error {
		if err := logger.Log(keyvals...); err != nil {
			panic(fmt.Errorf("%v: %w", keyvals, err))
		}

		return nil
	})

	return GlobalLogger
}

func logColorFunc(keyvals ...interface{}) term.FgBgColor {
	for i := 0; i < len(keyvals)-1; i += 2 {
		if keyvals[i] != "level" {
			continue
		}

		level, ok := keyvals[i+1].(level.Value)
		if !ok {
			continue
		}

		switch level.String() {
		case "debug":
			return term.FgBgColor{Fg: term.DarkGray}
		case "info":
			return term.FgBgColor{Fg: term.Gray}
		case "warn":
			return term.FgBgColor{Fg: term.Yellow}
		case "error":
			return term.FgBgColor{Fg: term.Red}
		default:
			return term.FgBgColor{}
		}
	}

	return term.FgBgColor{}
}
