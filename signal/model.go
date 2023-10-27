package signal

import (
	"unsafe"

	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
	"github.com/zishang520/socket.io/v2/socket"
)

type SingalContext struct {
	authInfo *auth.AuthInfo
}

func (ctx *SingalContext) Authed() bool {
	return ctx.authInfo != nil
}

func (ctx *SingalContext) Rooms() []socket.Room {
	if ctx.authInfo != nil {
		if rooms := ctx.authInfo.Rooms; rooms != nil && len(rooms) > 0 {
			return unsafe.Slice((*socket.Room)(unsafe.Pointer(&rooms[0])), len(rooms))
		} else {
			return []socket.Room{}
		}
	} else {
		return []socket.Room{}
	}
}

func GetSingalContext(s *socket.Socket) *SingalContext {
	d := s.Data()
	if d != nil {
		return s.Data().(*SingalContext)
	} else {
		return nil
	}
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

func JoinRoom(s *socket.Socket) error {
	ctx := GetSingalContext(s)
	if ctx == nil {
		return errors.Unauthorized("")
	}
	rooms := ctx.Rooms()
	if len(rooms) == 0 {
		return errors.InvalidParam("room is not provided")
	}
	s.Join(rooms...)
	return nil
}
