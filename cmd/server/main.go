package main

import (
	"log/slog"
	"os"

	"github.com/tepzxl/contentflow/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("run server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
