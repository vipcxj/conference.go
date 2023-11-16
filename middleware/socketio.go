package middleware

import (
	"reflect"

	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
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

		tokenAny, ok := s.Handshake().Auth.(map[string]interface{})["token"]
		if !ok {
			next(socket.NewExtendedError("Unauthorized", nil))
			return
		}
		token, ok := tokenAny.(string)
		if !ok {
			panic(errors.FatalError("Invalid token type %v", reflect.TypeOf(token)))
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
