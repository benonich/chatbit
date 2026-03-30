package domain

import "github.com/google/uuid"

type Room struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name,omitempty"`
	Peers   []string `json:"peers,omitempty"`
	P2POnly bool     `json:"p2p_only,omitempty"` // should the room use P2P
	TTL     int64    `json:"ttl,omitempty"`
}

type JoinRooms struct {
	Rooms              []string `json:"rooms,omitempty"`
	LastConnectionTime uint64   `json:"last_connection_time,omitempty"`
}

type LeaveRooms struct {
	Rooms []string `json:"rooms,omitempty"`
}

func (r Room) GetID() string {
	return r.ID
}
func (r Room) GetIDByte() []byte {
	parsedUUID, _ := uuid.Parse(r.ID)
	return parsedUUID[:]
}
