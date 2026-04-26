package domain

import (
	"time"

	"github.com/google/uuid"
)

type Peer struct {
	ID       string
	LastSeen time.Time
}

func (p Peer) GetID() string {
	return p.ID
}
func (p Peer) GetIDByte() []byte {
	parsedUUID, _ := uuid.Parse(p.ID)
	return parsedUUID[:]
}
