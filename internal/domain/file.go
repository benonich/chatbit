package domain

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

var FileDir = "/Users/benoni/code/benoni/chatbit/files"

type File struct {
	Message

	TotalSize  int64  `json:"totalSize"`
	ChunkTotal int64  `json:"chunksTotal"`
	ChunkSize  int64  `json:"chunkSize"`
	Chunks     int64  `json:"chunks"`
	MimeType   string `json:"mimeType"`
	SHA256     string `json:"sha256"`
}

type Transfer struct {
	File
	OsFile   *os.File
	Received int64 // chunks empfangen
}

type TransferACK struct {
	ID    uuid.UUID `json:"id"`
	Chunk int64     `json:"chunk"`
}

type TransferNACK struct {
	ID     uuid.UUID `json:"id"`
	Chunk  int64     `json:"chunk"`
	Reason string    `json:"reason"`
}

func (m File) GetID() []byte {
	return m.ID[:]
}

func (m File) GetFileDir() string {
	return filepath.Join(FileDir, m.RoomID.String())
}

func (m File) GetFilePath() string {
	return filepath.Join(FileDir, m.RoomID.String(), m.ID.String())
}
