package signal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-contrib/graceful"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/utils"
	"go.uber.org/zap"
)

type ackArgs struct {
	arg any
	err error
}

type WebSocketSignal struct {
	conn        *websocket.Conn
	ctx         *SignalContext
	next_msg_id atomic.Uint64
	msg_mux     sync.Mutex
	msg_cbs     map[string]func(ack AckFunc, args ...any)
	acks_mux    sync.Mutex
	acks        map[uint64]chan *ackArgs
}

type WsMsgFlag int

const (
	WS_MSG_FLAG_IS_ACK_NORMAL WsMsgFlag = 0
	WS_MSG_FLAG_IS_ACK_ERR    WsMsgFlag = 1
	WS_MSG_FLAG_NEED_ACK      WsMsgFlag = 2
	WS_MSG_FLAG_NO_ACK        WsMsgFlag = 4
)

func isValidWsMsgFlag(flag uint64) bool {
	return flag == 0 || flag == 1 || flag == 2 || flag == 4
}
func (flag WsMsgFlag) is_normal_ack() bool {
	return flag == WS_MSG_FLAG_IS_ACK_NORMAL
}
func (flag WsMsgFlag) is_err_ack() bool {
	return flag == WS_MSG_FLAG_IS_ACK_ERR
}
func (flag WsMsgFlag) is_ack() bool {
	return flag.is_normal_ack() || flag.is_err_ack()
}
func (flag WsMsgFlag) need_ack() bool {
	return flag == WS_MSG_FLAG_NEED_ACK
}

func encodeWsTextData(evt string, msg_id uint64, flag WsMsgFlag, data any) (string, error) {
	data_str, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s;%d;%d;%s", evt, msg_id, int(flag), data_str), nil
}

// evt;msg_id;flag(WsMsgFlag);json_data
func decodeWsTextData(s string) (evt string, msg_id uint64, flag WsMsgFlag, data string, err error) {
	evt_pos := strings.Index(s, ";")
	if evt_pos != -1 {
		evt = s[0:evt_pos]
		s = s[evt_pos+1:]
		msg_id_pos := strings.Index(s, ";")
		if msg_id_pos != -1 {
			msg_id, err = strconv.ParseUint(s[0:msg_id_pos], 10, 64)
			if err != nil {
				err = errors.FatalError("unable to parse msg id when decoding msg, %v", err)
				return
			}
			s = s[msg_id_pos+1:]
			flag_pos := strings.Index(s, ";")
			if flag_pos != -1 {
				var flag_int uint64
				flag_int, err = strconv.ParseUint(s[0:flag_pos], 10, 8)
				if err != nil {
					err = errors.FatalError("unable to parse ack flag when decoding msg, %v", err)
					return
				}
				if !isValidWsMsgFlag(flag_int) {
					err = errors.FatalError("invalid flag in ws msg: %d", flag_int)
					return
				}
				flag = WsMsgFlag(flag_int)
				s = s[flag_pos+1:]
				data_pos := strings.Index(s, ";")
				if data_pos != -1 {
					data = s[data_pos:]
					return
				} else {
					err = errors.FatalError("invalid ws msg format")
				}
			} else {
				err = errors.FatalError("invalid ws msg format")
			}
		} else {
			err = errors.FatalError("invalid ws msg format")
		}
	} else {
		err = errors.FatalError("invalid ws msg format")
	}
	return
}

func newWebSocketSignal(conn *websocket.Conn) *WebSocketSignal {
	signal := &WebSocketSignal{
		conn: conn,
		msg_cbs: make(map[string]func(ack AckFunc, args ...any)),
		acks: make(map[uint64]chan *ackArgs),
	}
	conn.OnMessage(func(c *websocket.Conn, mt websocket.MessageType, b []byte) {
		if mt == websocket.TextMessage {
			s := string(b)
			evt, msg_id, flag, data_str, err := decodeWsTextData(s)
			if err != nil {
				signal.Sugar().Errorf("unable to decode message, %v", err)
				return
			}
			var data any = nil
			if data_str != "" {
				err = json.Unmarshal([]byte(data_str), &data)
				if err != nil {
					if flag.is_ack() {
						err = errors.FatalError("unable to decode ack message, %v", err)
					} else {
						err = errors.FatalError("unable to decode %s message, %v", evt, err)
					}
				}
			}
			signal.msg_mux.Lock()
			defer signal.msg_mux.Unlock()
			if flag.is_ack() {
				signal.acks_mux.Lock()
				ack_ch, ok := signal.acks[msg_id]
				if ok {
					delete(signal.acks, msg_id)
					signal.acks_mux.Unlock()
					if err != nil {
						ack_ch <- &ackArgs{
							arg: nil,
							err: err,
						}
					} else {
						if flag.is_err_ack() {
							ack_ch <- &ackArgs{
								arg: nil,
								err: errors.ClientError(data),
							}
						} else {
							ack_ch <- &ackArgs{
								arg: data,
								err: nil,
							}
						}
					}
				} else {
					signal.acks_mux.Unlock()
					if err != nil {
						signal.Sugar().Errorf("%v", err)
					}
				}
			} else {
				cb, ok := signal.msg_cbs[evt]
				if ok {
					var ack_fun AckFunc = nil
					if flag.need_ack() {
						ack_fun = func(args []any, err error) {
							var data any
							if err != nil {
								data = err.Error()
							} else {
								if len(args) > 0 {
									data = args[0]
								} else {
									data = nil
								}
							}
							var msg string
							if err != nil {
								msg, err = encodeWsTextData("", msg_id, WS_MSG_FLAG_IS_ACK_ERR, data)
							} else {
								msg, err = encodeWsTextData("", msg_id, WS_MSG_FLAG_IS_ACK_NORMAL, data)
							}
							if err != nil {
								signal.Sugar().Errorf("unable to encode ws ack msg with arg %v", data)
								return
							}
							err = conn.WriteMessage(websocket.TextMessage, []byte(msg))
							if err != nil {
								signal.Sugar().Errorf("unable to send ws ack msg with arg %v", data)
								return
							}
						}
					}
					cb(ack_fun, data)
				}
			}
		} else {
			signal.Sugar().Warnf("binary ws msg is not supported, ignore it")
		}
	})
	return signal
}

func (signal *WebSocketSignal) GetContext() *SignalContext {
	return signal.ctx
}

func (signal *WebSocketSignal) Sugar() *zap.SugaredLogger {
	ctx := signal.GetContext()
	if ctx != nil {
		return ctx.Sugar()
	} else {
		return log.Sugar()
	}
}

func (signal *WebSocketSignal) Close() {
	signal.conn.CloseAndClean(nil)
}

func (signal *WebSocketSignal) SendMsg(timeout time.Duration, ack bool, evt string, args ...any) (res []any, err error) {
	if len(args) > 1 {
		return nil, errors.InvalidMessage("too many parameter")
	}
	var arg any = nil
	if len(args) > 0 {
		arg = args[0]
	}
	msg_id := signal.next_msg_id.Add(2)
	var flag WsMsgFlag = WS_MSG_FLAG_NO_ACK
	var release = func () {}
	var ch chan *ackArgs = nil
	if ack {
		ch = make(chan *ackArgs)
		flag = WS_MSG_FLAG_NEED_ACK
		signal.acks_mux.Lock()
		signal.acks[msg_id] = ch
		signal.acks_mux.Unlock()
		release = func ()  {
			signal.acks_mux.Lock()
			defer signal.acks_mux.Unlock()
			delete(signal.acks, msg_id)
		}
	}
	msg, err := encodeWsTextData(evt, msg_id, flag, arg)
	if err != nil {
		release()
		return nil, err
	}
	err = signal.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		release()
		return nil, err
	}
	if ack {
		select {
		case <- time.After(timeout):
			release()
			return nil, errors.MsgTimeout("send %s message timeout", evt)
		case ack_args := <- ch:
			if ack_args.err != nil {
				return nil, ack_args.err
			} else if utils.IsNilInterface(ack_args.arg) {
				return nil, nil
			} else {
				return []any{ack_args.arg}, nil
			}
		}
	} else {
		return nil, nil
	}
}

func (signal *WebSocketSignal) On(evt string, cb func(ack AckFunc, args ...any)) error {
	if evt == "disconnect" {
		signal.conn.OnClose(func(c *websocket.Conn, err error) {
			if err != nil {
				cb(nil, err.Error())
			} else {
				cb(nil)
			}
		})
	} else {
		signal.msg_mux.Lock()
		defer signal.msg_mux.Unlock()
		signal.msg_cbs[evt] = cb
	}
	return nil
}

func ConfigureWebsocketSingalServer(global *Global, g *graceful.Graceful) error {
	handler := func (gctx *gin.Context)  {
		w := gctx.Writer
		r := gctx.Request
		auth_header, ok := r.Header["Authorization"]
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var authInfo auth.AuthInfo
		authed := false
		for _, auth_str := range auth_header {
			auth_arr := strings.SplitN(auth_str, " ", 2)
			var token_str string
			if len(auth_arr) == 0 {
				continue
			} else if len(auth_arr) == 1 {
				token_str = auth_arr[0]
			} else {
				token_str = auth_arr[1]
			}
			var _authInfo auth.AuthInfo
			err := auth.Decode(token_str, &_authInfo)
			if err == nil {
				authed = true
				authInfo = _authInfo
				break
			}
		}
		if !authed {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var signal_id string
		signal_id_header, ok := r.Header["Signal-Id"]
		if ok && len(signal_id_header) > 0 {
			signal_id = signal_id_header[0]
		} else {
			signal_id = uuid.NewString()
		}
		upgrader := websocket.NewUpgrader()
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			conn.CloseAndClean(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.WriteString(err.Error())
			return
		}
		signal := newWebSocketSignal(conn)
		ctx := newSignalContext(global, signal, &authInfo, signal_id)
		signal.ctx = ctx
		_, err = InitSignal(signal)
		if err != nil {
			conn.CloseAndClean(err)
			w.WriteHeader(http.StatusInternalServerError)
			w.WriteString(err.Error())
			return
		}
	}
	g.GET("/ws", handler)
	return nil
}
