package signal

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-contrib/graceful"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	nbws "github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	"github.com/vipcxj/conference.go/websocket"
	"go.uber.org/zap"
)

type WebSocketSignal struct {
	signal  *websocket.WebSocketSignal
	ctx     *SignalContext
	msg_cbs *MsgCbs
}

func newWebSocketSignal(upgrader *nbws.Upgrader) *WebSocketSignal {
	signal := websocket.NewWebSocketSignal(websocket.WS_SIGNAL_MODE_SERVER, upgrader)
	ws := &WebSocketSignal{
		signal:  signal,
		msg_cbs: NewMsgCbs(),
	}
	signal.On(func(evt string, ack websocket.AckFunc, args ...any) {
		ws.msg_cbs.Run(evt, ack, args...)
	})
	return ws
}

func (signal *WebSocketSignal) GetContext() *SignalContext {
	return signal.ctx
}

func (signal *WebSocketSignal) setContext(ctx *SignalContext) {
	signal.ctx = ctx
	signal.signal.SetLogger(ctx.Logger())
	signal.signal.SetSugar(ctx.Sugar())
}

func (signal *WebSocketSignal) Sugar() *zap.SugaredLogger {
	if signal != nil && signal.signal != nil {
		return signal.signal.Sugar()
	} else {
		return log.Sugar()
	}
}

func (signal *WebSocketSignal) Close() {
	signal.signal.Close()
}

func (signal *WebSocketSignal) SendMsg(ctx context.Context, ack bool, evt string, args ...any) (res []any, err error) {
	return signal.signal.SendMsg(ctx, ack, evt, args...)
}

func (signal *WebSocketSignal) On(evt string, cb MsgCb) error {
	signal.msg_cbs.AddCallback(evt, cb)
	return nil
}

func (signal *WebSocketSignal) OnCustom(cb CustomMsgCb) error {
	return signal.signal.OnCustom(cb)
}

func (signal *WebSocketSignal) OnCustomAck(cb func(msg *model.CustomAckMessage)) error {
	return signal.signal.OnCustomAck(cb)
}

func (signal *WebSocketSignal) OnClose(cb func(args ...any)) {
	signal.signal.OnClose(func(err error) {
		cb(err)
	})
}

func ConfigureWebsocketSingalServer(global *Global, g *graceful.Graceful) error {
	handler := func(gctx *gin.Context) {
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
		upgrader := nbws.NewUpgrader()
		signal := newWebSocketSignal(upgrader)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			conn.CloseAndClean(err)
			// w.WriteHeader(http.StatusInternalServerError)
			// w.WriteString(err.Error())
			return
		}
		ctx := newSignalContext(global, signal, &authInfo, signal_id)
		signal.setContext(ctx)
		_, err = InitSignal(signal)
		if err != nil {
			conn.CloseAndClean(err)
			// w.WriteHeader(http.StatusInternalServerError)
			// w.WriteString(err.Error())
			return
		}
	}
	g.GET("/ws", handler)
	return nil
}
