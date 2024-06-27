package proto

import (
	gproto "google.golang.org/protobuf/proto"
)

type RoomMessage interface {
	gproto.Message
	GetRouter() *Router
	FixRouter(room string, socketId string, node string)
	CopyPlain() RoomMessage
}

func GetRoom(msg RoomMessage) string {
	router := msg.GetRouter()
	if router != nil {
		return router.GetRoom()
	} else {
		return ""
	}
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
		"room":       x.GetRoom(),
		"nodeFrom":   x.GetNodeFrom(),
		"nodeTo":     x.GetNodeTo(),
		"socketFrom": x.GetSocketFrom(),
		"socketTo":   x.GetSocketTo(),
	}
}

func (x *Router) CopyPlain() *Router {
	return &Router{
		Room:       x.GetRoom(),
		NodeFrom:   x.GetNodeFrom(),
		NodeTo:     x.GetNodeTo(),
		SocketFrom: x.GetSocketFrom(),
		SocketTo:   x.GetSocketTo(),
	}
}

func fixRouter(router *Router, room string, socketId string, node string) {
	if router.Room == "" {
		router.Room = room
	}
	router.SocketFrom = socketId
	router.NodeFrom = node
}

func (x *WantMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *WantMessage) CopyPlain() RoomMessage {
	return &WantMessage{
		Router:      x.GetRouter().CopyPlain(),
		ReqTypes:    x.GetReqTypes(),
		Pattern:     x.GetPattern(),
		TransportId: x.GetTransportId(),
	}
}

func (x *StateMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *StateMessage) CopyPlain() RoomMessage {
	return &StateMessage{
		Router: x.GetRouter().CopyPlain(),
		PubId:  x.GetPubId(),
		Addr:   x.GetAddr(),
		Tracks: x.GetTracks(),
	}
}

func (x *SelectMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *SelectMessage) CopyPlain() RoomMessage {
	return &SelectMessage{
		Router:      x.GetRouter().CopyPlain(),
		PubId:       x.GetPubId(),
		TransportId: x.GetTransportId(),
		Tracks:      x.GetTracks(),
	}
}

func (x *WantParticipantMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *WantParticipantMessage) CopyPlain() RoomMessage {
	return &WantParticipantMessage{
		Router: x.GetRouter().CopyPlain(),
	}
}

func (x *StateParticipantMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *StateParticipantMessage) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"router":   x.GetRouter().ToMap(),
		"userId":   x.GetUserId(),
		"userName": x.GetUserName(),
		"socketId": x.GetSocketId(),
		"joinId":   x.GetJoinId(),
	}
}

func (x *StateParticipantMessage) CopyPlain() RoomMessage {
	return &StateParticipantMessage{
		Router:   x.GetRouter().CopyPlain(),
		UserId:   x.GetUserId(),
		UserName: x.GetUserName(),
		SocketId: x.GetSocketId(),
		JoinId:   x.GetJoinId(),
	}
}

func (x *StateLeaveMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *StateLeaveMessage) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"router":   x.GetRouter().ToMap(),
		"userId":   x.GetUserId(),
		"socketId": x.GetSocketId(),
		"joinId":   x.GetJoinId(),
	}
}

func (x *StateLeaveMessage) CopyPlain() RoomMessage {
	return &StateLeaveMessage{
		Router:   x.GetRouter().CopyPlain(),
		UserId:   x.GetUserId(),
		SocketId: x.GetSocketId(),
		JoinId:   x.GetJoinId(),
	}
}

func (x *PingMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *PingMessage) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"router": x.GetRouter().ToMap(),
		"msgId":  x.GetMsgId(),
	}
}

func (x *PingMessage) CopyPlain() RoomMessage {
	return &PingMessage{
		Router: x.GetRouter().CopyPlain(),
		MsgId:  x.GetMsgId(),
	}
}

func (x *PongMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *PongMessage) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"router": x.GetRouter().ToMap(),
		"msgId":  x.GetMsgId(),
	}
}

func (x *PongMessage) CopyPlain() RoomMessage {
	return &PongMessage{
		Router: x.GetRouter().CopyPlain(),
		MsgId:  x.GetMsgId(),
	}
}

func (x *CustomMessage) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"router":  x.GetRouter().ToMap(),
		"content": x.GetContent(),
		"msgId":   x.GetMsgId(),
		"ack":     x.GetAck(),
	}
}

func (x *CustomMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *CustomMessage) CopyPlain() RoomMessage {
	return &CustomMessage{
		Router:  x.GetRouter().CopyPlain(),
		Content: x.GetContent(),
		MsgId:   x.GetMsgId(),
		Ack:     x.GetAck(),
	}
}

func (x *CustomClusterMessage) GetRouter() *Router {
	if x == nil {
		return nil
	}
	if x.Msg == nil {
		return nil
	}
	return x.Msg.Router
}

func (x *CustomClusterMessage) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"evt": x.GetEvt(),
		"msg": x.GetMsg().ToMap(),
	}
}

func (x *CustomClusterMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Msg == nil {
		x.Msg = &CustomMessage{
			Router: &Router{},
		}
	} else if x.Msg.Router == nil {
		x.Msg.Router = &Router{}
	}
	fixRouter(x.Msg.Router, room, socketId, node)
}

func (x *CustomClusterMessage) CopyPlain() RoomMessage {
	return &CustomClusterMessage{
		Evt: x.GetEvt(),
		Msg: x.GetMsg().CopyPlain().(*CustomMessage),
	}
}

func (x *CustomAckMessage) ToMap() map[string]any {
	if x == nil {
		return nil
	}
	return map[string]any{
		"router":  x.GetRouter().ToMap(),
		"msgId":   x.GetMsgId(),
		"content": x.GetContent(),
		"err":     x.GetErr(),
	}
}

func (x *CustomAckMessage) FixRouter(room string, socketId string, node string) {
	if x == nil {
		return
	}
	if x.Router == nil {
		x.Router = &Router{}
	}
	fixRouter(x.Router, room, socketId, node)
}

func (x *CustomAckMessage) CopyPlain() RoomMessage {
	return &CustomAckMessage{
		Router:  x.GetRouter().CopyPlain(),
		MsgId:   x.GetMsgId(),
		Content: x.GetContent(),
		Err:     x.GetErr(),
	}
}
