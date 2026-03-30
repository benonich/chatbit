package main

import (
	"chatbit/internal/core"
	"chatbit/internal/port"
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	slog.SetLogLoggerLevel(slog.LevelDebug)

	app := core.NewApplication()

	// log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	logger.Info("ChatBit start")

	serv := port.NewServer(app)
	serv.Start()

}
