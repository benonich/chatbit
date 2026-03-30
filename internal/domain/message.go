package domain

import "github.com/google/uuid"

type Message struct {
	ID        string            `json:"id,omitempty"`
	RoomID    string            `json:"room_id,omitempty"`
	Alias     string            `json:"alias,omitempty"`
	AliasID   string            `json:"alias_id,omitempty"`
	Message   string            `json:"message,omitempty"`
	TimeStamp int64             `json:"timestamp,omitempty"`
	Received  []MessageReceived `json:"received,omitempty"` // Alias of the receivers
}
type MessageReceived struct {
	AliasID   string `json:"alias_id,omitempty"`
	TimeStamp int64  `json:"timestamp,omitempty"`
}

func (m Message) GetID() string {
	return m.ID
}

func (m Message) GetIDByte() []byte {
	parsedUUID, _ := uuid.Parse(m.ID)
	return parsedUUID[:]
}

func (m Message) GetRoomID() string {
	return m.RoomID
}

func (m Message) GetRoomIDByte() []byte {
	parsedUUID, _ := uuid.Parse(m.RoomID)
	return parsedUUID[:]
}

type GetLastMessage struct {
	RoomID         string `json:"room_id,omitempty"`
	SinceTimeStamp uint64 `json:"since_timestamp,omitempty"`
}
