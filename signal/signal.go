package signal

import (
	"reflect"
	"time"

	"github.com/zishang520/socket.io/v2/socket"
)

type AckFunc = func([]any, error)

type SignalServer interface {
	OnSignal(cb func(signal Signal)) error
}

type Signal interface {
	SendMsg(timeout time.Duration, ack bool, evt string, args ...any) (res []any, err error)
	On(evt string, cb func(ack AckFunc, args ...any)) error
	GetAuth() (any, error)
	GetData() (any, error)
	SetData(data any) error
}

type SocketIOSignalServer struct {
	server *socket.Server
}

func NewSocketIOSingalServer() SignalServer {
	io := socket.NewServer(nil, nil)
	return &SocketIOSignalServer{
		server: io,
	}
}

func (server *SocketIOSignalServer) OnSignal(cb func(signal Signal)) error {
	return server.server.On("connection", func(clients ...any) {
		socket := clients[0].(*socket.Socket)
		cb(NewSocketIOSingal(socket))
	})
}

type SocketIOSignal struct {
	socket *socket.Socket
}

func NewSocketIOSingal(socket *socket.Socket) Signal {
	return &SocketIOSignal{
		socket: socket,
	}
}

type resWithError struct {
	res []any
	err error
}

func (signal *SocketIOSignal) SendMsg(timeout time.Duration, ack bool, evt string, args ...any) (res []any, err error) {
	var socket *socket.Socket = nil
	if timeout > 0 {
		socket = signal.socket.Timeout(timeout)
	} else {
		socket = signal.socket
	}
	if ack {
		ch := make(chan *resWithError, 0)
		args = append(args, func(res []any, err error) {
			ch <- &resWithError{
				res: res,
				err: err,
			}
		})
		err := socket.Emit(evt, args...)
		if err != nil {
			return nil, err
		}
		res := <-ch
		return res.res, res.err
	} else {
		return nil, socket.Emit(evt, args...)
	}
}

func (signal *SocketIOSignal) On(evt string, cb func(ack AckFunc, args ...any)) error {
	return signal.socket.On(evt, func(args ...any) {
		if len(args) == 0 {
			cb(nil)
			return
		}
		last := args[len(args)-1]
		var ark AckFunc = nil
		if reflect.TypeOf(last).Kind() == reflect.Func {
			ok := false
			ark, ok = last.(AckFunc)
			if !ok {
				panic("Invalid ack param")
			}
			if len(args) > 1 {
				args = args[0 : len(args)-1]
			} else {
				args = nil
			}
		}
		if args != nil {
			cb(ark, args...)
		} else {
			cb(ark)
		}
	})
}

func (signal *SocketIOSignal) GetAuth() (any, error) {
	return signal.socket.Handshake().Auth, nil
}

func (signal *SocketIOSignal) GetData() (any, error) {
	return signal.GetData()
}

func (signal *SocketIOSignal) SetData(data any) error {
	return signal.SetData(data)
}
