package middleware

import (
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
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
		authData := s.Handshake().Auth.(map[string]interface{})
		tokenAny, ok := authData["token"]
		if !ok {
			next(socket.NewExtendedError("Unauthorized", nil))
			return
		}
		token, ok := tokenAny.(string)
		if !ok {
			panic(errors.FatalError("Invalid token type %v", reflect.TypeOf(tokenAny)))
		}
		var signalId string
		signalIdAny, ok := authData["id"]
		if ok {
			signalId, ok = signalIdAny.(string)
			if !ok {
				panic(errors.FatalError("Invalid signal id type %v", reflect.TypeOf(signalIdAny)))
			}
		} else {
			signalId = uuid.NewString()
			log.Sugar().Debugf("no signal id provided in auth data, generate one new %v", signalId)
		}
		authInfo := &auth.AuthInfo{}
		err := auth.Decode(token, authInfo)
		if err != nil || authInfo.Usage != auth.AUTH_USAGE || (authInfo.Timestamp + int64(authInfo.Deadline) < time.Now().Unix()) {
			next(socket.NewExtendedError("Unauthorized", err))
			return
		}
		signal.SetAuthInfoAndId(s, authInfo, signalId)
		next(nil)
	}
}
