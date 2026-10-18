package main

import (
	"chatbit/internal/adapter/vapid"
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"chatbit/internal/port"
	"log/slog"
	"os"
)

const MaxMessageTTL = 60 * 60 * 24 * 30

// Injected at build time via -ldflags (see Taskfile.yml)
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	slog.SetLogLoggerLevel(slog.LevelDebug)

	app := core.NewApplication()

	// log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	logger.Info("ChatBit start")

	// Store every message, capped to MaxMessageTTL
	_, err := app.PS.Message.Subscribe(func(obj domain.Message) {
		if obj.TTL > MaxMessageTTL || obj.TTL <= 0 {
			obj.TTL = MaxMessageTTL
		}

		err := app.DB.Message.AddWithTTL(obj, obj.TTL)
		if err != nil {
			logger.Error("not possible to store message", slog.String("room", obj.RoomID.String()), slog.String("error", err.Error()))
		}
	})
	if err != nil {
		logger.Error("not possible to subscribe to messages", slog.String("error", err.Error()))
		os.Exit(1)
	}

	vapid.LoadKeys()

	serv := port.NewServer(app, port.BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	})
	err = serv.Start()
	if err != nil {
		logger.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}

}
