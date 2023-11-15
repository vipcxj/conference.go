package signal

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/utils"
	"github.com/zishang520/socket.io/v2/socket"
)

func FatalErrorAndClose(s *socket.Socket, msg string, cause string) {
	s.Timeout(time.Second*1).EmitWithAck("error", &ErrorMessage{
		Msg:   msg,
		Fatal: true,
		Cause: cause,
	})(func(a []any, err error) {
		s.Disconnect(true)
	})
}

func ErrToMsg(err any) string {
	var msg string
	switch e := err.(type) {
	case error:
		msg = e.Error()
	default:
		msg = fmt.Sprint(e)
	}
	return fmt.Sprintf("%s\n%s\n", msg, string(debug.Stack()))
}

func FinallyResponse(s *socket.Socket, ark func([]any, error), arkArgs []any, cause string) {
	rawErr := recover()
	if rawErr != nil {
		if ark != nil {
			switch err := rawErr.(type) {
			case error:
				ark(nil, err)
			default:
				e := errors.FatalError("%v", rawErr)
				ark(nil, e)
			}
		} else {
			FatalErrorAndClose(s, ErrToMsg(rawErr), cause)
		}
	} else {
		if ark != nil {
			ark(arkArgs, nil)
		}
	}
}

func parseArgs[R any, PR *R](out PR, args ...any) (func([]any, error), error) {
	if len(args) == 0 {
		if out == nil {
			return nil, nil
		} else {
			return nil, errors.FatalError("too little parameter")
		}
	}
	last := args[len(args)-1]
	var ark func([]any, error) = nil
	if reflect.TypeOf(last).Kind() == reflect.Func {
		ark = last.(func([]any, error))
		args = args[0 : len(args)-1]
	}
	if len(args) == 0 {
		if out == nil {
			return nil, nil
		} else {
			return nil, errors.FatalError("too little parameter")
		}
	}
	err := mapstructure.Decode(args[0], out)
	return ark, err
}

func NewUUID(id string, base string) string {
	newBytes := uuid.MustParse(id)
	baseBytes := uuid.MustParse(base)
	utils.XOrBytes(newBytes[:], baseBytes[:], newBytes[:])
	return newBytes.String()
}

func MatchRoom(pattern string, room string) bool {
	if pattern == "" {
		panic(errors.InvalidParam("pattern is required").GenCallStacks())
	}
	if strings.Contains(pattern, "..") || strings.HasPrefix(pattern, ".") || strings.HasSuffix(pattern, ".") {
		panic(errors.InvalidParam("invalid pattern %s", pattern).GenCallStacks())
	}
	if room == "" {
		panic(errors.InvalidParam("room is required").GenCallStacks())
	}
	if strings.Contains(room, "*") || strings.Contains(room, "..") || strings.HasPrefix(room, ".") || strings.HasSuffix(room, ".") {
		panic(errors.InvalidParam("invalid room %s", room).GenCallStacks())
	}
	if pattern == room || pattern == "**" {
		return true
	}
	return matchRoom(strings.Split(pattern, "."), strings.Split(room, "."))
}

func matchRoom(pt_parts []string, rm_parts []string) bool {
	len_pt := len(pt_parts)
	len_rm := len(rm_parts)
	pt_i := 0
	rm_i := 0
	for {
		pt_part := pt_parts[pt_i]
		rm_part := rm_parts[rm_i]
		if pt_part == "**" {
			i := pt_i + 1
			for ; i < len_pt; i++ {
				if pt_parts[i] != "**" {
					break
				}
			}
			if i == len_pt {
				return true
			}
			j := rm_i
			for ; j < len_rm; j++ {
				if matchRoom(pt_parts[i:], rm_parts[j:]) {
					return true
				}
			}
			return false
		} else if pt_part == "*" || pt_part == rm_part {
			pt_i++
			rm_i++
		} else {
			return false
		}
		if pt_i >= len(pt_parts) {
			return rm_i == len_rm
		} else if rm_i >= len_rm {
			return pt_i == len_pt || pt_parts[pt_i] == "**"
		}
	}
}
