package domain

import (
	"log/slog"
	"slices"

	"github.com/google/uuid"
)

type Room struct {
	ID    uuid.UUID  `json:"id,omitempty"`
	Name  string     `json:"name,omitempty"`
	Peers []RoomPeer `json:"peers,omitempty"`
}

type JoinRooms struct {
	Rooms              []RoomPeer `json:"rooms,omitempty"`
	LastConnectionTime uint64     `json:"last_connection_time,omitempty"`
}

type RoomPeer struct {
	ID           uuid.UUID `json:"id,omitempty"`
	Notification bool      `json:"notification,omitempty"`
}

type LeaveRooms struct {
	Rooms []uuid.UUID `json:"rooms,omitempty"`
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
		slog.Info("add peer to room", slog.String("peer", peerID.String()), slog.String("room", r.ID.String()))
		r.Peers = append(r.Peers, RoomPeer{ID: peerID, Notification: notification})
	}
}
