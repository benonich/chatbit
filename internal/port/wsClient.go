package port

import (
	"bytes"
	"chatbit/internal/adapter/pubsub"
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"os"
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

	slog.Info("peer connected", slog.String("peerID:", c.UID))

	err := c.app.DB.Peer.Add(domain.Peer{
		ID:       c.UID,
		LastSeen: time.Now(),
	})
	if err != nil {
		slog.Error("not possible update peer last connection time", slog.String("error", err.Error()))
	}

	c.conn.SetReadLimit(maxMessageSize)

	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(string) error {
		err := c.app.DB.Peer.Add(domain.Peer{
			ID:       c.UID,
			LastSeen: time.Now(),
		})
		if err != nil {
			slog.Error("not possible update peer last connection time", slog.String("error", err.Error()))
		}

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

			if msg.Store {
				err = c.app.PS.Message.Publish(msg)
				if err != nil {
					slog.Error("not possible to store message in database", slog.String("error", err.Error()))
				}
			}

			room, err := c.app.DB.Room.Get(msg.RoomID)
			if err != nil {
				slog.Error("not possible to get room", slog.String("error", err.Error()))
				continue
			}

			for _, peer := range room.Peers {
				err = c.psWsMsgAdapter.PublishTo(msg.RoomID, peer.ID, message)
				if err != nil {
					slog.Debug("not possible send message over ws, peer is offline", slog.String("peer", peer.ID), slog.String("error", err.Error()))

					p, err := c.app.DB.Peer.Get(peer.ID)
					if err != nil {
						slog.Error("not possible to get peer", slog.String("error", err.Error()))
					}

					if p.LastSeen.Before(time.Now().Add(-24 * 7 * time.Hour)) {
						slog.Info("peer offline, will be removed from room", slog.String("room", msg.RoomID), slog.String("peerID", peer.ID), slog.String("peerOfflineSince", p.LastSeen.Format(time.RFC3339)))

						err = c.app.DB.Room.Update(msg.RoomID, func(r *domain.Room) error {
							r.RemovePeer(peer.ID)

							slog.Info("remove peer from room", slog.String("room", r.ID), slog.String("peer", peer.ID))

							return nil
						})

						continue
					}

					if peer.Notification {
						slog.Info("send push notification", slog.String("peer", peer.ID), slog.String("peerOfflineSince", p.LastSeen.Format(time.RFC3339)))
						c.sendPushNotification(peer.ID)
					}

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
				slog.Info("peer will leave room", slog.String("room", room), slog.String("peer", c.UID))
				err = c.psWsMsgAdapter.UnsubscribeFromTopic(room, c.UID)
				if err != nil {
					slog.Error("not possible to unsubscribe from topic", slog.String("error", err.Error()))
					continue
				}

				err = c.app.DB.Room.Update(room, func(r *domain.Room) error {
					r.RemovePeer(c.UID)

					if len(r.Peers) == 0 {
						slog.Info("room has no peers, will be deleted", slog.String("room", room))

						r = nil

						return nil
					}

					return nil
				})
				if err != nil {
					slog.Error("not possible to update room", slog.String("error", err.Error()))
					continue
				}
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
				err = c.psWsMsgAdapter.AddSubscriberToTopic(room.ID, c.UID)
				if err != nil {
					slog.Error("not possible subscribe to room", slog.String("error", err.Error()))
					continue
				}

				err = c.app.DB.Room.Update(room.ID, func(r *domain.Room) error {
					r.AddPeer(c.UID, room.Notification)
					return nil
				})
				if errors.Is(err, domain.ErrObjNotFound) {
					slog.Info("peer will create new room", slog.String("room", room.ID), slog.String("peer", c.UID))
					r := &domain.Room{
						ID: room.ID,
						Peers: []domain.RoomPeer{
							{
								ID:           c.UID,
								Notification: room.Notification,
							},
						},
					}
					err = c.app.DB.Room.Add(r)
				}
				if err != nil && !errors.Is(err, domain.ErrObjNotFound) {
					slog.Error("not possible to update room", slog.String("error", err.Error()))
					continue
				}

				allMs, err = c.app.DB.Message.GetAllSinceTimeStamp(room.ID, msg.LastConnectionTime)
				if err != nil {
					slog.Error("not possible to get all messages since", slog.String("error", err.Error()))
					continue
				}

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

					slog.Debug("message sent", slog.String("UID", c.UID), slog.String("roomID", room.ID), slog.String("msgID:", m.ID))

					c.send <- msgSend
				}
			}
		case domain.WsTypeCreateRoom:
			room := domain.Room{}

			err = json.Unmarshal(wsMsg.Data, &room)
			if err != nil {
				slog.Error("not possible to unmarshal room infos", slog.String("error", err.Error()))
			}

			err = c.app.DB.Room.Add(&room)
			if err != nil {
				slog.Error("not possible to create new room", slog.String("error", err.Error()))
				continue
			}

		case domain.WsTypeSubscribePush:
			push := domain.Push{}

			err = json.Unmarshal(wsMsg.Data, &push)
			if err != nil {
				slog.Error("not possible to unmarshal subscribe push infos", slog.String("error", err.Error()))
			}

			push.ID = c.UID

			slog.Info("subscribe push notifications", slog.String("UID", c.UID))

			err = c.app.DB.Push.Add(push)
			if err != nil {
				slog.Error("not possible to add pushID", slog.String("error", err.Error()))
				continue
			}
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

			ticker.Reset(pingPeriod)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
		}
	}
}

func (c *WsClient) sendPushNotification(peer string) {
	pushID, err := c.app.DB.Push.Get(peer)
	if err != nil {
		slog.Error("not possible to get pushID", slog.String("error", err.Error()))
	}

	sub := &webpush.Subscription{
		Endpoint: pushID.Endpoint,
		Keys: webpush.Keys{
			P256dh: pushID.P256DH,
			Auth:   pushID.Auth,
		},
	}

	slog.Debug("Message published", slog.String("peer:", peer), slog.String("AliasID:", pushID.ID))

	body, _ := json.Marshal(map[string]string{
		"title": "- ChatBit -",
		"body":  "🔒 Neue Nachricht",
		"url":   "/",
	})

	option := webpush.Options{
		Subscriber:      "info@benoni.dev",
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		TTL:             30,
	}

	resp, err := webpush.SendNotification(body, sub, &option)
	if err != nil {
		slog.Error("not possible to send push notification", slog.String("error", err.Error()))
	}

	defer resp.Body.Close()
}
