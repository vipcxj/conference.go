package proto

import (
	gproto "google.golang.org/protobuf/proto"
)

type RoomMessage interface {
	gproto.Message
	GetRouter() *Router
	FixRouter(room string, user string, node string)
	CopyPlain() RoomMessage
}

type CanToMap interface {
	ToMap() map[string]any
}

func ToClientMessage(message RoomMessage) any {
	can_to_map, ok := message.(CanToMap)
	if ok {
		msg_map := can_to_map.ToMap()
		if msg_map != nil {
			router, ok := msg_map["router"]
			if ok {
				router_map, ok := router.(map[string]any)
				if ok {
					delete(router_map, "nodeFrom")
					delete(router_map, "nodeTo")
				}
			}
		}
		return msg_map
	} else {
		msg := message.CopyPlain()
		msg.GetRouter().NodeFrom = ""
		msg.GetRouter().NodeTo = ""
		return msg
	}
}

func (x *Router) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"room":     x.GetRoom(),
		"nodeFrom": x.GetNodeFrom(),
		"nodeTo":   x.GetNodeTo(),
		"userFrom": x.GetUserFrom(),
		"userTo":   x.GetUserTo(),
	}
}

func (x *Router) CopyPlain() *Router {
	return &Router{
		Room:     x.GetRoom(),
		NodeFrom: x.GetNodeFrom(),
		NodeTo:   x.GetNodeTo(),
		UserFrom: x.GetUserFrom(),
		UserTo:   x.GetUserTo(),
	}
}

func fixRouter(router *Router, room string, user string, node string) {
	if router.Room == "" {
		router.Room = room
	}
	router.UserFrom = user
	router.NodeFrom = node
}

func (x *WantMessage) FixRouter(room string, user string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, user, node)
}

func (x *WantMessage) CopyPlain() RoomMessage {
	return &WantMessage{
		Router:      x.GetRouter().CopyPlain(),
		ReqTypes:    x.GetReqTypes(),
		Pattern:     x.GetPattern(),
		TransportId: x.GetTransportId(),
	}
}

func (x *StateMessage) FixRouter(room string, user string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, user, node)
}

func (x *StateMessage) CopyPlain() RoomMessage {
	return &StateMessage{
		Router: x.GetRouter().CopyPlain(),
		PubId:  x.GetPubId(),
		Addr:   x.GetAddr(),
		Tracks: x.GetTracks(),
	}
}

func (x *SelectMessage) FixRouter(room string, user string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, user, node)
}

func (x *SelectMessage) CopyPlain() RoomMessage {
	return &SelectMessage{
		Router:      x.GetRouter().CopyPlain(),
		PubId:       x.GetPubId(),
		TransportId: x.GetTransportId(),
		Tracks:      x.GetTracks(),
	}
}

func (x *WantParticipantMessage) FixRouter(room string, user string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, user, node)
}

func (x *WantParticipantMessage) CopyPlain() RoomMessage {
	return &WantParticipantMessage{
		Router: x.GetRouter().CopyPlain(),
	}
}

func (x *StateParticipantMessage) FixRouter(room string, user string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, user, node)
}

func (x *StateParticipantMessage) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"router":   x.GetRouter().ToMap(),
		"userId":   x.GetUserId(),
		"userName": x.GetUserName(),
	}
}

func (x *StateParticipantMessage) CopyPlain() RoomMessage {
	return &StateParticipantMessage{
		Router:   x.GetRouter().CopyPlain(),
		UserId:   x.GetUserId(),
		UserName: x.GetUserName(),
	}
}

func (x *UserMessage) FixRouter(room string, user string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, user, node)
}

func (x *UserMessage) CopyPlain() RoomMessage {
	return &UserMessage{
		Router:  x.GetRouter().CopyPlain(),
		Content: x.GetContent(),
		MsgId:   x.GetMsgId(),
	}
}

func (x *UserAckMessage) FixRouter(room string, user string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, user, node)
}

func (x *UserAckMessage) CopyPlain() RoomMessage {
	return &UserAckMessage{
		Router: x.GetRouter().CopyPlain(),
		MsgId:  x.GetMsgId(),
	}
}
