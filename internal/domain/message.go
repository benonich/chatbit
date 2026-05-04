package domain

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type Message struct {
	ID        uuid.UUID `json:"id,omitempty"`
	Type      int64     `json:"type,omitempty"` // 0: text, 1: image, 2: file
	RoomID    uuid.UUID `json:"room_id,omitempty"`
	Alias     string    `json:"alias,omitempty"`
	AliasID   string    `json:"alias_id,omitempty"`
	Message   string    `json:"message,omitempty"`
	TimeStamp uint64    `json:"timestamp,omitempty"`
	Store     bool      `json:"store,omitempty"`
	TTL       int64     `json:"ttl,omitempty"`
}

func (m Message) GetID() []byte {
	return append(m.RoomID[:], m.ID[:]...)
}

func (m Message) GetRoomID() []byte {
	return m.RoomID[:]
}

func (m Message) GetTimestamp() uint64 {
	return m.TimeStamp
}

type GetLastMessage struct {
	RoomID         string `json:"room_id,omitempty"`
	SinceTimeStamp uint64 `json:"since_timestamp,omitempty"`
}

type UUID string

func (u *UUID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if _, err := uuid.Parse(s); err != nil {
		return fmt.Errorf("invalid uuid: %w", err)
	}
	*u = UUID(s)
	return nil
}
