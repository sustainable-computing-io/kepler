// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func New(level, format string) *slog.Logger {
	logLevel := parseLogLevel(level)
	return slog.New(handlerForFormat(format, logLevel))
}

func handlerForFormat(format string, logLevel slog.Level) slog.Handler {
	switch format {
	case "json":
		return slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: true,
		})

	case "text":
		return slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: true,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.SourceKey {
					if src, ok := a.Value.Any().(*slog.Source); ok {

						// Split the path into components
						parts := strings.Split(filepath.ToSlash(src.File), "/")

						// If we have enough components, take the last 3 -> 2 dirs + filename
						if len(parts) > 2 {
							src.File = filepath.Join(parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1])
						} else if len(parts) > 0 {
							// If not, take all the parts
							src.File = filepath.Join(parts...)
						}
					}
				}
				return a
			},
		})

	default:
		panic(fmt.Sprintf("invalid format: %s", format))
	}
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
