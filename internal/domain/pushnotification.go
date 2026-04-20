package domain

import "github.com/google/uuid"

type Push struct {
	ID       string `json:"id,omitempty"`
	Endpoint string `json:"endpoint"`
	P256DH   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func (p Push) GetID() string {
	return p.ID
}
func (p Push) GetIDByte() []byte {
	parsedUUID, _ := uuid.Parse(p.ID)
	return parsedUUID[:]
}
