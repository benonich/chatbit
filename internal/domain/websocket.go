package domain

import "encoding/json"

type WsMessage struct {
	Type string          `json:"type,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type WsMessageType string

const (
	WsTypeMessage         WsMessageType = "message"
	WsTypeWelcome         WsMessageType = "hello"
	WsTypeLeaveRoom       WsMessageType = "leave_rooms"
	WsTypeJoinRoom        WsMessageType = "join_rooms"
	WsTypeRTCOffer        WsMessageType = "rtc_offer"
	WsTypeRTCAnswer       WsMessageType = "rtc_answer"
	WsTypeRTCCandidate    WsMessageType = "rtc_candidate"
	WsTypePresenceRequest WsMessageType = "presence_request"
	WsTypePresenceAnswer  WsMessageType = "presence_answer"
)
