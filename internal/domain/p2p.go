package domain

import "encoding/json"

type Offer struct {
	RoomID string          `json:"room_id,omitempty"`
	Offer  json.RawMessage `json:"offer,omitempty"`
}

type Answer struct {
	RoomID string          `json:"room_id,omitempty"`
	Answer json.RawMessage `json:"answer,omitempty"`
}

type Candidate struct {
	RoomID    string          `json:"room_id,omitempty"`
	Candidate json.RawMessage `json:"candidate,omitempty"`
}

type PresenceRequest struct {
	ID      string `json:"id,omitempty"`
	RoomID  string `json:"room_id,omitempty"`
	AliasID string `json:"alias_id,omitempty"`
}

type PresenceAnswer struct {
	ID      string `json:"id,omitempty"`
	RoomID  string `json:"room_id,omitempty"`
	AliasID string `json:"alias_id,omitempty"`
}
