package domain

import (
	"log/slog"
	"slices"

	"github.com/google/uuid"
)

// MaxMessageTTL is the longest time in seconds the server keeps a message (30 days).
const MaxMessageTTL = int64(60 * 60 * 24 * 30)

// MessageTTLSteps are the only shorter TTLs a message can have (5 min, 1 h, 1 d, 7 d).
// A freely chosen TTL would be visible to the server on every message of the same sender
// and so mark them as coming from one person; with a few fixed steps many people share one.
var MessageTTLSteps = []int64{5 * 60, 60 * 60, 24 * 60 * 60, 7 * 24 * 60 * 60}

type Room struct {
	ID    uuid.UUID  `json:"id,omitempty"`
	Name  string     `json:"name,omitempty"`
	Peers []RoomPeer `json:"peers,omitempty"`

	// Maximum TTL of all messages in seconds. It is set once when the room is created on the
	// server and can not be changed afterwards. A message may only ask for a shorter TTL.
	TTL int64 `json:"ttl,omitempty"`
}

type JoinRooms struct {
	Rooms              []JoinRoom `json:"rooms,omitempty"`
	LastConnectionTime uint64     `json:"lastConnectionTime,omitempty"`
}

type JoinRoom struct {
	ID           uuid.UUID `json:"id,omitempty"`
	Notification bool      `json:"notification,omitempty"`

	// Only used if the room does not exist on the server yet (or has no TTL yet)
	TTL int64 `json:"ttl,omitempty"`
}

// RoomInfo is sent to the client after joining, so it knows the TTL that is in force.
type RoomInfo struct {
	ID  uuid.UUID `json:"id"`
	TTL int64     `json:"ttl"`
}

type RoomPeer struct {
	ID           uuid.UUID `json:"id,omitempty"`
	Notification bool      `json:"notification,omitempty"`
}

type LeaveRooms struct {
	Rooms []uuid.UUID `json:"rooms,omitempty"`
}

// ClampTTL limits a TTL to (0, MaxMessageTTL]; 0 or negative means the maximum.
func ClampTTL(ttl int64) int64 {
	if ttl <= 0 || ttl > MaxMessageTTL {
		return MaxMessageTTL
	}

	return ttl
}

// SetTTLOnce sets the TTL if the room has none yet and returns the TTL in force.
func (r *Room) SetTTLOnce(ttl int64) int64 {
	if r.TTL == 0 {
		r.TTL = ClampTTL(ttl)
	}

	return r.TTL
}

// MessageTTL returns the TTL a message gets in this room: the room TTL, or — if a shorter TTL
// is requested — the largest of the MessageTTLSteps that is not longer than the request
// (at least the smallest step). It is never longer than the room TTL. 0 = no own TTL requested.
func (r *Room) MessageTTL(requested int64) int64 {
	roomTTL := ClampTTL(r.TTL)
	if requested <= 0 || requested >= roomTTL {
		return roomTTL
	}

	ttl := MessageTTLSteps[0]
	for _, step := range MessageTTLSteps {
		if step <= requested {
			ttl = step
		}
	}

	return min(ttl, roomTTL)
}

func (r *Room) GetID() []byte {
	return r.ID[:]
}

func (r *Room) RemovePeer(peerID uuid.UUID) {
	r.Peers = slices.DeleteFunc(r.Peers, func(v RoomPeer) bool {
		if v.ID == peerID {
			slog.Debug("remove peer from room", slog.String("peer", peerID.String()), slog.String("room", r.ID.String()))
			return true
		}

		return false
	})
}

func (r *Room) AddPeer(peerID uuid.UUID, notification bool) {
	if !slices.ContainsFunc(r.Peers, func(peer RoomPeer) bool {
		return peer.ID == peerID
	}) {
		slog.Debug("add peer to room", slog.String("peer", peerID.String()), slog.String("room", r.ID.String()))
		r.Peers = append(r.Peers, RoomPeer{ID: peerID, Notification: notification})
	}
}
