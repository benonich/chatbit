package port

import (
	"chatbit/internal/core"
	"errors"
	"log"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 120 * time.Second
)

type Server struct {
	app     *core.Application
	Server  *http.Server
	servMux *http.ServeMux
}

func NewServer(app *core.Application) *Server {
	return &Server{app: app}
}

func (s *Server) Start() error {

	//vapid.GenerateVapID()

	// WebSocket connection
	s.servMux = http.NewServeMux()

	s.StartFileServer()

	s.StartWebSocketServer()

	// Push notification
	//servMux.HandleFunc("/vapidPublicKey", vapid.VapIDPubKeyFunc)
	//servMux.HandleFunc("/subscribe", vapid.SubscribeFunc)
	//servMux.HandleFunc("/send", vapid.SendFunc)

	servObj := &http.Server{
		Addr:              ":" + "8081",
		Handler:           s.servMux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	if err := servObj.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}

	return nil
}
