package domain

import (
	"log/slog"
	"slices"

	"github.com/google/uuid"
)

type Room struct {
	ID    string     `json:"id,omitempty"`
	Name  string     `json:"name,omitempty"`
	Peers []RoomPeer `json:"peers,omitempty"`
}

type JoinRooms struct {
	Rooms              []RoomPeer `json:"rooms,omitempty"`
	LastConnectionTime uint64     `json:"last_connection_time,omitempty"`
}

type RoomPeer struct {
	ID           string `json:"id,omitempty"`
	Notification bool   `json:"notification,omitempty"`
}

type LeaveRooms struct {
	Rooms []string `json:"rooms,omitempty"`
}

func (r *Room) GetID() string {
	return r.ID
}
func (r *Room) GetIDByte() []byte {
	parsedUUID, _ := uuid.Parse(r.ID)
	return parsedUUID[:]
}

func (r *Room) RemovePeer(peerID string) {
	r.Peers = slices.DeleteFunc(r.Peers, func(v RoomPeer) bool {
		if v.ID == peerID {
			slog.Debug("remove peer from room", slog.String("peer", peerID), slog.String("room", r.ID))
			return true
		}

		return false
	})
}

func (r *Room) AddPeer(peerID string, notification bool) {
	if !slices.ContainsFunc(r.Peers, func(peer RoomPeer) bool {
		return peer.ID == peerID
	}) {
		slog.Info("add peer to room", slog.String("peer", peerID), slog.String("room", r.ID))
		r.Peers = append(r.Peers, RoomPeer{ID: peerID, Notification: notification})
	}
}
