package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	"github.com/vipcxj/conference.go/utils"
	"go.uber.org/zap"
)

type AckFunc = func([]any, error)
type MsgCb = func(evt string, ack AckFunc, args ...any)

type ackArgs struct {
	arg any
	err error
}

type WsSignalMode int

const (
	WS_SIGNAL_MODE_CLIENT WsSignalMode = 0
	WS_SIGNAL_MODE_SERVER WsSignalMode = 1
)

func (mode WsSignalMode) IsClient() bool {
	return mode == WS_SIGNAL_MODE_CLIENT
}

func (mode WsSignalMode) IsServer() bool {
	return mode == WS_SIGNAL_MODE_SERVER
}

type WebSocketSignal struct {
	mode               WsSignalMode
	conn               *websocket.Conn
	conn_ch            chan any
	next_msg_id        atomic.Uint64
	closed             bool
	close_err          error
	close_mux          sync.Mutex
	close_cb           func(error)
	close_ch           chan any
	msg_mux            sync.Mutex
	msg_cb             MsgCb
	custom_msg_mux     sync.Mutex
	custom_msg_cb      func(evt string, msg *model.CustomMessage)
	custom_ack_msg_mux sync.Mutex
	custom_ack_msg_cb  func(msg *model.CustomAckMessage)
	acks_mux           sync.Mutex
	acks               map[uint64]chan *ackArgs
	logger             *zap.Logger
	sugar              *zap.SugaredLogger
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
				data = s[flag_pos+1:]
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
	return
}

func NewWebSocketSignal(mode WsSignalMode, upgrader *websocket.Upgrader) *WebSocketSignal {
	sig := &WebSocketSignal{
		mode:     mode,
		conn_ch:  make(chan any),
		close_ch: make(chan any),
		acks:     make(map[uint64]chan *ackArgs),
	}
	if mode.IsClient() {
		sig.next_msg_id.Add(1)
	}
	upgrader.OnOpen(func(c *websocket.Conn) {
		sig.conn = c
		close(sig.conn_ch)
	})
	msg_handler := func(c *websocket.Conn, mt websocket.MessageType, b []byte) {
		if mt == websocket.TextMessage {
			s := string(b)
			evt, msg_id, flag, data_str, err := decodeWsTextData(s)
			if err != nil {
				sig.Sugar().Errorf("unable to decode message, %v", err)
				return
			}
			var data any
			if strings.HasPrefix(evt, "custom:") {
				data = model.CustomMessage{}
			} else if evt == "custom-ack" {
				data = model.CustomAckMessage{}
			}
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
			if flag.is_ack() {
				sig.acks_mux.Lock()
				ack_ch, ok := sig.acks[msg_id]
				if ok {
					delete(sig.acks, msg_id)
					sig.acks_mux.Unlock()
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
					sig.acks_mux.Unlock()
					if err != nil {
						sig.Sugar().Errorf("%v", err)
					}
				}
			} else {
				if strings.HasPrefix(evt, "custom:") {
					msg := data.(*model.CustomMessage)
					sig.custom_msg_mux.Lock()
					cb := sig.custom_msg_cb
					sig.custom_msg_mux.Unlock()
					cb(evt[7:], msg)
				} else if evt == "custom-ack" {
					msg := data.(*model.CustomAckMessage)
					sig.custom_ack_msg_mux.Lock()
					cb := sig.custom_ack_msg_cb
					sig.custom_ack_msg_mux.Unlock()
					cb(msg)
				} else {
					sig.msg_mux.Lock()
					if sig.msg_cb != nil {
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
									sig.Sugar().Errorf("unable to encode ws ack msg with arg %v", data)
									return
								}
								err = c.WriteMessage(websocket.TextMessage, []byte(msg))
								if err != nil {
									sig.Sugar().Errorf("unable to send ws ack msg with arg %v", data)
									return
								}
							}
						}
						sig.msg_cb(evt, ack_fun, data)
					}
					sig.msg_mux.Unlock()
				}
			}
		} else {
			sig.Sugar().Warnf("binary ws msg is not supported, ignore it")
		}
	}
	upgrader.OnMessage(msg_handler)
	upgrader.OnClose(func(c *websocket.Conn, err error) {
		close(sig.close_ch)
		sig.close_mux.Lock()
		defer sig.close_mux.Unlock()
		if !sig.closed {
			sig.closed = true
			sig.close_err = err
			if sig.close_cb != nil {
				sig.close_cb(err)
			}
		}
	})
	return sig
}

func (signal *WebSocketSignal) Logger() *zap.Logger {
	if signal != nil && signal.logger != nil {
		return signal.logger
	} else {
		return log.Logger()
	}
}

func (signal *WebSocketSignal) SetLogger(logger *zap.Logger) {
	signal.logger = logger
}

func (signal *WebSocketSignal) Sugar() *zap.SugaredLogger {
	if signal != nil && signal.sugar != nil {
		return signal.sugar
	} else {
		return log.Sugar()
	}
}

func (signal *WebSocketSignal) SetSugar(sugar *zap.SugaredLogger) {
	signal.sugar = sugar
}

func (signal *WebSocketSignal) waitConn(ctx context.Context) (conn *websocket.Conn, err error) {
	if signal.conn != nil {
		return signal.conn, nil
	}
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case <-signal.conn_ch:
		return signal.conn, nil
	case <-signal.close_ch:
		return nil, net.ErrClosed
	}
}

func (signal *WebSocketSignal) Close() {
	conn, _ := signal.waitConn(context.Background())
	if conn != nil {
		conn.CloseAndClean(nil)
	}
}

func (signal *WebSocketSignal) SendMsg(ctx context.Context, ack bool, evt string, args ...any) (res []any, err error) {
	conn, err := signal.waitConn(ctx)
	if err != nil {
		return nil, err
	}
	if len(args) > 1 {
		return nil, errors.InvalidMessage("too many parameter")
	}
	var arg any = nil
	if len(args) > 0 {
		arg = args[0]
	}
	msg_id := signal.next_msg_id.Add(2)
	var flag WsMsgFlag = WS_MSG_FLAG_NO_ACK
	var release = func() {}
	var ch chan *ackArgs = nil
	if ack {
		ch = make(chan *ackArgs)
		flag = WS_MSG_FLAG_NEED_ACK
		signal.acks_mux.Lock()
		signal.acks[msg_id] = ch
		signal.acks_mux.Unlock()
		release = func() {
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
	err = conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		release()
		return nil, err
	}
	if ack {
		select {
		case <-signal.close_ch:
			if signal.close_err != nil {
				return nil, signal.close_err
			} else {
				return nil, net.ErrClosed
			}
		case <-ctx.Done():
			release()
			return nil, context.Cause(ctx)
		case ack_args := <-ch:
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

func (signal *WebSocketSignal) On(cb MsgCb) error {
	signal.msg_mux.Lock()
	defer signal.msg_mux.Unlock()
	signal.msg_cb = cb
	return nil
}

func (signal *WebSocketSignal) OnCustom(cb func(evt string, msg *model.CustomMessage)) error {
	signal.custom_msg_mux.Lock()
	defer signal.custom_msg_mux.Unlock()
	signal.custom_msg_cb = cb
	return nil
}

func (signal *WebSocketSignal) OnCustomAck(cb func(msg *model.CustomAckMessage)) error {
	signal.custom_ack_msg_mux.Lock()
	defer signal.custom_ack_msg_mux.Unlock()
	signal.custom_ack_msg_cb = cb
	return nil
}

func (signal *WebSocketSignal) OnClose(cb func(err error)) {
	signal.close_mux.Lock()
	defer signal.close_mux.Unlock()
	if signal.closed {
		cb(signal.close_err)
	} else {
		signal.close_cb = cb
	}
}
