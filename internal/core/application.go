package core

import (
	"chatbit/internal/adapter/database"
	"chatbit/internal/adapter/pubsub"
	"chatbit/internal/controller/store"
	"chatbit/internal/domain"
)

type Application struct {
	DB DB
	PS PS
}

type DB struct {
	Room    store.DBInternal[domain.Room]
	Message store.DBMessageInternal[domain.Message]
	Push    store.DBInternal[domain.Push]
}

type PS struct {
	Message *pubsub.Adapter[domain.Message]
}

func NewApplication() *Application {
	db := database.NewDatabase()
	db.Open()

	return &Application{
		DB: DB{
			Room:    store.NewAdapterInternal[domain.Room](database.NewAdapter[domain.Room](db)),
			Message: store.NewMessageAdapterInternal[domain.Message](database.NewAdapterTs[domain.Message](db)),
			Push:    store.NewAdapterInternal[domain.Push](database.NewAdapter[domain.Push](db)),
		},
		PS: PS{
			Message: pubsub.NewAdapter[domain.Message](),
		},
	}
}
