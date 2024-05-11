package signal

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/utils"
	"github.com/zishang520/socket.io/v2/socket"
)

func FatalErrorAndClose(s *socket.Socket, err any, cause string) {
	err_msg := ErrToMsg(err, cause)
	err_msg.Fatal = true
	s.Timeout(time.Second*1).EmitWithAck("error", err_msg)(func(a []any, err error) {
		s.Disconnect(true)
	})
}

func ErrToString(err any) string {
	var msg string
	switch e := err.(type) {
	case error:
		msg = e.Error()
	default:
		msg = fmt.Sprint(e)
	}
	return fmt.Sprintf("%s\n%s\n", msg, string(debug.Stack()))
}

func ErrToMsg(err any, cause string) *ErrorMessage {
	switch typedErr := err.(type) {
	case *errors.ConferenceError:
		return &ErrorMessage{
			Msg:        typedErr.Msg,
			Fatal:      typedErr.Code == errors.ERR_FATAL,
			CallFrames: typedErr.CallFrames,
			Cause:      cause,
		}
	case error:
		return &ErrorMessage{
			Msg:   typedErr.Error(),
			Cause: cause,
		}
	default:
		return &ErrorMessage{
			Msg:   ErrToString(err),
			Cause: cause,
		}
	}
}

func ArgsToString(args []any) string {
	arg_list := strings.Join(utils.MapSlice(args, func(arg any) (mapped string, remove bool) {
		return fmt.Sprintf("%v", arg), false
	}), ",")
	return fmt.Sprintf("[%s]", arg_list)
}

func FinallyResponse(s *SignalContext, ark func([]any, error), arkArgs []any, cause string, onlyErr bool) {
	rawErr := recover()
	if rawErr != nil {
		if ark != nil {
			s.Sugar().Debugf("send ack \"%v\" of %v msg", rawErr, cause)
			switch err := rawErr.(type) {
			case error:
				ark(nil, err)
			default:
				e := errors.FatalError("%v", rawErr)
				ark(nil, e)
			}
		} else {
			FatalErrorAndClose(s.Socket, ErrToString(rawErr), cause)
		}
	} else if !onlyErr {
		if ark != nil {
			s.Sugar().Debugf("send ack %s of %v msg", ArgsToString(arkArgs), cause)
			if len(arkArgs) > 0 {
				ark(arkArgs, nil)
			} else {
				ark([]any{"ack"}, nil)
			}
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

type PatternNode[T any] interface {
	GetChildPattern(p string) (PatternNode[T], bool)
	SetChildPattern(p string, n PatternNode[T])
}

type PatternTree[T any] struct {
	children map[string]*PatternTree[T]
	hasData  bool
	data     T
}

func (me *PatternTree[T]) IsEmpty() bool {
	return !me.hasData && len(me.children) == 0
}

func (me *PatternTree[T]) RawData(pattern string) map[string]T {
	results := make(map[string]T)
	if me.hasData {
		results[pattern] = me.data
	}
	for key, pt := range me.children {
		rs := pt.RawData(fmt.Sprintf("%s.%s", pattern, key))
		results = utils.MapPutMap(rs, results)
	}
	return results
}

func (me *PatternTree[T]) removePatternData(pList []string) {
	if len(pList) == 0 {
		var zero T
		me.hasData = false
		me.data = zero
		return
	}
	key := pList[0]
	next, found := me.children[key]
	if found {
		next.removePatternData(pList[1:])
		if next.IsEmpty() {
			delete(me.children, key)
		}
	}
}

func (me *PatternTree[T]) collect(mList []string, doubleStar bool, results []T) []T {
	if len(mList) == 0 {
		if me.hasData {
			results = append(results, me.data)
		}
		pt, found := me.children["**"]
		for found {
			if pt.hasData {
				results = append(results, pt.data)
			}
			pt, found = pt.children["**"]
		}
		return results
	}
	if doubleStar && me.hasData {
		results = append(results, me.data)
	}
	key := mList[0]
	pt, found := me.children["**"]
	if found {
		results = pt.collect(mList, true, results)
	}
	pt, found = me.children["*"]
	if found {
		results = pt.collect(mList[1:], false, results)
	}
	pt, found = me.children[key]
	if found {
		results = pt.collect(mList[1:], false, results)
	}
	if doubleStar {
		for i := 1; i < len(mList); i++ {
			results = me.collect(mList[i:], false, results)
		}
	}
	return results
}

type PatternMap[T any] struct {
	// should always nonull
	inner map[string]*PatternTree[T]
	mu    sync.Mutex
}

func NewPatternMap[T any]() *PatternMap[T] {
	return &PatternMap[T]{}
}

func (me *PatternMap[T]) RawData() map[string]T {
	results := make(map[string]T)
	for pattern, pt := range me.inner {
		results = utils.MapPutMap(pt.RawData(pattern), results)
	}
	return results
}

func (me *PatternMap[T]) removePatternData(pList []string) {
	inner := me.inner
	if inner == nil {
		return
	}
	if len(pList) == 0 {
		panic(errors.ThisIsImpossible().GenCallStacks(0))
	}
	key := pList[0]
	pt, found := inner[key]
	if found {
		pt.removePatternData(pList[1:])
	}
}

func (pm *PatternMap[T]) UpdatePatternData(pattern string, mapper func(old T, found bool) (new T, remove bool)) {
	CheckPattern(pattern)
	pList := strings.Split(pattern, ".")
	pm.mu.Lock()
	defer pm.mu.Unlock()
	inner := pm.inner
	if inner == nil {
		inner = make(map[string]*PatternTree[T])
		pm.inner = inner
	}
	for i, p := range pList {
		found := false
		hasData := false
		var old T
		var pt *PatternTree[T]
		pt, found = inner[p]
		if found {
			hasData = pt.hasData
			old = pt.data
		}
		if i+1 == len(pList) {
			var new T
			var remove bool
			new, remove = mapper(old, hasData)
			if remove {
				pm.removePatternData(pList)
			} else {
				if found {
					pt.hasData = true
					pt.data = new
				} else {
					pt = &PatternTree[T]{
						hasData: true,
						data:    new,
					}
					inner[p] = pt
				}
			}
		} else {
			if !found {
				pt = &PatternTree[T]{}
				inner[p] = pt
			}
			inner = pt.children
			if inner == nil {
				inner = make(map[string]*PatternTree[T])
				pt.children = inner
			}
		}
	}
}

func (pm *PatternMap[T]) Collect(toMatch string) []T {
	CheckRoom(toMatch)
	if pm.inner == nil {
		return nil
	}
	toMatchList := strings.Split(toMatch, ".")
	key := toMatchList[0]
	leftKeys := toMatchList[1:]
	var results []T
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pt, found := pm.inner["**"]
	if found {
		results = pt.collect(toMatchList, true, results)
	}
	pt, found = pm.inner["*"]
	if found {
		results = pt.collect(leftKeys, false, results)
	}
	pt, found = pm.inner[key]
	if found {
		results = pt.collect(leftKeys, false, results)
	}
	return results
}

func CheckPattern(pattern string) {
	if pattern == "" {
		panic(errors.InvalidParam("pattern is required").GenCallStacks(1))
	}
	if strings.Contains(pattern, "..") || strings.HasPrefix(pattern, ".") || strings.HasSuffix(pattern, ".") {
		panic(errors.InvalidParam("invalid pattern %s", pattern).GenCallStacks(1))
	}
}

func CheckRoom(room string) {
	if room == "" {
		panic(errors.InvalidParam("room is required").GenCallStacks(1))
	}
	if strings.Contains(room, "*") || strings.Contains(room, "..") || strings.HasPrefix(room, ".") || strings.HasSuffix(room, ".") {
		panic(errors.InvalidParam("invalid room %s", room).GenCallStacks(1))
	}
}

func MatchRoom(pattern string, room string) bool {
	CheckPattern(pattern)
	CheckRoom(room)
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
			if pt_i == len_pt {
				return true
			} else {
				for i := pt_i; i < len_pt; i++ {
					if pt_parts[i] != "**" {
						return false
					}
				}
				return true
			}
		}
	}
}
