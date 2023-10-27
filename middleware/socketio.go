package middleware

import (
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/signal"
	"github.com/zishang520/socket.io/v2/socket"
)

func SocketIOAuthHandler() func(*socket.Socket, func(*socket.ExtendedError)) {
	return func(s *socket.Socket, next func(*socket.ExtendedError)) {
		sCtx := signal.GetSingalContext(s)
		if sCtx != nil && sCtx.Authed() {
			next(nil)
			return
		}

		token, ok := s.Handshake().Auth.(map[string]string)["token"]
		if !ok || token == "" {
			next(socket.NewExtendedError("Unauthorized", nil))
			return
		}
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
