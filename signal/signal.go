package signal

import (
	"fmt"
	"reflect"
	"time"

	"github.com/gin-contrib/graceful"
	"github.com/gin-gonic/gin"
	"github.com/vipcxj/conference.go/log"
	"github.com/zishang520/socket.io/v2/socket"
	"go.uber.org/zap"
)

type AckFunc = func([]any, error)

type Signal interface {
	SendMsg(timeout time.Duration, ack bool, evt string, args ...any) (res []any, err error)
	On(evt string, cb func(ack AckFunc, args ...any)) error
	GetAuth() (any, error)
	GetContext() *SignalContext
}

func ConfigureSocketIOSingalServer(global *Global, g *graceful.Graceful) error {
	io := socket.NewServer(nil, nil)
	io.Use(SocketIOAuthHandler(global))
	err := io.On("connection", func(clients ...any) {
		fmt.Printf("on connection\n")
		socket := clients[0].(*socket.Socket)
		ctx, err := InitSignal(socket)
		var suger *zap.SugaredLogger
		if ctx != nil {
			suger = ctx.Sugar()
		} else {
			suger = log.Sugar()
		}
		if err != nil {
			suger.Errorf("socket connect failed, %v", err)
			FatalErrorAndClose(socket, ErrToString(err), "init signal")
		} else {
			suger.Info("socket connected")
		}
	})
	if err != nil {
		return err
	}
	handler := io.ServeHandler(nil)
	g.GET("/socket.io/", gin.WrapH(handler))
	g.POST("/socket.io/", gin.WrapH(handler))
	return nil
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
		ch := make(chan *resWithError)
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

func (signal *SocketIOSignal) GetContext() *SignalContext {
	d := signal.socket.Data()
	if d != nil {
		return d.(*SignalContext)
	} else {
		return nil
	}
}

