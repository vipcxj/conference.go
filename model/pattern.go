package model

import (
	"fmt"

	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/proto"
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
	PATTERN_OP_TRACK_RID
	PATTERN_OP_TRACK_LABEL_ALL_MATCH
	PATTERN_OP_TRACK_LABEL_SOME_MATCH
	PATTERN_OP_TRACK_LABEL_NONE_MATCH
	PATTERN_OP_TRACK_LABEL_ALL_HAS
	PATTERN_OP_TRACK_LABEL_SOME_HAS
	PATTERN_OP_TRACK_LABEL_NONE_HAS
	PATTERN_OP_TRACK_TYPE
)

const (
	pt_op_all_str             = "ALL"
	pt_op_some_str            = "SOME"
	pt_op_none_str            = "NONE"
	pt_op_pid_str             = "PUBLISH_ID"
	pt_op_sid_str             = "STREAM_ID"
	pt_op_tid_str             = "TRACK_ID"
	pt_op_rid_str             = "TRACK_RID"
	pt_op_t_lb_all_match_str  = "TRACK_LABEL_ALL_MATCH"
	pt_op_t_lb_some_match_str = "TRACK_LABEL_SOME_MATCH"
	pt_op_t_lb_none_match_str = "TRACK_LABEL_NONE_MATCH"
	pt_op_t_lb_all_has_str    = "TRACK_LABEL_ALL_HAS"
	pt_op_t_lb_some_has_str   = "TRACK_LABEL_SOME_HAS"
	pt_op_t_lb_none_has_str   = "TRACK_LABEL_NONE_HAS"
	pt_op_t_type_str          = "TRACK_TYPE"
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
	case PATTERN_OP_TRACK_RID:
		return pt_op_rid_str
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
	case PATTERN_OP_TRACK_TYPE:
		return pt_op_t_type_str
	default:
		return pt_op_unknown_str
	}
}

type PublicationPattern = proto.PublicationPattern

func String(me *proto.PublicationPattern) string {
	return fmt.Sprintf("{ Op: %v, Args: %v, Children: %v }", me.Op, me.Args, me.Children)
}

func checkArgsNum(me *proto.PublicationPattern, n int, atLeast bool) error {
	if !atLeast && len(me.Args) != n {
		return errors.InvalidPubPattern(fmt.Sprintf("The pub pattern op '%v' accept %d args, but gotten %v", me.Op, n, me.Args))
	} else if atLeast && len(me.Args) < n {
		return errors.InvalidPubPattern(fmt.Sprintf("The pub pattern op '%v' accept at least %d args, but gotten %v", me.Op, n, me.Args))
	} else {
		return nil
	}
}

func Validate(me *proto.PublicationPattern) error {
	var err error
	switch me.Op {
	case proto.PatternOp_PATTERN_OP_ALL, proto.PatternOp_PATTERN_OP_SOME, proto.PatternOp_PATTERN_OP_NONE:
		if err = checkArgsNum(me, 0, false); err != nil {
			return err
		}
		for _, c := range me.Children {
			err := Validate(c)
			if err != nil {
				return err
			}
		}
	default:
		if len(me.Children) > 0 {
			return errors.InvalidPubPattern("The pub pattern op '%v' accept zero child, but gotten %v", me.Op, me.Children)
		}
		switch me.Op {
		case proto.PatternOp_PATTERN_OP_PUBLISH_ID, proto.PatternOp_PATTERN_OP_STREAM_ID, proto.PatternOp_PATTERN_OP_TRACK_ID, proto.PatternOp_PATTERN_OP_TRACK_RID, proto.PatternOp_PATTERN_OP_TRACK_LABEL_ALL_HAS, proto.PatternOp_PATTERN_OP_TRACK_LABEL_SOME_HAS, proto.PatternOp_PATTERN_OP_TRACK_LABEL_NONE_HAS, proto.PatternOp_PATTERN_OP_TRACK_TYPE:
			if err = checkArgsNum(me, 1, true); err != nil {
				return err
			}
			if me.Op == proto.PatternOp_PATTERN_OP_TRACK_TYPE {
				t := me.Args[0]
				if t != "video" && t != "audio" {
					return errors.InvalidPubPattern("The pub pattern op '%v' accept only \"video\" and \"audio\", but gotten %v", me.Op, me.Args)
				}
			}
		case proto.PatternOp_PATTERN_OP_TRACK_LABEL_ALL_MATCH, proto.PatternOp_PATTERN_OP_TRACK_LABEL_SOME_MATCH, proto.PatternOp_PATTERN_OP_TRACK_LABEL_NONE_MATCH:
			if l := len(me.Args); l < 2 || l%2 != 0 {
				return errors.InvalidPubPattern("The pub pattern op '%v' accept at least 2 even number of args, but gotten %v", me.Op, me.Children)
			}
		default:
			return errors.InvalidPubPattern("Unkonwn op: %d", me.Op)
		}
	}
	return nil
}

func MatchLabel(me *Track, name, value string) bool {
	v, ok := me.GetLabels()[name]
	if ok {
		return v == value
	} else {
		return false
	}
}

func HasLabel(me *Track, name string) bool {
	if me.GetLabels() == nil {
		return false
	}
	_, ok := me.GetLabels()[name]
	return ok
}

func Match(me *proto.PublicationPattern, track *Track) bool {
	switch me.Op {
	case proto.PatternOp_PATTERN_OP_ALL:
		for _, c := range me.Children {
			if !Match(c, track) {
				return false
			}
		}
		return true
	case proto.PatternOp_PATTERN_OP_SOME:
		for _, c := range me.Children {
			if Match(c, track) {
				return true
			}
		}
		return false
	case proto.PatternOp_PATTERN_OP_NONE:
		for _, c := range me.Children {
			if Match(c, track) {
				return false
			}
		}
		return true
	case proto.PatternOp_PATTERN_OP_PUBLISH_ID:
		return utils.InSlice(me.Args, track.GetPubId(), nil)
	case proto.PatternOp_PATTERN_OP_STREAM_ID:
		return utils.InSlice(me.Args, track.GetStreamId(), nil)
	case proto.PatternOp_PATTERN_OP_TRACK_ID:
		return utils.InSlice(me.Args, track.GetGlobalId(), nil)
	case proto.PatternOp_PATTERN_OP_TRACK_RID:
		return utils.InSlice(me.Args, track.GetRid(), nil)
	case proto.PatternOp_PATTERN_OP_TRACK_LABEL_ALL_MATCH:
		for i := 0; i < len(me.Args); {
			key := me.Args[i]
			value := me.Args[i+1]
			if !MatchLabel(track, key, value) {
				return false
			}
			i += 2
		}
		return true
	case proto.PatternOp_PATTERN_OP_TRACK_LABEL_SOME_MATCH:
		for i := 0; i < len(me.Args); {
			key := me.Args[i]
			value := me.Args[i+1]
			if MatchLabel(track, key, value) {
				return true
			}
			i += 2
		}
		return false
	case proto.PatternOp_PATTERN_OP_TRACK_LABEL_NONE_MATCH:
		for i := 0; i < len(me.Args); {
			key := me.Args[i]
			value := me.Args[i+1]
			if MatchLabel(track, key, value) {
				return false
			}
			i += 2
		}
		return true
	case proto.PatternOp_PATTERN_OP_TRACK_LABEL_ALL_HAS:
		for _, name := range me.Args {
			if !HasLabel(track, name) {
				return false
			}
		}
		return true
	case proto.PatternOp_PATTERN_OP_TRACK_LABEL_SOME_HAS:
		for _, name := range me.Args {
			if HasLabel(track, name) {
				return true
			}
		}
		return false
	case proto.PatternOp_PATTERN_OP_TRACK_LABEL_NONE_HAS:
		for _, name := range me.Args {
			if HasLabel(track, name) {
				return false
			}
		}
		return true
	case proto.PatternOp_PATTERN_OP_TRACK_TYPE:
		return utils.InSlice(me.Args, track.GetType(), nil)
	default:
		return false
	}
}

func MatchTracks(me *proto.PublicationPattern, tracks []*Track, reqTypes []string) (matched []*Track, unmatched []*Track) {
	tracksByPub := map[string][]*Track{}
	unmatched = []*Track{}
	for _, track := range tracks {
		if Match(me, track) {
			ts := tracksByPub[track.GetPubId()]
			tracksByPub[track.GetPubId()] = append(ts, track)
		} else {
			unmatched = append(unmatched, track)
		}
	}
	reqVideo := utils.InSlice(reqTypes, "video", nil)
	reqAudio := utils.InSlice(reqTypes, "audio", nil)
	matched = []*Track{}
	for _, ts := range tracksByPub {
		videoCond := !reqVideo
		audioCond := !reqAudio
		if !videoCond || !audioCond {
			for _, track := range ts {
				if track.GetType() == "video" {
					videoCond = true
				} else if track.GetType() == "audio" {
					audioCond = true
				}
				if videoCond && audioCond {
					break
				}
			}
		}
		if !videoCond || !audioCond {
			unmatched = append(unmatched, ts...)
		} else {
			matched = append(matched, ts...)
		}
	}
	return
}
