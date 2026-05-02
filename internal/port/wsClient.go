package port

import (
	"bytes"
	"chatbit/internal/adapter/pubsub"
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
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
	UID            string
	psWsMsgAdapter *pubsub.AdapterTopic[WsOutbound]
	app            *core.Application

	// File Transfer
	fileTransfer   map[string]*domain.Transfer
	fileTransferMu sync.Mutex
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

			room, err := c.app.DB.Room.Get(msg.RoomID)
			if err != nil {
				slog.Error("not possible to get room", slog.String("error", err.Error()))
				continue
			}

			for _, peer := range room.Peers {
				err = c.psWsMsgAdapter.PublishTo(msg.RoomID, peer.ID, WsOutbound{MsgType: websocket.TextMessage, Data: message})
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

			slog.Info("subscribe push notifications", slog.String("UID", c.UID))

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
				slog.Error("not possible to write deadliner", slog.String("error", err.Error()), slog.String("UID:", c.UID))
				return
			}
			if !ok {
				// The channel is closed
				slog.Error("channel closed from unsubscribe event", slog.String("UID:", c.UID))
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err = c.conn.WriteMessage(message.MsgType, message.Data)
			if err != nil {
				return
			}

			ticker.Reset(pingPeriod)
		case <-ticker.C:
			slog.Debug("timeout", slog.String("UID:", c.UID))
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

const FileTTL = 60 * 60 * 24 * 7
const FileDir = "/Users/benoni/code/benoni/chatbit/files"

func (c *WsClient) sendFileTransferStart(fileId string) error {
	hdr, err := c.app.DB.File.Get(fileId)

	f, err := os.Open(filepath.Join(FileDir, hdr.ID))
	if err != nil {
		slog.Error("cannot create temp file", slog.String("error", err.Error()))
		return err
	}

	slog.Info("transfer request started", slog.String("file", hdr.ID), slog.Int64("size", hdr.TotalSize))

	transB, _ := json.Marshal(hdr)

	msg := domain.WsMessage{
		Type: string(domain.WsTypeFileTransferStart),
		Data: transB,
	}

	msgB, _ := json.Marshal(msg)

	c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: msgB}

	buf := make([]byte, hdr.ChunkSize)
	index := 0

	ub, err := uuid.Parse(hdr.ID)
	if err != nil {
		return err
	}

	for {
		n, err := f.Read(buf)
		if n > 0 {
			packet := make([]byte, 16+4+len(buf[:n]))
			copy(packet[0:16], ub[:])
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

func (c *WsClient) handleTransferStart(hdr *domain.File) {

	_, err := uuid.Parse(hdr.ID)
	if err != nil {
		slog.Error("transfer_start parse error", slog.String("error", err.Error()))
		return
	}

	slog.Debug("transfer_start", slog.String("file", hdr.ID), slog.Int64("size", hdr.TotalSize))

	f, err := os.Create(filepath.Join(FileDir, hdr.ID))
	if err != nil {
		slog.Error("cannot create temp file", slog.String("error", err.Error()))
		return
	}

	if err := f.Truncate(hdr.TotalSize); err != nil {
		slog.Error("cannot truncate temp file", slog.String("error", err.Error()))
		f.Close()
		os.Remove(f.Name())
		return
	}

	c.fileTransferMu.Lock()
	c.fileTransfer[hdr.ID] = &domain.Transfer{
		Message: domain.Message{
			ID:        hdr.ID,
			Type:      1,
			RoomID:    hdr.RoomID,
			Alias:     hdr.Alias,
			AliasID:   hdr.AliasID,
			Message:   "",
			TimeStamp: hdr.TimeStamp,
			Store:     hdr.Store,
			TTL:       hdr.TTL,
		},
		ChunkTotal: hdr.ChunkTotal,
		ChunkSize:  hdr.ChunkSize,
		Chunks:     hdr.Chunks,
		File:       f,
		FilePath:   f.Name(),
		Received:   0,
	}
	c.fileTransferMu.Unlock()

	err = c.app.DB.File.AddWithTTL(hdr, FileTTL)
	if err != nil {
		slog.Error("not possible to add file to database", slog.String("error", err.Error()))
		return
	}

	slog.Info("transfer started", slog.String("file", hdr.ID), slog.Int64("size", hdr.TotalSize))

	ackB, _ := json.Marshal(domain.TransferACK{ID: hdr.ID, Chunk: 0})
	// ACK send next package
	ack, _ := json.Marshal(domain.WsMessage{
		Type: string(domain.WsTypeFileTransferACK),
		Data: ackB,
	})
	c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: ack}
}

func (c *WsClient) handleTransferDone(done *domain.File) {
	c.fileTransferMu.Lock()
	t, ok := c.fileTransfer[done.ID]
	delete(c.fileTransfer, done.ID)
	c.fileTransferMu.Unlock()

	if !ok {
		return
	}

	t.File.Close()

	if t.Received != t.ChunkTotal {
		slog.Warn("transfer incomplete",
			slog.Int64("received", t.Received),
			slog.Int64("expected", t.ChunkTotal),
		)
		os.Remove(t.FilePath)
		return
	}

	err := c.app.PS.Message.Publish(t.Message)
	if err != nil {
		slog.Error("not possible to unmarshal ws message ", slog.String("error", err.Error()))
	}

	msgB, err := json.Marshal(t.Message)
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

	err = c.psWsMsgAdapter.Publish(t.Message.RoomID, WsOutbound{MsgType: websocket.TextMessage, Data: wsMsgB})

	if err != nil {
		slog.Error("not possible to update file in database", slog.String("error", err.Error()))
		return
	}
}

func (c *WsClient) handleChunk(data []byte) {
	if len(data) < 20 {
		return
	}
	ub := data[:16]
	id := fmt.Sprintf("%x-%x-%x-%x-%x", ub[0:4], ub[4:6], ub[6:8], ub[8:10], ub[10:16])

	chunkId := int64(binary.BigEndian.Uint32(data[16:20]))

	payload := data[20:]

	c.fileTransferMu.Lock()
	t, ok := c.fileTransfer[id]
	c.fileTransferMu.Unlock()
	if !ok {
		slog.Warn("chunk for unknown transfer", slog.String("id", id))
		return
	}

	// send chunk to all peers in rooms exclude own subID
	err := c.psWsMsgAdapter.PublishExclude(t.RoomID, c.UID, WsOutbound{MsgType: websocket.BinaryMessage, Data: data})
	if err != nil {
		slog.Error("not possible to publish chunk", slog.String("error", err.Error()))
	}

	if t.Store {
		// Direkt an den richtigen Offset schreiben – kein Assembly nötig
		offset := chunkId * t.ChunkSize // chunkSize = 64*1024
		if _, err := t.File.WriteAt(payload, offset); err != nil {
			slog.Error("chunk write error", slog.String("error", err.Error()))

			nackB, _ := json.Marshal(domain.TransferNACK{ID: id, Chunk: chunkId, Reason: "write_error"})

			// NACK send package again
			nack, _ := json.Marshal(domain.WsMessage{
				Type: string(domain.WsTypeFileTransferNACK),
				Data: nackB,
			})

			c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: nack}
			return
		}
	}

	c.fileTransferMu.Lock()
	t.Received++
	c.fileTransferMu.Unlock()

	if t.Received == t.ChunkTotal {
		slog.Info("transfer done", slog.String("file", id))
		return
	}

	ackB, _ := json.Marshal(domain.TransferACK{ID: id, Chunk: chunkId + 1})

	ack, _ := json.Marshal(domain.WsMessage{
		Type: string(domain.WsTypeFileTransferACK),
		Data: ackB,
	})
	c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: ack}
}
