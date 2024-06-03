package signal

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/log"
	"github.com/zishang520/socket.io/v2/socket"
)

func SocketIOAuthHandler(global *Global) func(*socket.Socket, func(*socket.ExtendedError)) {
	return func(s *socket.Socket, next func(*socket.ExtendedError)) {
		sCtx := GetSingalContext(s)
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
		if err != nil || authInfo.Usage != auth.AUTH_USAGE || authInfo.Key == "" || authInfo.UID == "" || (authInfo.Timestamp + int64(authInfo.Deadline) < time.Now().Unix()) {
			next(socket.NewExtendedError("Unauthorized", err))
			return
		}
		SetAuthInfoAndId(s, authInfo, signalId, global)
		next(nil)
	}
}
