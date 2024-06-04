package signal

import (
	"fmt"
	"reflect"
	"time"

	"github.com/gin-contrib/graceful"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/zishang520/socket.io/v2/socket"
	"go.uber.org/zap"
)

func GetSingalContextFromSocketIO(s *socket.Socket) *SignalContext {
	d := s.Data()
	if d != nil {
		ctx, ok := d.(*SignalContext)
		if ok {
			return ctx
		} else {
			return nil
		}
	} else {
		return nil
	}
}

type authAndId struct {
	auth *auth.AuthInfo
	id string
}

func SocketIOAuthHandler(global *Global) func(*socket.Socket, func(*socket.ExtendedError)) {
	return func(s *socket.Socket, next func(*socket.ExtendedError)) {
		sCtx := GetSingalContextFromSocketIO(s)
		if sCtx != nil && sCtx.Authed() {
			next(nil)
			return
		}
		authData, ok := s.Handshake().Auth.(map[string]interface{})
		if !ok {
			next(socket.NewExtendedError("Unauthorized", nil))
			return
		}
		tokenAny, ok := authData["token"]
		if !ok {
			next(socket.NewExtendedError("Unauthorized", nil))
			return
		}
		token, ok := tokenAny.(string)
		if !ok {
			next(socket.NewExtendedError(fmt.Sprintf("Invalid token type %v", reflect.TypeOf(tokenAny)), nil))
		}
		var signalId string
		signalIdAny, ok := authData["id"]
		if ok {
			signalId, ok = signalIdAny.(string)
			if !ok {
				next(socket.NewExtendedError(fmt.Sprintf("Invalid signal id type %v", reflect.TypeOf(signalIdAny)), nil))
			}
		} else {
			signalId = uuid.NewString()
			log.Sugar().Debugf("no signal id provided in auth data, generate one new %v", signalId)
		}
		authInfo := &auth.AuthInfo{}
		err := auth.Decode(token, authInfo)
		if err != nil || authInfo.Usage != auth.AUTH_USAGE || authInfo.Key == "" || authInfo.UID == "" || (authInfo.Timestamp+int64(authInfo.Deadline) < time.Now().Unix()) {
			next(socket.NewExtendedError("Unauthorized", err))
			return
		}
		s.SetData(&authAndId {
			auth: authInfo,
			id: signalId,
		})
		next(nil)
	}
}

func ConfigureSocketIOSingalServer(global *Global, g *graceful.Graceful) error {
	io := socket.NewServer(nil, nil)
	io.Use(SocketIOAuthHandler(global))
	err := io.On("connection", func(clients ...any) {
		fmt.Printf("on connection\n")
		socket := clients[0].(*socket.Socket)
		authData := socket.Data().(*authAndId)
		signal := NewSocketIOSingal(socket)
		ctx := newSignalContext(global, signal, authData.auth, authData.id)
		socket.SetData(ctx)
		ctx, err := InitSignal(signal)
		sugar := signal.Sugar()
		if err != nil {
			sugar.Errorf("socket connect failed, %v", err)
			FatalErrorAndClose(ctx, ErrToString(err), "init signal")
		} else {
			sugar.Info("socket connected")
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
		if timeout > 0 {
			select {
			case <-time.After(timeout):
				return nil, errors.MsgTimeout("send %s message timeout.", evt)
			case res := <-ch:
				return res.res, res.err
			}
		} else {
			res := <-ch
			return res.res, res.err
		}
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
		lastType := reflect.TypeOf(last)
		if lastType != nil && lastType.Kind() == reflect.Func {
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

func (signal *SocketIOSignal) GetContext() *SignalContext {
	return GetSingalContextFromSocketIO(signal.socket)
}

func (signal *SocketIOSignal) Sugar() *zap.SugaredLogger {
	ctx := signal.GetContext()
	if ctx != nil {
		return ctx.Sugar()
	} else {
		return log.Sugar()
	}
}

func (signal *SocketIOSignal) Close() {
	signal.socket.Disconnect(true)
}