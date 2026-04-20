package port

import (
	"bytes"
	"chatbit/internal/adapter/pubsub"
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/SherClockHolmes/webpush-go"
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
	UID            string
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
		slog.Info("client disconnected", slog.String("UID", c.UID))
		c.conn.Close()
		c.psWsMsgAdapter.Unsubscribe(c.UID)
	}()

	slog.Info("client connected", slog.String("UID:", c.UID))
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		slog.Debug("received pong websocket message", slog.String("UID", c.UID))
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

			c.app.PS.Message.Publish(msg)
			//c.psWsMsgAdapter.Publish(msg.RoomID, message)
			//slog.Debug("Message published", slog.String("roomID", msg.RoomID), slog.String("msgID:", msg.ID))

			room, err := c.app.DB.Room.Get(msg.RoomID)
			if err != nil {
				slog.Error("not possible to get room", slog.String("error", err.Error()))
				continue
			}
			for _, peer := range room.Peers {
				err = c.psWsMsgAdapter.PublishTo(msg.RoomID, peer, message)
				if err != nil {
					slog.Error("not possible to publish message, send Notification", slog.String("error", err.Error()))
					pushID, err := c.app.DB.Push.Get(peer)
					if err != nil {
						slog.Error("not possible to get pushID", slog.String("error", err.Error()))
						continue
					}
					sub := &webpush.Subscription{
						Endpoint: pushID.Endpoint,
						Keys: webpush.Keys{
							P256dh: pushID.P256DH,
							Auth:   pushID.Auth,
						},
					}
					slog.Debug("Message published", slog.String("roomID", msg.RoomID), slog.String("msgID:", msg.ID), slog.String("AliasID:", pushID.ID))
					body, _ := json.Marshal(map[string]string{
						"title": "ChatBit",
						"body":  "NEW MESSAGE FROM ChatBit",
						"url":   "/",
					})

					option := webpush.Options{
						VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
						VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
						TTL:             30,
					}

					resp, err := webpush.SendNotification(body, sub, &option)
					if err != nil {
						slog.Error("not possible to send push notification", slog.String("error", err.Error()))
					}
					defer resp.Body.Close()

					continue
				}
			}

		case domain.WsTypeLeaveRoom:
			msg := domain.LeaveRooms{}

			err = json.Unmarshal(wsMsg.Data, &msg)
			if err != nil {
				slog.Error("not possible to unmarshal ws message LeaveRooms", slog.String("error", err.Error()))
				continue
			}

			for _, room := range msg.Rooms {
				slog.Info("leave room", slog.String("roomID", room), slog.String("UID:", c.UID))
				c.psWsMsgAdapter.UnsubscribeFromTopic(c.UID, room)
			}

		case domain.WsTypeJoinRoom:
			msg := domain.JoinRooms{}

			err = json.Unmarshal(wsMsg.Data, &msg)
			if err != nil {
				slog.Error("not possible to unmarshal ws message JoinRooms", slog.String("error", err.Error()))
				continue
			}

			var allMs []domain.Message
			var msgSend []byte

			for _, room := range msg.Rooms {
				slog.Info("join room", slog.String("UID", c.UID), slog.String("roomID", room))
				c.psWsMsgAdapter.AddSubscriberToTopic(c.UID, room)

				// TODO add mutex
				rooms, err := c.app.DB.Room.Get(room)
				if !slices.Contains(rooms.Peers, c.UID) {
					slog.Info("add peer to room", slog.String("UID", c.UID), slog.String("roomID", room))
					rooms.Peers = append(rooms.Peers, c.UID)
					rooms.ID = room
					err = c.app.DB.Room.Add(rooms)
					if err != nil {
						slog.Error("not possible to add peer to room", slog.String("error", err.Error()))
					}
				}

				allMs, err = c.app.DB.Message.GetAllSinceTimeStamp(room, msg.LastConnectionTime)

				for _, m := range allMs {

					msgSend, err = json.Marshal(m)
					if err != nil {
						slog.Error("not possible to marshal Message", slog.String("error", err.Error()))
						continue
					}

					wsMsgSend := domain.WsMessage{
						Type: string(domain.WsTypeMessage),
						Data: msgSend,
					}

					msgSend, err = json.Marshal(wsMsgSend)
					if err != nil {
						slog.Error("not possible to marshal WsMessage", slog.String("error", err.Error()))
						continue
					}

					slog.Debug("Message sent", slog.String("UID", c.UID), slog.String("roomID", room), slog.String("msgID:", m.ID))

					c.send <- msgSend
				}
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

		case domain.WsTypeSubscribePush:
			msg := domain.Push{}

			_ = json.Unmarshal(wsMsg.Data, &msg)

			msg.ID = c.UID

			err = c.app.DB.Push.Add(msg)
			if err != nil {
				slog.Error("not possible to add pushID", slog.String("error", err.Error()))
			}
			slog.Info("Subscribe Request received")
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
				slog.Error("not possible to write deadliner", slog.String("error", err.Error()), slog.String("UID:", c.UID))
				return
			}
			if !ok {
				// The channel is closed
				slog.Error("channel closed from unsubscribe event", slog.String("UID:", c.UID))
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err = c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			slog.Debug("send ping websocket message", slog.String("UID:", c.UID))
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
		}
	}
}
