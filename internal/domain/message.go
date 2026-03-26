package domain

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
