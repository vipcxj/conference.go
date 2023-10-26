package middleware

import (
	"strings"

	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/signal"
	"github.com/zishang520/socket.io/v2/socket"
)

func SocketIOAuthHandler() (func(*socket.Socket, func(*socket.ExtendedError)) ) {
	return func(s *socket.Socket, next func(*socket.ExtendedError)) {
		sCtx := signal.GetSingalContext(s)
		if sCtx != nil && sCtx.Authed() {
			next(nil)
			return
		}

		tokens, ok := s.Handshake().Headers["Authorization"]
		if !ok || tokens == nil || len(tokens) == 0 || (len(tokens) == 1 && tokens[0] == "") {
			next(socket.NewExtendedError("Unauthorized", nil))
			return
		}
		if len(tokens) > 1 {
			next(socket.NewExtendedError("Too many authization header", nil))
			return
		}
		token := tokens[0]
		token = strings.TrimPrefix(token, "Bearer ")
		authInfo := &auth.AuthInfo{}
		err := auth.Decode(token, authInfo)
		if err != nil {
			next(socket.NewExtendedError("Unauthorized", err))
			return
		}
		signal.SetAuthInfo(s, authInfo)
		next(nil)
	}
}