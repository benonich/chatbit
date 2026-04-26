package main

import (
	"chatbit/internal/adapter/vapid"
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"chatbit/internal/port"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/SherClockHolmes/webpush-go"
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

	vapid.GenerateVapID()

	serv := port.NewServer(app)
	serv.Start()

}

func TestPush() {
	sub := &webpush.Subscription{
		Endpoint: "https://web.push.apple.com/QPmd3jYQ_PPW7fimyS8m8CDfpIs18i65X9AD0zdnZkaENJydDj9lVvJxxR0F_38_3wSWW1mMIhWBcHMUmhJGk1uF36zoPCXAhsmZg-J1ykeNKMdOIedM2MPAU-8pBSjndiDxyd1J5c3tsBp1U3dzyGRIoLyXjxdCOqyBJCj36gQ",
		Keys: webpush.Keys{
			P256dh: "BKKLwJH_ZqXDI5MsgRvt5iLiXgr6oFN6H2k0ue-CG7bSHfQlMCfDws_MqLUTJYFArg6FABpJ3jTctWICvUJT8uY",
			Auth:   "1Cx-jdTImzK1WEOpSGuyNg",
		},
	}

	body, _ := json.Marshal(map[string]string{
		"title": "ChatBit",
		"body":  "New Message 🔒",
		"url":   "/",
	})

	option := webpush.Options{
		Subscriber:      "alessandro@benoni.ch",
		TTL:             30,
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
	}

	resp, err := webpush.SendNotification(body, sub, &option)
	if err != nil {
		slog.Error("not possible to send push notification", slog.String("error", err.Error()))
	}
	defer resp.Body.Close()
}
