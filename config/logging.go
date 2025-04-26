//
// logging.go
// Copyright (C) 2025 Karol Będkowski <Karol Będkowski@kkomp>
//
// Distributed under terms of the GPLv3 license.
//

package config

import (
	"log/slog"

	"github.com/prometheus/common/promslog"
)

func SetupLogging(logLevel, logFormat *string) *slog.Logger {
	promslogConfig := &promslog.Config{
		Level:  promslog.NewLevel(),
		Format: promslog.NewFormat(),
		Style:  promslog.SlogStyle,
	}

	if logLevel != nil {
		if err := promslogConfig.Level.Set(*logLevel); err != nil {
			slog.Error("configure logging error", "err", err)
		}
	}

	if logFormat != nil {
		if err := promslogConfig.Format.Set(*logFormat); err != nil {
			slog.Error("configure logging error", "err", err)
		}
	}

	logger := promslog.New(promslogConfig)
	slog.SetDefault(logger)

	return logger
}
