package signal

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/graceful"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	"github.com/zishang520/socket.io/v2/socket"
	"go.uber.org/zap"
)

type IOSocketAckFunc = func ([]any, error)

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
	id   string
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
		s.SetData(&authAndId{
			auth: authInfo,
			id:   signalId,
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
	socket             *socket.Socket
	msg_cbs            *MsgCbs
	custom_msg_cb      CustomMsgCb
	custom_msg_mux     sync.Mutex
	custom_ack_msg_cb  CustomAckMsgCb
	custom_ack_msg_mux sync.Mutex
	on_panic           func(custom_msg bool, err any, evt string, args ...any) (recover bool)
}

func NewSocketIOSingal(socket *socket.Socket) Signal {
	ss := &SocketIOSignal{
		socket: socket,
		msg_cbs: NewMsgCbs(),
	}
	ss.socket.OnAny(func(args ...any) {
		if len(args) > 0 {
			evt, ok := args[0].(string)
			if ok {
				if strings.HasPrefix(evt, "custom:") {
					evt = evt[7:]
					if len(args) > 1 {
						msg := model.CustomMessage{}
						err := mapstructure.Decode(args[1], &msg)
						if err == nil {
							func() {
								defer func() {
									if ss.on_panic != nil {
										if err := recover(); err != nil {
											if !ss.on_panic(true, err, evt, &msg) {
												panic(err)
											}
										}
									}
								}()
								ss.custom_msg_mux.Lock()
								defer ss.custom_msg_mux.Unlock()
								if ss.custom_msg_cb != nil {
									ss.custom_msg_cb(evt, &msg)
								}
							}()
						}
					}
				} else if evt == "custom-ack" {
					if len(args) > 1 {
						msg := model.CustomAckMessage{}
						err := mapstructure.Decode(args[1], &msg)
						if err == nil {
							func() {
								defer func() {
									if ss.on_panic != nil {
										if err := recover(); err != nil {
											if !ss.on_panic(true, err, evt, &msg) {
												panic(err)
											}
										}
									}
								}()
								ss.custom_ack_msg_mux.Lock()
								defer ss.custom_ack_msg_mux.Unlock()
								if ss.custom_ack_msg_cb != nil {
									ss.custom_ack_msg_cb(&msg)
								}
							}()
						}
					}
				} else {
					var real_args []any
					var ack AckFunc
					if len(args) == 1 {
						real_args = nil
					} else {
						last := args[len(args)-1]
						lastType := reflect.TypeOf(last)
						if lastType != nil && lastType.Kind() == reflect.Func {
							io_ack, ok := last.(IOSocketAckFunc)
							if !ok {
								panic("Invalid ack param")
							}
							ack = func(args []any, err *errors.ConferenceError) {
								io_ack(args, err)
							}
							if len(args) > 2 {
								real_args = args[1 : len(args)-1]
							} else {
								real_args = nil
							}
						} else {
							real_args = args[1:]
						}
					}
					if real_args == nil {
						ss.msg_cbs.Run(evt, ack)
					} else {
						ss.msg_cbs.Run(evt, ack, real_args...)
					}
				}
			}
		}
	})
	return ss
}

type resWithError struct {
	res []any
	err error
}

func (signal *SocketIOSignal) SendMsg(ctx context.Context, ack bool, evt string, args ...any) (res []any, err error) {
	var socket *socket.Socket = nil
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		timeout := time.Until(deadline)
		if timeout > 0 {
			socket = signal.socket.Timeout(timeout)
		} else {
			socket = signal.socket
		}
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
		select {
		case <- ctx.Done():
			return nil, errors.MsgTimeout("send %s message timeout.", evt)
		case res := <-ch:
			return res.res, res.err
		}
	} else {
		return nil, socket.Emit(evt, args...)
	}
}

func (signal *SocketIOSignal) On(evt string, cb MsgCb) error {
	signal.msg_cbs.AddCallback(evt, cb)
	return nil
}

func (signal *SocketIOSignal) OnCustom(cb CustomMsgCb) error {
	signal.custom_msg_mux.Lock()
	defer signal.custom_msg_mux.Unlock()
	signal.custom_msg_cb = cb
	return nil
}

func (signal *SocketIOSignal) OnCustomAck(cb CustomAckMsgCb) error {
	signal.custom_ack_msg_mux.Lock()
	defer signal.custom_ack_msg_mux.Unlock()
	signal.custom_ack_msg_cb = cb
	return nil
}

func (signal *SocketIOSignal) OnClose(cb func(args ...any)) {
	signal.socket.On("disconnect", func(args ...any) {
		cb(args...)
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
