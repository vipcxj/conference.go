package singal

import (
	"github.com/vipcxj/conference.go/auth"
	"github.com/zishang520/socket.io/v2/socket"
)

type SingalContext struct {
	authInfo *auth.AuthInfo
}

func (ctx *SingalContext)Authed() bool {
	return ctx.authInfo != nil
}

func GetSingalContext(s *socket.Socket) *SingalContext {
	return s.Data().(*SingalContext)
}

func SetAuthInfo(s *socket.Socket, authInfo *auth.AuthInfo) {
	raw := s.Data()
	if raw == nil {
		ctx := &SingalContext{
			authInfo: authInfo,
		}
		s.SetData(ctx)
	} else {
		raw.(*SingalContext).authInfo = authInfo
	}
}