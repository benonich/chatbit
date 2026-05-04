package port

import (
	"bytes"
	"chatbit/internal/adapter/pubsub"
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
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
	FileDir = "/Users/benoni/code/benoni/chatbit/files"
)

type WsOutbound struct {
	MsgType int // websocket.TextMessage oder websocket.BinaryMessage
	Data    []byte
}

type WsClient struct {
	// The websocket connection.
	conn *websocket.Conn

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
	FileDir = os.Getenv("IMAGE_TMP_DIR")
	slog.Info("FileDir", slog.String("Dir", FileDir))
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *WsClient) readPump() {
	defer func() {
		slog.Info("client disconnected", slog.String("UID", c.UID.String()))
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
		msgType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
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
			msg := domain.LeaveRooms{}

			err = json.Unmarshal(wsMsg.Data, &msg)
			if err != nil {
				slog.Error("not possible to unmarshal ws message LeaveRooms", slog.String("error", err.Error()))
				continue
			}

			for _, room := range msg.Rooms {
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

					slog.Debug("message sent", slog.String("UID", c.UID.String()), slog.String("roomID", room.ID.String()), slog.String("msgID:", m.ID.String()))

					c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: msgSend}
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

			slog.Info("subscribe push notifications", slog.String("UID", c.UID.String()))

			err = c.app.DB.Push.Add(push)
			if err != nil {
				slog.Error("not possible to add pushID", slog.String("error", err.Error()))
				continue
			}
		case domain.WsTypeFileTransferStart:
			var file = &domain.File{}

			err = json.Unmarshal(wsMsg.Data, file)
			if err != nil {
				slog.Error("transfer_start parse error", slog.String("error", err.Error()))
				continue
			}

			c.handleTransferStart(file)

			err = c.psWsMsgAdapter.PublishExclude(file.RoomID, c.UID, WsOutbound{MsgType: websocket.TextMessage, Data: message})
			if err != nil {
				slog.Error("not possible to add pushID", slog.String("error", err.Error()))
				continue
			}
		case domain.WsTypeFileTransferDone:
			var file = &domain.File{}

			err = json.Unmarshal(wsMsg.Data, file)
			if err != nil {
				slog.Error("transfer_start parse error", slog.String("error", err.Error()))
				continue
			}

			c.handleTransferDone(file)

			err = c.psWsMsgAdapter.Publish(file.RoomID, WsOutbound{MsgType: websocket.TextMessage, Data: message})
			if err != nil {
				slog.Error("not possible to add pushID", slog.String("error", err.Error()))
				continue
			}

		case domain.WsTypeFileTransferRequest:
			var file = &domain.File{}

			err = json.Unmarshal(wsMsg.Data, file)
			if err != nil {
				slog.Error("transfer_start parse error", slog.String("error", err.Error()))
				continue
			}

			c.sendFileTransferStart(file.ID)
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
				slog.Error("not possible to write deadliner", slog.String("error", err.Error()), slog.String("UID:", c.UID.String()))
				return
			}
			if !ok {
				// The channel is closed
				slog.Error("channel closed from unsubscribe event", slog.String("UID:", c.UID.String()))
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err = c.conn.WriteMessage(message.MsgType, message.Data)
			if err != nil {
				return
			}

			ticker.Reset(pingPeriod)
		case <-ticker.C:
			slog.Debug("timeout", slog.String("UID:", c.UID.String()))
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				return
			}
		}
	}
}

func (c *WsClient) sendPushNotification(peerID uuid.UUID) {
	pushID, err := c.app.DB.Push.Get(peerID[:])
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

func (c *WsClient) sendFileTransferStart(fileId uuid.UUID) error {
	// TODO is needed?
	hdr, err := c.app.DB.File.Get(fileId[:])

	f, err := os.Open(filepath.Join(FileDir, hdr.ID.String()))
	if err != nil {
		slog.Error("cannot create temp file", slog.String("error", err.Error()))
		return err
	}

	slog.Info("transfer request started", slog.String("file", hdr.ID.String()), slog.Int64("size", hdr.TotalSize))

	transB, _ := json.Marshal(hdr)

	msg := domain.WsMessage{
		Type: string(domain.WsTypeFileTransferStart),
		Data: transB,
	}

	msgB, _ := json.Marshal(msg)

	c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: msgB}

	buf := make([]byte, hdr.ChunkSize)
	index := 0

	for {
		n, err := f.Read(buf)
		if n > 0 {
			packet := make([]byte, 16+4+len(buf[:n]))
			copy(packet[0:16], hdr.ID[:])
			binary.BigEndian.PutUint32(packet[16:20], uint32(index))
			copy(packet[20:], buf[:n])
			c.send <- WsOutbound{MsgType: websocket.BinaryMessage, Data: packet}
			index++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	msgB, err = json.Marshal(hdr.Message)
	if err != nil {
		slog.Error("not possible to unmarshal ws message ", slog.String("error", err.Error()))
	}

	wsMsg := domain.WsMessage{
		Type: string(domain.WsTypeFileTransferDone),
		Data: msgB,
	}

	wsMsgB, err := json.Marshal(wsMsg)
	if err != nil {
		slog.Error("not possible to unmarshal ws message ", slog.String("error", err.Error()))
	}

	c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: wsMsgB}

	return nil
}
