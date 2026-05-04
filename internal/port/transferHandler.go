package port

import (
	"chatbit/internal/domain"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func (c *WsClient) handleTransferStart(hdr *domain.File) {
	slog.Debug("transfer_start", slog.String("file", hdr.ID.String()), slog.Int64("size", hdr.TotalSize))

	var fileName string
	var file *os.File
	var err error

	if hdr.Store {
		file, err = os.Create(filepath.Join(FileDir, hdr.ID.String()))
		if err != nil {
			slog.Error("cannot create temp file", slog.String("error", err.Error()))
			return
		}

		if err := file.Truncate(hdr.TotalSize); err != nil {
			slog.Error("cannot truncate temp file", slog.String("error", err.Error()))
			file.Close()
			os.Remove(file.Name())
			return
		}

		fileName = file.Name()
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
		File:       file,
		FilePath:   fileName,
		Received:   0,
	}
	c.fileTransferMu.Unlock()

	err = c.app.DB.File.AddWithTTL(hdr, FileTTL)
	if err != nil {
		slog.Error("not possible to add file to database", slog.String("error", err.Error()))
		return
	}

	slog.Info("transfer started", slog.String("file", hdr.ID.String()), slog.Int64("size", hdr.TotalSize))

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

		err := os.Remove(t.FilePath)
		if err != nil {
			slog.Error("not possible to remove file", slog.String("error", err.Error()))
		}

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

	uid, err := uuid.Parse(id)
	if err != nil {
		slog.Error("not possible convert uuid from chunk", slog.String("error", err.Error()))
		return
	}

	chunkId := int64(binary.BigEndian.Uint32(data[16:20]))

	payload := data[20:]

	c.fileTransferMu.Lock()
	t, ok := c.fileTransfer[uid]
	c.fileTransferMu.Unlock()
	if !ok {
		slog.Warn("chunk for unknown transfer", slog.String("id", id))
		return
	}

	// send chunk to all peers in rooms exclude own subID
	err = c.psWsMsgAdapter.PublishExclude(t.RoomID, c.UID, WsOutbound{MsgType: websocket.BinaryMessage, Data: data})
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

	ackB, _ := json.Marshal(domain.TransferACK{ID: uid, Chunk: chunkId + 1})

	ack, _ := json.Marshal(domain.WsMessage{
		Type: string(domain.WsTypeFileTransferACK),
		Data: ackB,
	})
	c.send <- WsOutbound{MsgType: websocket.TextMessage, Data: ack}
}
