package port

import (
	"bytes"
	"chatbit/internal/adapter/pubsub"
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"encoding/json"
	"log"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
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

type WsClient struct {
	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send           chan []byte
	subID          string
	psWsMsgAdapter *pubsub.AdapterTopic[[]byte]
	app            *core.Application
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *WsClient) readPump() {
	defer func() {
		slog.Info("client disconnected", slog.String("subID", c.subID))
		c.conn.Close()
		c.psWsMsgAdapter.Unsubscribe(c.subID)
		c.app.PS.Message.Unsubscribe(c.subID)
	}()

	slog.Info("client connected", slog.String("subID:", c.subID))
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		slog.Debug("received pong websocket message", slog.String("subID", c.subID))
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

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

		err = json.Unmarshal(message, &wsMsg)
		if err != nil {
			slog.Error("not possible to unmarshal ws message ", slog.String("error", err.Error()))
		}

		switch domain.WsMessageType(wsMsg.Type) {
		case domain.WsTypeMessage:
			msg := domain.Message{}

			err = json.Unmarshal(wsMsg.Data, &msg)
			if err != nil {
				slog.Error("not possible to unmarshal ws message Message", slog.String("error", err.Error()))
				continue
			}

			c.app.PS.Message.Publish(msg.RoomID, msg)
			c.psWsMsgAdapter.Publish(msg.RoomID, message)
			slog.Debug("Message published", slog.String("roomID", msg.RoomID), slog.String("msgID:", msg.ID))
		case domain.WsTypeJoinRoom:
			msg := domain.JoinRooms{}

			err = json.Unmarshal(wsMsg.Data, &msg)
			if err != nil {
				slog.Error("not possible to unmarshal ws message JoinRooms", slog.String("error", err.Error()))
				continue
			}

			for _, room := range msg.Rooms {
				slog.Info("join room", slog.String("subID", c.subID), slog.String("roomID", room))
				c.psWsMsgAdapter.AddSubscriberToTopic(c.subID, room)
			}

		case domain.WsTypeLeaveRoom:
			msg := domain.LeaveRooms{}

			err = json.Unmarshal(wsMsg.Data, &msg)
			if err != nil {
				slog.Error("not possible to unmarshal ws message LeaveRooms", slog.String("error", err.Error()))
				continue
			}

			for _, room := range msg.Rooms {
				slog.Info("leave room", slog.String("roomID", room), slog.String("subID:", c.subID))
				c.psWsMsgAdapter.UnsubscribeFromTopic(c.subID, room)
			}

		case domain.WsTypeGetLastMessage:
			msg := domain.GetLastMessage{}

			err = json.Unmarshal(wsMsg.Data, &msg)
			if err != nil {
				slog.Error("not possible to unmarshal ws message GetLastMessage", slog.String("error", err.Error()))
				continue
			}

			allMs, err := c.app.DB.Message.GetAllSinceTimeStamp(msg.RoomID, msg.SinceTimeStamp)

			for _, m := range allMs {
				msgSend, err := json.Marshal(m)

				wsMsgSend := domain.WsMessage{
					Type: string(domain.WsTypeMessage),
					Data: msgSend,
				}

				msgSend, err := json.Marshal(wsMsgSend)

				c.send <- msgSend
			}

		case domain.WsTypePresenceRequest:
			msg := domain.PresenceRequest{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			c.psWsMsgAdapter.Publish(msg.RoomID, message)
			slog.Info("Presence Request published")

		case domain.WsTypePresenceAnswer:
			msg := domain.PresenceAnswer{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			c.psWsMsgAdapter.Publish(msg.RoomID, message)
			slog.Info("Presence Answer published")

		case domain.WsTypeRTCOffer:
			msg := domain.Offer{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			c.psWsMsgAdapter.Publish(msg.RoomID, message)
			slog.Info("RTC Offer published")
		case domain.WsTypeRTCAnswer:
			msg := domain.Answer{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			c.psWsMsgAdapter.Publish(msg.RoomID, message)
			slog.Info("RTC Answer published")
		case domain.WsTypeRTCCandidate:
			msg := domain.Candidate{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			c.psWsMsgAdapter.Publish(msg.RoomID, message)
			slog.Info("ICE Candidate published")

		}
	}
}

func (c *WsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				// There is no deadline set, so the connection is dead
				slog.Error("not possible to write deadliner", slog.String("error", err.Error()), slog.String("subID:", c.subID))
				return
			}
			if !ok {
				// The channel is closed
				slog.Error("channel closed from unsubscribe event", slog.String("subID:", c.subID))
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err = c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			slog.Debug("send ping websocket message", slog.String("subID:", c.subID))
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
		}
	}
}
