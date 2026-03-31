package main

import (
	"chatbit/internal/adapter/vapid"
	"chatbit/internal/core"
	"chatbit/internal/domain"
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

	app.PS.Message.Subscribe(func(obj domain.Message) {
		app.DB.Message.Add(obj)
	})

	privateKey, publicKey, _ := vapid.GenerateVAPIDKeys()

	serv := port.NewServer(app)
	serv.Start()

}
