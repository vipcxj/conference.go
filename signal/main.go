package signal

import (
	"time"

	"github.com/zishang520/socket.io/v2/socket"
)

func FatalErrorAndClose(s *socket.Socket, msg string) {
	s.Timeout(time.Second*1).EmitWithAck("fatal error", msg)(func(a []any, err error) {
		s.Disconnect(true)
	})
}
