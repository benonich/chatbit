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
	"sync"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
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
	// Must be larger than the binary chunk size (64 KB + 20 byte header).
	maxMessageSize = 128 * 1024
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
	FileTTL = int64(60 * 60 * 24 * 7)
)

type WsOutbound struct {
	MsgType int // websocket.TextMessage oder websocket.BinaryMessage
	Data    []byte
}

func NewWsOutboundMessage(msgType domain.WsMessageType, msg any) WsOutbound {
	msgB, err := json.Marshal(msg)
	if err != nil {
		slog.Error("not possible to unmarshal ws message ", slog.String("error", err.Error()))
	}

	wsMsg := domain.WsMessage{
		Type: string(msgType),
		Data: msgB,
	}

	wsData, err := json.Marshal(wsMsg)
	if err != nil {
		slog.Error("not possible to unmarshal ws message ", slog.String("error", err.Error()))
	}

	return WsOutbound{MsgType: websocket.TextMessage, Data: wsData}
}

type WsClient struct {
	// The websocket connection.
	conn   *websocket.Conn
	ticker *time.Ticker

	// Buffered channel of outbound messages.
	send           chan WsOutbound
	UID            uuid.UUID
	psWsMsgAdapter *pubsub.AdapterTopic[uuid.UUID, WsOutbound]
	app            *core.Application

	// File Transfer
	fileTransfer   map[uuid.UUID]*domain.Transfer
	fileTransferMu sync.Mutex
}

func init() {
	domain.FileDir = os.Getenv("IMAGE_TMP_DIR")
	slog.Info("FileDir", slog.String("Dir", domain.FileDir))
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *WsClient) readPump() {
	defer func() {
		slog.Info("client disconnected readPump", slog.String("UID", c.UID.String()))
		c.ticker.Stop()
		c.conn.Close()
		c.psWsMsgAdapter.Unsubscribe(c.UID)
	}()

	slog.Info("peer connected", slog.String("peerID:", c.UID.String()))

	err := c.app.DB.Peer.Add(domain.Peer{
		ID:       c.UID,
		LastSeen: time.Now(),
	})
	if err != nil {
		slog.Error("not possible update peer last connection time", slog.String("error", err.Error()))
	}

	c.conn.SetReadLimit(maxMessageSize)

	c.conn.SetPongHandler(func(string) error {
		slog.Debug("receive pong", slog.String("UID:", c.UID.String()))

		err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			slog.Error("error during SetReadDeadline", slog.String("error", err.Error()))
		}

		return nil
	})

	for {
		msgType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}

			slog.Info("client disconnected", slog.String("UID", c.UID.String()), slog.String("error", err.Error()))

			break
		}

		if msgType == websocket.BinaryMessage {
			c.handleChunk(message)
			continue
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		wsMsg := domain.WsMessage{}

		err = json.Unmarshal(message, &wsMsg)
		if err != nil {
			slog.Error("not possible to unmarshal ws message ", slog.String("error", err.Error()))
		}

		switch domain.WsMessageType(wsMsg.Type) {
		case domain.WsTypeMessage:
			msg, ok := unmarshalWs[domain.Message](wsMsg.Data)
			if !ok {
				continue
			}

			if msg.Store {
				err = c.app.PS.Message.Publish(msg)
				if err != nil {
					slog.Error("not possible to store message in database", slog.String("error", err.Error()))
				}
			}

			room, err := c.app.DB.Room.Get(msg.RoomID[:])
			if err != nil {
				slog.Error("not possible to get room", slog.String("error", err.Error()))
				continue
			}

			for _, peer := range room.Peers {
				err = c.psWsMsgAdapter.PublishTo(msg.RoomID, peer.ID, WsOutbound{MsgType: websocket.TextMessage, Data: message})
				if err != nil {
					slog.Debug("not possible send message over ws, peer is offline", slog.String("peer", peer.ID.String()), slog.String("error", err.Error()))

					p, err := c.app.DB.Peer.Get(peer.ID[:])
					if err != nil {
						slog.Error("not possible to get peer", slog.String("error", err.Error()))
					}

					if p.LastSeen.Before(time.Now().Add(-24 * 7 * time.Hour)) {
						slog.Info("peer offline, will be removed from room", slog.String("room", msg.RoomID.String()), slog.String("peerID", peer.ID.String()), slog.String("peerOfflineSince", p.LastSeen.Format(time.RFC3339)))

						err = c.app.DB.Room.Update(msg.RoomID[:], func(r *domain.Room) error {
							r.RemovePeer(peer.ID)

							slog.Info("remove peer from room", slog.String("room", r.ID.String()), slog.String("peer", peer.ID.String()))

							return nil
						})

						continue
					}

					if peer.Notification {
						slog.Info("send push notification", slog.String("peer", peer.ID.String()), slog.String("peerOfflineSince", p.LastSeen.Format(time.RFC3339)))
						c.sendPushNotification(peer.ID)
					}

					continue
				}
			}
		case domain.WsTypeLeaveRoom:
			leave, ok := unmarshalWs[domain.LeaveRooms](wsMsg.Data)
			if !ok {
				continue
			}

			for _, room := range leave.Rooms {
				slog.Info("peer will leave room", slog.String("room", room.String()), slog.String("peer", c.UID.String()))
				err = c.psWsMsgAdapter.UnsubscribeFromTopic(room, c.UID)
				if err != nil {
					slog.Error("not possible to unsubscribe from topic", slog.String("error", err.Error()))
					continue
				}

				err = c.app.DB.Room.Update(room[:], func(r *domain.Room) error {
					r.RemovePeer(c.UID)

					if len(r.Peers) == 0 {
						slog.Info("room has no peers, will be deleted", slog.String("room", room.String()))

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
			join, ok := unmarshalWs[domain.JoinRooms](wsMsg.Data)
			if !ok {
				continue
			}

			var allMs []domain.Message
			var msgSend []byte

			for _, room := range join.Rooms {
				err = c.psWsMsgAdapter.AddSubscriberToTopic(room.ID, c.UID)
				if err != nil {
					slog.Error("not possible subscribe to room", slog.String("error", err.Error()))
					continue
				}

				err = c.app.DB.Room.Update(room.ID[:], func(r *domain.Room) error {
					r.AddPeer(c.UID, room.Notification)
					return nil
				})
				if errors.Is(err, domain.ErrObjNotFound) {
					slog.Info("peer will create new room", slog.String("room", room.ID.String()), slog.String("peer", c.UID.String()))
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

				allMs, err = c.app.DB.Message.GetAllSinceTimeStamp(room.ID, join.LastConnectionTime)
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

					slog.Debug("message sent", slog.String("UID", c.UID.String()), slog.String("roomID", room.ID.String()), slog.String("msgID:", m.ID.String()))

					c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: msgSend}
				}
			}
		case domain.WsTypeCreateRoom:
			room, ok := unmarshalWs[domain.Room](wsMsg.Data)
			if !ok {
				continue
			}

			err = c.app.DB.Room.Add(&room)
			if err != nil {
				slog.Error("not possible to create new room", slog.String("error", err.Error()))
				continue
			}

		case domain.WsTypeSubscribePush:
			push, ok := unmarshalWs[domain.Push](wsMsg.Data)
			if !ok {
				continue
			}

			push.ID = c.UID

			slog.Info("subscribe push notifications", slog.String("UID", c.UID.String()))

			err = c.app.DB.Push.Add(push)
			if err != nil {
				slog.Error("not possible to add pushID", slog.String("error", err.Error()))
				continue
			}
		case domain.WsTypeFileTransferStart:
			file, ok := unmarshalWs[domain.File](wsMsg.Data)
			if !ok {
				continue
			}

			c.handleTransferStart(&file)

			err = c.psWsMsgAdapter.PublishExclude(file.RoomID, c.UID, WsOutbound{MsgType: websocket.TextMessage, Data: message})
			if err != nil {
				slog.Error("not possible to handle transfer start", slog.String("error", err.Error()))
				continue
			}
		case domain.WsTypeFileTransferDone:
			file, ok := unmarshalWs[domain.File](wsMsg.Data)
			if !ok {
				continue
			}

			c.handleTransferDone(&file)

			err = c.psWsMsgAdapter.Publish(file.RoomID, WsOutbound{MsgType: websocket.TextMessage, Data: message})
			if err != nil {
				slog.Error("not possible to handle transfer done", slog.String("error", err.Error()))
				continue
			}

		case domain.WsTypeFileTransferRequest:
			file, ok := unmarshalWs[domain.File](wsMsg.Data)
			if !ok {
				continue
			}

			err = c.sendFileTransferStart(&file)
			if err != nil {
				slog.Error("not possible to start transfer", slog.String("error", err.Error()))
				continue
			}
		}
	}
}

func (c *WsClient) writePump() {
	c.ticker = time.NewTicker(pingPeriod)
	defer func() {
		slog.Info("client disconnected writePump", slog.String("UID", c.UID.String()))
		c.ticker.Stop()
		c.conn.Close()
	}()

	var err error

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// The channel is closed
				slog.Error("channel closed from unsubscribe event", slog.String("UID:", c.UID.String()))
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				// There is no deadline set, so the connection is dead
				slog.Error("not possible to write deadliner", slog.String("error", err.Error()), slog.String("UID:", c.UID.String()))
				return
			}

			err = c.conn.WriteMessage(message.MsgType, message.Data)
			if err != nil {
				slog.Error("not possible to write message", slog.String("error", err.Error()), slog.String("UID:", c.UID.String()))

				return
			}

		case <-c.ticker.C:
			err = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				// There is no deadline set, so the connection is dead
				slog.Error("not possible to write deadliner", slog.String("error", err.Error()), slog.String("UID:", c.UID.String()))
				return
			}

			slog.Debug("send ping", slog.String("UID:", c.UID.String()))

			err = c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				slog.Error("not possible to write Ping message", slog.String("error", err.Error()), slog.String("UID:", c.UID.String()))

				return
			}
		}
	}
}

func unmarshalWs[T any](data json.RawMessage) (T, bool) {
	var v T
	err := json.Unmarshal(data, &v)
	if err != nil {
		slog.Error("not possible to Unmarshal message", slog.String("error", err.Error()))
		return v, false
	}

	return v, true
}

func (c *WsClient) sendPushNotification(peerID uuid.UUID) {
	pushID, err := c.app.DB.Push.Get(peerID[:])
	if err != nil {
		slog.Error("not possible to get pushID", slog.String("error", err.Error()))
		return
	}

	sub := &webpush.Subscription{
		Endpoint: pushID.Endpoint,
		Keys: webpush.Keys{
			P256dh: pushID.P256DH,
			Auth:   pushID.Auth,
		},
	}

	slog.Debug("Message published", slog.String("peer:", peerID.String()), slog.String("AliasID:", pushID.ID.String()))

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
