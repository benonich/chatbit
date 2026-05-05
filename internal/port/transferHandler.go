package port

import (
	"chatbit/internal/domain"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func (c *WsClient) handleTransferStart(hdr *domain.File) {
	slog.Debug("transfer_start", slog.String("file", hdr.ID.String()), slog.Int64("size", hdr.TotalSize))

	fileHdr := &domain.Transfer{
		File:     *hdr,
		Received: 0,
	}

	var err error

	if hdr.Store {
		err = os.MkdirAll(hdr.GetFileDir(), 0o755)
		if err != nil {
			slog.Error("cannot create dir", slog.String("error", err.Error()))
			return
		}

		fileHdr.OsFile, err = os.Create(hdr.GetFilePath())
		if err != nil {
			slog.Error("cannot create file", slog.String("error", err.Error()))
			return
		}

		err = fileHdr.OsFile.Truncate(hdr.TotalSize)
		if err != nil {
			slog.Error("cannot truncate file", slog.String("error", err.Error()))
			fileHdr.OsFile.Close()
			os.Remove(fileHdr.OsFile.Name())
			return
		}
	}

	if hdr.TTL > FileTTL || hdr.TTL == 0 {
		hdr.TTL = FileTTL
	}

	err = c.app.DB.File.AddWithTTL(hdr, hdr.TTL)
	if err != nil {
		slog.Error("not possible to add file header to database", slog.String("error", err.Error()))
		return
	}

	c.fileTransferMu.Lock()
	c.fileTransfer[hdr.ID] = fileHdr
	c.fileTransferMu.Unlock()

	c.sendACK(hdr.ID, 0)

	slog.Info("transfer started", slog.String("file", hdr.ID.String()), slog.Int64("size", hdr.TotalSize))

}

func (c *WsClient) handleTransferDone(done *domain.File) {
	c.fileTransferMu.Lock()
	t, ok := c.fileTransfer[done.ID]
	delete(c.fileTransfer, done.ID)
	c.fileTransferMu.Unlock()

	if !ok {
		return
	}

	t.OsFile.Close()

	if t.Received != t.ChunkTotal {
		slog.Warn("transfer incomplete",
			slog.Int64("received", t.Received),
			slog.Int64("expected", t.ChunkTotal),
		)

		err := os.Remove(t.GetFilePath())
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

	err = c.psWsMsgAdapter.PublishExclude(t.Message.RoomID, c.UID, WsOutbound{MsgType: websocket.TextMessage, Data: wsMsgB})
	if err != nil {
		slog.Error("not possible to update file in database", slog.String("error", err.Error()))
		return
	}
}

func (c *WsClient) handleChunk(data []byte) {
	uid, chunkId, payload, err := c.readFileHeader(data)
	if err != nil {
		slog.Error("not possible convert uuid from chunk", slog.String("error", err.Error()))
		return
	}

	c.fileTransferMu.Lock()
	t, ok := c.fileTransfer[uid]
	c.fileTransferMu.Unlock()

	if !ok {
		slog.Warn("chunk for unknown transfer", slog.String("id", uid.String()))
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
		_, err = t.OsFile.WriteAt(payload, offset)
		if err != nil {
			slog.Error("chunk write error", slog.String("error", err.Error()))

			c.sendNACK(uid, chunkId)

			return
		}
	}

	c.fileTransferMu.Lock()
	t.Received++
	c.fileTransferMu.Unlock()

	if t.Received == t.ChunkTotal {
		slog.Info("transfer done", slog.String("file", uid.String()))
		return
	}

	c.sendACK(uid, chunkId+1)
}

func (c *WsClient) sendFileTransferStart(file *domain.File) error {
	hdr, err := c.app.DB.File.Get(file.ID[:])
	if err != nil {
		slog.Error("not possible to get file header from database", slog.String("error", err.Error()))
		return err
	}

	f, err := os.Open(hdr.GetFilePath())
	if err != nil {
		slog.Error("cannot create temp file", slog.String("error", err.Error()))
		return err
	}

	slog.Info("transfer request started", slog.String("file", hdr.ID.String()), slog.Int64("size", hdr.TotalSize))

	err = c.psWsMsgAdapter.PublishTo(c.UID, c.UID, NewWsOutboundMessage(domain.WsTypeFileTransferStart, hdr))
	if err != nil {
		slog.Error("not possible to publish transfer start", slog.String("error", err.Error()))
	}

	buf := make([]byte, hdr.ChunkSize)
	var index int64

	for {
		n, err := f.Read(buf)
		if n > 0 {
			packet := c.addFileHeader(hdr.ID, index, buf[:n])

			err = c.psWsMsgAdapter.PublishTo(c.UID, c.UID, WsOutbound{MsgType: websocket.BinaryMessage, Data: packet})
			if err != nil {
				slog.Error("not possible to publish transfer start", slog.String("error", err.Error()))
			}

			index++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	err = c.psWsMsgAdapter.PublishTo(c.UID, c.UID, NewWsOutboundMessage(domain.WsTypeFileTransferDone, hdr.Message))
	if err != nil {
		slog.Error("not possible to publish transfer start", slog.String("error", err.Error()))
	}

	return nil
}

func (c *WsClient) addFileHeader(id uuid.UUID, index int64, chunk []byte) []byte {
	packet := make([]byte, 16+4+len(chunk))
	copy(packet[0:16], id[:])
	binary.BigEndian.PutUint32(packet[16:20], uint32(index))
	copy(packet[20:], chunk)
	return packet
}

func (c *WsClient) readFileHeader(data []byte) (uid uuid.UUID, index int64, chunk []byte, err error) {
	if len(data) < 20 {
		err = fmt.Errorf("invalid chunk size")
		return
	}

	ub := data[:16]
	id := fmt.Sprintf("%x-%x-%x-%x-%x", ub[0:4], ub[4:6], ub[6:8], ub[8:10], ub[10:16])

	uid, err = uuid.Parse(id)
	if err != nil {
		slog.Error("not possible convert uuid from chunk", slog.String("error", err.Error()))
		err = fmt.Errorf("invalid UUID")
		return
	}

	index = int64(binary.BigEndian.Uint32(data[16:20]))

	chunk = data[20:]

	return
}

func (c *WsClient) sendACK(id uuid.UUID, chunk int64) {
	err := c.psWsMsgAdapter.PublishTo(c.UID, c.UID, NewWsOutboundMessage(domain.WsTypeFileTransferACK, domain.TransferACK{ID: id, Chunk: chunk}))
	if err != nil {
		slog.Error("not possible to publish ACK", slog.String("error", err.Error()))
	}
}

func (c *WsClient) sendNACK(id uuid.UUID, chunk int64) {
	err := c.psWsMsgAdapter.PublishTo(c.UID, c.UID, NewWsOutboundMessage(domain.WsTypeFileTransferNACK, domain.TransferNACK{ID: id, Chunk: chunk}))
	if err != nil {
		slog.Error("not possible to publish ACK", slog.String("error", err.Error()))
	}
}
