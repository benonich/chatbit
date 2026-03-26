package port

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// ------------- Message Protokoll -------------

type SignalMessage struct {
	Type      string          `json:"type"` // "join", "offer", "answer", "candidate", "presence"
	RoomID    string          `json:"roomId,omitempty"`
	UserID    string          `json:"userId,omitempty"`    // für join / presence
	From      string          `json:"from,omitempty"`      // sender
	To        string          `json:"to,omitempty"`        // empfänger
	SDP       json.RawMessage `json:"sdp,omitempty"`       // offer/answer SDP
	Candidate json.RawMessage `json:"candidate,omitempty"` // ICE candidate
	Online    *bool           `json:"online,omitempty"`    // presence
	Peers     []string        `json:"peers,omitempty"`     // beim join: aktuelle Peers im Raum
}

// ------------- Client & Hub -------------

type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	userID string
	roomID string
}

type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[string]*Client // roomID -> userID -> client
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[string]*Client),
	}
}

// Client in Raum registrieren
func (h *Hub) Join(roomID, userID string, c *Client) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[string]*Client)
	}

	// Liste der vorhandenen Peers zurückgeben
	peers := make([]string, 0, len(h.rooms[roomID]))
	for uid := range h.rooms[roomID] {
		if uid != userID {
			peers = append(peers, uid)
		}
	}

	h.rooms[roomID][userID] = c
	c.roomID = roomID
	c.userID = userID

	return peers
}

// Client aus Raum entfernen
func (h *Hub) Leave(c *Client) {
	if c == nil || c.roomID == "" || c.userID == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[c.roomID]
	if !ok {
		return
	}

	delete(room, c.userID)
	if len(room) == 0 {
		delete(h.rooms, c.roomID)
	}
}

// Nachricht an bestimmten Peer in Raum schicken
func (h *Hub) SendTo(roomID, userID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room := h.rooms[roomID]
	if room == nil {
		return
	}
	if target, ok := room[userID]; ok {
		select {
		case target.send <- data:
		default:
			log.Printf("send buffer full for user %s in room %s", userID, roomID)
		}
	}
}

// Nachricht an alle im Raum broadcasten
func (h *Hub) Broadcast(roomID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room := h.rooms[roomID]
	if room == nil {
		return
	}
	for _, client := range room {
		select {
		case client.send <- data:
		default:
			log.Printf("send buffer full for user %s in room %s", client.userID, roomID)
		}
	}
}

// ------------- WebSocket Handler -------------

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Für Entwicklung: alles erlauben
		return true
	},
}

func main() {
	hub := NewHub()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, w, r)
	})

	log.Println("Signaling Server läuft auf :8080, Endpoint: /ws")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	go writePump(client)
	readPump(hub, client)
}

// Liest Messages vom Client
func readPump(hub *Hub, c *Client) {
	defer func() {
		hub.Leave(c)
		// presence offline broadcasten
		if c.roomID != "" && c.userID != "" {
			online := false
			msg := SignalMessage{
				Type:   "presence",
				RoomID: c.roomID,
				UserID: c.userID,
				Online: &online,
			}
			if data, err := json.Marshal(msg); err == nil {
				hub.Broadcast(c.roomID, data)
			}
		}
		c.conn.Close()
	}()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			break
		}

		var msg SignalMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Println("invalid json:", err)
			continue
		}

		switch msg.Type {

		case "join":
			// Raum beitreten
			if msg.RoomID == "" || msg.UserID == "" {
				log.Println("join missing roomId or userId")
				continue
			}

			peers := hub.Join(msg.RoomID, msg.UserID, c)

			// dem Joinenden seine Peerliste schicken
			selfOnline := true
			resp := SignalMessage{
				Type:   "joined",
				RoomID: msg.RoomID,
				UserID: msg.UserID,
				Peers:  peers,
				Online: &selfOnline,
			}
			if b, err := json.Marshal(resp); err == nil {
				c.send <- b
			}

			// den anderen Bescheid geben (presence)
			online := true
			pres := SignalMessage{
				Type:   "presence",
				RoomID: msg.RoomID,
				UserID: msg.UserID,
				Online: &online,
			}
			if b, err := json.Marshal(pres); err == nil {
				hub.Broadcast(msg.RoomID, b)
			}

		case "offer", "answer", "candidate":
			// direkt an den Zielpeer weiterleiten
			if msg.RoomID == "" || msg.To == "" {
				log.Println(msg.Type, "missing roomId or to")
				continue
			}
			// ursprüngliche Message unverändert weiterleiten
			hub.SendTo(msg.RoomID, msg.To, data)

		default:
			log.Println("unknown message type:", msg.Type)
		}
	}
}

// Schreibt Messages zum Client
func writePump(c *Client) {
	defer c.conn.Close()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// channel zu -> connection schließen
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Println("write error:", err)
				return
			}
		}
	}
}
