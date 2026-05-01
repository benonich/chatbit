package domain

import (
	"os"

	"github.com/google/uuid"
)

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
	Message

	ChunkTotal int64 `json:"chunks_total"`
	ChunkSize  int64 `json:"chunk_size"`
	Chunks     int64 `json:"chunks"`
	File       *os.File
	FilePath   string
	Received   int64 // chunks empfangen
}

type TransferACK struct {
	ID    string `json:"id"`
	Chunk int64  `json:"chunk"`
}

type TransferNACK struct {
	ID     string `json:"id"`
	Chunk  int64  `json:"chunk"`
	Reason string `json:"reason"`
}

func (m File) GetID() string {
	return m.ID
}

func (m File) GetIDByte() []byte {
	parsedUUID, _ := uuid.Parse(m.ID)
	return parsedUUID[:]
}
