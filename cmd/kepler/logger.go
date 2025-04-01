/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func setupLogger(level, format string) *slog.Logger {
	logLevel := parseLogLevel(level)
	return slog.New(handlerForFormat(format, logLevel))
}

func handlerForFormat(format string, logLevel slog.Level) slog.Handler {
	switch format {
	case "json":
		return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     logLevel,
			AddSource: true,
		})

	case "text":
		return slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
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
