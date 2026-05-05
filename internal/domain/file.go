package domain

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

var FileDir = "/Users/benoni/code/benoni/chatbit/files"

type File struct {
	Message

	TotalSize  int64  `json:"total_size"`
	ChunkTotal int64  `json:"chunks_total"`
	ChunkSize  int64  `json:"chunk_size"`
	Chunks     int64  `json:"chunks"`
	MimeType   string `json:"mime_type"`
	SHA256     string `json:"sha_256"`
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
