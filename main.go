package main

import (
	"bytes"
	"chatbit/internal/adapter/pubsub"
	"chatbit/internal/adapter/vapid"
	"chatbit/internal/domain"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 120 * time.Second
	StaticPath        = "dist/"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 16 * 1024
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var pubsubAdaper *pubsub.AdapterTopic

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	logger.Info("ChatBit start")

	logger.Warn("Hello World! %s test", "SOME KEY", "warn")

	pubsubAdaper = pubsub.NewAdapterTopic()

	Start()

}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	path := filepath.Join(StaticPath, r.URL.Path)

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(StaticPath, "index.html"))

		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		log.Println("error : " + path)
		return
	}

	log.Println(path)

	http.FileServer(http.Dir(StaticPath)).ServeHTTP(w, r)
}

type Server struct {
	Server *http.Server
}

func Start() error {

	vapid.GenerateVapID()

	s := Server{}

	servMux := http.NewServeMux()
	servMux.Handle("/", &s)
	servMux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r)
	})

	servMux.HandleFunc("/vapidPublicKey", vapid.VapIDPubKeyFunc)
	servMux.HandleFunc("/subscribe", vapid.SubscribeFunc)
	servMux.HandleFunc("/send", vapid.SendFunc)

	servObj := &http.Server{
		Addr:              ":" + "8081",
		Handler:           servMux,
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

// serveWs handles websocket requests from the peer.
func serveWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{conn: conn, send: make(chan []byte, 256)}

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()

	client.subID = uuid.New().String()

	_, _ = pubsubAdaper.SubscribeWithID(client.subID, client.subID, func(obj interface{}) {
		slog.Info("New message")

		data, ok := obj.([]byte)
		if !ok {
			return
		}

		client.send <- data
	})

}

type Client struct {
	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send  chan []byte
	subID string
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		log.Println("client disconnected")
		c.conn.Close()
		pubsubAdaper.Unsubscribe(c.subID)
	}()

	log.Println("client connected")
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		wsMsg := domain.WsMessage{}

		_ = json.Unmarshal(message, &wsMsg)
		switch domain.WsMessageType(wsMsg.Type) {
		case domain.WsTypeMessage:
			msg := domain.Message{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			pubsubAdaper.Publish(msg.RoomID, message)
			slog.Info("Message published")
		case domain.WsTypeJoinRoom:
			msg := domain.JoinRooms{}

			_ = json.Unmarshal(wsMsg.Data, &msg)
			slog.Info("Join room: ", msg.Rooms, "subid:", c.subID)
			for _, room := range msg.Rooms {
				slog.Info("Join room: ", room, "subid:", c.subID)
				_ = pubsubAdaper.AddSubscriberToTopic(c.subID, room)
			}
		case domain.WsTypeRTCOffer:
			msg := domain.Offer{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			pubsubAdaper.Publish(msg.RoomID, message)
			slog.Info("RTC Offer published")
		case domain.WsTypeRTCAnswer:
			msg := domain.Answer{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			pubsubAdaper.Publish(msg.RoomID, message)
			slog.Info("RTC Answer published")
		case domain.WsTypeRTCCandidate:
			msg := domain.Candidate{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			pubsubAdaper.Publish(msg.RoomID, message)
			slog.Info("ICE Candidate published")
		case domain.WsTypePresenceRequest:
			msg := domain.PresenceRequest{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			pubsubAdaper.Publish(msg.RoomID, message)
			slog.Info("Presence Request published")
		case domain.WsTypePresenceAnswer:
			msg := domain.PresenceAnswer{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			pubsubAdaper.Publish(msg.RoomID, message)
			slog.Info("Presence Answer published")
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
