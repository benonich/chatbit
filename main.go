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

func main() {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	slog.SetLogLoggerLevel(slog.LevelDebug)

	app := core.NewApplication()

	// log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	logger.Info("ChatBit start")

	app.PS.Message.Subscribe(func(obj domain.Message) {
		if obj.TTL > MaxMessageTTL || obj.TTL <= 0 {
			obj.TTL = MaxMessageTTL
		}

		app.DB.Message.AddWithTTL(obj, obj.TTL)
	})

	vapid.GenerateVapID()

	serv := port.NewServer(app)
	serv.Start()

}
