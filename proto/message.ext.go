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

func (x *Router) CopyPlain() *Router {
	return &Router{
		Room: x.GetRoom(),
		NodeFrom: x.GetNodeFrom(),
		NodeTo: x.GetNodeTo(),
		UserFrom: x.GetUserFrom(),
		UserTo: x.GetUserTo(),
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
		Router: x.GetRouter().CopyPlain(),
		ReqTypes: x.GetReqTypes(),
		Pattern: x.GetPattern(),
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
		PubId: x.GetPubId(),
		Addr: x.GetAddr(),
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
		Router: x.GetRouter().CopyPlain(),
		PubId: x.GetPubId(),
		TransportId: x.GetTransportId(),
		Tracks: x.GetTracks(),
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
		Router: x.GetRouter().CopyPlain(),
		Content: x.GetContent(),
		MsgId: x.GetMsgId(),
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
		MsgId: x.GetMsgId(),
	}
}