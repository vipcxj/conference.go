package signal

import (
	"fmt"

	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/utils"
)

type PatternOp int

const (
	PATTERN_OP_ALL PatternOp = iota
	PATTERN_OP_SOME
	PATTERN_OP_NONE
	PATTERN_OP_PUBLISH_ID
	PATTERN_OP_STREAM_ID
	PATTERN_OP_TRACK_ID
	PATTERN_OP_TRACK_LABEL_ALL_MATCH
	PATTERN_OP_TRACK_LABEL_SOME_MATCH
	PATTERN_OP_TRACK_LABEL_NONE_MATCH
	PATTERN_OP_TRACK_LABEL_ALL_HAS
	PATTERN_OP_TRACK_LABEL_SOME_HAS
	PATTERN_OP_TRACK_LABEL_NONE_HAS
)

const (
	pt_op_all_str             = "ALL"
	pt_op_some_str            = "SOME"
	pt_op_none_str            = "NONE"
	pt_op_pid_str             = "PUBLISH_ID"
	pt_op_sid_str             = "STREAM_ID"
	pt_op_tid_str             = "TRACK_ID"
	pt_op_t_lb_all_match_str  = "TRACK_LABEL_ALL_MATCH"
	pt_op_t_lb_some_match_str = "TRACK_LABEL_SOME_MATCH"
	pt_op_t_lb_none_match_str = "TRACK_LABEL_NONE_MATCH"
	pt_op_t_lb_all_has_str    = "TRACK_LABEL_ALL_HAS"
	pt_op_t_lb_some_has_str   = "TRACK_LABEL_SOME_HAS"
	pt_op_t_lb_none_has_str   = "TRACK_LABEL_NONE_HAS"
	pt_op_unknown_str         = "UNKNOWN"
)

func (op PatternOp) String() string {
	switch op {
	case PATTERN_OP_ALL:
		return pt_op_all_str
	case PATTERN_OP_SOME:
		return pt_op_some_str
	case PATTERN_OP_NONE:
		return pt_op_none_str
	case PATTERN_OP_PUBLISH_ID:
		return pt_op_pid_str
	case PATTERN_OP_STREAM_ID:
		return pt_op_sid_str
	case PATTERN_OP_TRACK_ID:
		return pt_op_tid_str
	case PATTERN_OP_TRACK_LABEL_ALL_MATCH:
		return pt_op_t_lb_all_match_str
	case PATTERN_OP_TRACK_LABEL_SOME_MATCH:
		return pt_op_t_lb_some_match_str
	case PATTERN_OP_TRACK_LABEL_NONE_MATCH:
		return pt_op_t_lb_none_match_str
	case PATTERN_OP_TRACK_LABEL_ALL_HAS:
		return pt_op_t_lb_all_has_str
	case PATTERN_OP_TRACK_LABEL_SOME_HAS:
		return pt_op_t_lb_some_has_str
	case PATTERN_OP_TRACK_LABEL_NONE_HAS:
		return pt_op_t_lb_none_has_str
	default:
		return pt_op_unknown_str
	}
}

type PublicationPattern struct {
	Op       PatternOp
	Args     []string
	Children []PublicationPattern
}

func (me *PublicationPattern) String() string {
	return fmt.Sprintf("{ Op: %v, Args: %v, Children: %v }", me.Op, me.Args, me.Children)
}

func (me *PublicationPattern) checkArgsNum(n int, atLeast bool) error {
	if !atLeast && len(me.Args) != n {
		return errors.InvalidPubPattern(fmt.Sprintf("The pub pattern op '%v' accept %d args, but gotten %v", me.Op, n, me.Args))
	} else if atLeast && len(me.Args) < n {
		return errors.InvalidPubPattern(fmt.Sprintf("The pub pattern op '%v' accept at least %d args, but gotten %v", me.Op, n, me.Args))
	} else {
		return nil
	}
}

func (me *PublicationPattern) Validate() error {
	var err error
	switch me.Op {
	case PATTERN_OP_ALL, PATTERN_OP_SOME, PATTERN_OP_NONE:
		if err = me.checkArgsNum(0, false); err != nil {
			return err
		}
		for _, c := range me.Children {
			err := c.Validate()
			if err != nil {
				return err
			}
		}
	default:
		if len(me.Children) > 0 {
			return errors.InvalidPubPattern(fmt.Sprintf("The pub pattern op '%v' accept zero child, but gotten %v", me.Children))
		}
		switch me.Op {
		case PATTERN_OP_PUBLISH_ID, PATTERN_OP_STREAM_ID, PATTERN_OP_TRACK_ID, PATTERN_OP_TRACK_LABEL_ALL_HAS, PATTERN_OP_TRACK_LABEL_SOME_HAS, PATTERN_OP_TRACK_LABEL_NONE_HAS:
			if err = me.checkArgsNum(1, true); err != nil {
				return err
			}
		case PATTERN_OP_TRACK_LABEL_ALL_MATCH, PATTERN_OP_TRACK_LABEL_SOME_MATCH, PATTERN_OP_TRACK_LABEL_NONE_MATCH:
			if l := len(me.Args); l < 2 || l%2 != 0 {
				return errors.InvalidPubPattern(fmt.Sprintf("The pub pattern op '%v' accept at least 2 even number of args, but gotten %v", me.Children))
			}
		default:
			return errors.InvalidPubPattern(fmt.Sprintf("Unkonwn op: %d", me.Op))
		}
	}
	return nil
}

func (me *PublicationPattern) Match(track *Track) bool {
	switch me.Op {
	case PATTERN_OP_ALL:
		for _, c := range me.Children {
			if !c.Match(track) {
				return false
			}
		}
		return true
	case PATTERN_OP_SOME:
		for _, c := range me.Children {
			if c.Match(track) {
				return true
			}
		}
		return false
	case PATTERN_OP_NONE:
		for _, c := range me.Children {
			if c.Match(track) {
				return false
			}
		}
		return true
	case PATTERN_OP_PUBLISH_ID:
		return utils.InSlice(me.Args, track.PubId, nil)
	case PATTERN_OP_STREAM_ID:
		return utils.InSlice(me.Args, track.StreamId, nil)
	case PATTERN_OP_TRACK_ID:
		return utils.InSlice(me.Args, track.GlobalId, nil)
	case PATTERN_OP_TRACK_LABEL_ALL_MATCH:
		for i := 0; i < len(me.Args); {
			key := me.Args[i]
			value := me.Args[i+1]
			if !track.MatchLabel(key, value) {
				return false
			}
			i += 2
		}
		return true
	case PATTERN_OP_TRACK_LABEL_SOME_MATCH:
		for i := 0; i < len(me.Args); {
			key := me.Args[i]
			value := me.Args[i+1]
			if track.MatchLabel(key, value) {
				return true
			}
			i += 2
		}
		return false
	case PATTERN_OP_TRACK_LABEL_NONE_MATCH:
		for i := 0; i < len(me.Args); {
			key := me.Args[i]
			value := me.Args[i+1]
			if track.MatchLabel(key, value) {
				return false
			}
			i += 2
		}
		return true
	case PATTERN_OP_TRACK_LABEL_ALL_HAS:
		for _, name := range me.Args {
			if !track.HasLabel(name) {
				return false
			}
		}
		return true
	case PATTERN_OP_TRACK_LABEL_SOME_HAS:
		for _, name := range me.Args {
			if track.HasLabel(name) {
				return true
			}
		}
		return false
	case PATTERN_OP_TRACK_LABEL_NONE_HAS:
		for _, name := range me.Args {
			if track.HasLabel(name) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
