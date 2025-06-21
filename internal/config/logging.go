//
// logging.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/prometheus/common/promslog"
)

func SetupLogging(level, format *string) *slog.Logger {
	// default when no console
	logFormat := "logfmt"
	isatty := isatty.IsTerminal(os.Stderr.Fd())

	if format != nil && *format != "" {
		logFormat = *format
	} else if isatty {
		// default for console
		logFormat = "tint"
	}

	if logFormat == "tint" {
		logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
			AddSource:  true,
			Level:      parseLevel(level),
			NoColor:    !isatty,
			TimeFormat: time.TimeOnly,
		}))
		slog.SetDefault(logger)

		return logger
	}

	promslogConfig := &promslog.Config{
		Level:  promslog.NewLevel(),
		Format: promslog.NewFormat(),
		Style:  promslog.SlogStyle,
	}

	if level != nil {
		if err := promslogConfig.Level.Set(*level); err != nil {
			slog.Error("configure logging error", "err", err)
		}
	}

	if err := promslogConfig.Format.Set(logFormat); err != nil {
		slog.Error("configure logging error", "err", err)
	}

	logger := promslog.New(promslogConfig)
	slog.SetDefault(logger)

	return logger
}

func parseLevel(s *string) slog.Level {
	if s == nil {
		return slog.LevelInfo
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(*s)); err != nil {
		slog.Error("parse log level error", "level", *s, "err", err)
	}

	return level
}
