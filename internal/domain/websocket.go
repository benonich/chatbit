package domain

import "encoding/json"

type WsMessage struct {
	Type string          `json:"type,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type WsMessageType string

const (
	WsTypeMessage       WsMessageType = "message"
	WsTypeLeaveRoom     WsMessageType = "leave_rooms"
	WsTypeJoinRoom      WsMessageType = "join_rooms"
	WsTypeCreateRoom    WsMessageType = "create_room"
	WsTypeSubscribePush WsMessageType = "sub_push"
)
