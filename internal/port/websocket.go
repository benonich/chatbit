package port

import (
	"chatbit/internal/adapter/pubsub"
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type wsHandler struct {
	psWsMsgAdapter *pubsub.AdapterTopic[WsOutbound]
	app            *core.Application
}

func (s *Server) StartWebSocketServer() {
	servWS := &wsHandler{
		psWsMsgAdapter: pubsub.NewAdapterTopic[WsOutbound](),
		app:            s.app,
	}

	s.servMux.Handle("/ws", servWS)
}

// serveWs handles websocket requests from the peer.
func (h *wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("upgrade error", slog.String("error", err.Error()))
		return
	}

	clientUID := r.URL.Query().Get("client_uid")
	if clientUID == "" {
		http.Error(w, "missing client_uid", http.StatusBadRequest)
		return
	}

	client := &WsClient{
		UID:            clientUID,
		conn:           conn,
		send:           make(chan WsOutbound, 256),
		psWsMsgAdapter: h.psWsMsgAdapter,
		app:            h.app,
		fileTransfer:   make(map[string]*domain.Transfer),
		fileTransferMu: sync.Mutex{},
	}

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()

	_, _ = h.psWsMsgAdapter.SubscribeWithID(client.UID, client.UID, func(obj WsOutbound) {
		client.send <- obj
	})
}
