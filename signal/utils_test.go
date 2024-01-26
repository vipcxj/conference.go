package signal_test

import (
	"fmt"
	"testing"

	"github.com/vipcxj/conference.go/signal"
	"github.com/vipcxj/conference.go/utils"
)

type PatternMapTestMode int

const (
	NEW PatternMapTestMode = iota
	UPDATE
	REMOVE
	REMOVE_NO_OP
)

func checkCreatePatternMap(t *testing.T, m *signal.PatternMap[string], p string, mode PatternMapTestMode, raw map[string]string) {
	t.Helper()
	_, found := raw[p]
	switch mode {
	case UPDATE:
		utils.AssertTrue(t, found)
		raw[p] = fmt.Sprintf("%s-update", p)
	case NEW:
		utils.AssertFalse(t, found)
		raw[p] = p
	case REMOVE:
		utils.AssertTrue(t, found)
		delete(raw, p)
	case REMOVE_NO_OP:
		utils.AssertFalse(t, found)
	default:
		t.Fail()
	}
	m.UpdatePatternData(p, func(old string, found bool) (new string, remove bool) {
		switch mode {
		case NEW, REMOVE_NO_OP:
			utils.AssertFalse(t, found)
		case UPDATE, REMOVE:
			utils.AssertTrue(t, found)
		default:
			t.Fail()
		}
		if mode == UPDATE {
			p = fmt.Sprintf("%s-update", p)
		}
		return p, (mode == REMOVE || mode == REMOVE_NO_OP)
	})
	utils.AssertEqualMap(t, raw, m.RawData())
}

func checkMatchPatternMap(t *testing.T, pm *signal.PatternMap[string], m string, raw map[string]string) {
	t.Helper()
	ps := pm.Collect(m)
	for _, p := range ps {
		utils.AssertTrueWP(t, signal.MatchRoom(p, m), fmt.Sprintf("%s should match %s, ", m, p))
	}
	for p, _ := range raw {
		if !utils.InSlice(ps, p, nil) {
			utils.AssertFalseWP(t, signal.MatchRoom(p, m), fmt.Sprintf("%s should not match %s, ", m, p))
		}
	}
}

func TestCreatePatternMap(t *testing.T) {
	m := signal.NewPatternMap[string]()
	raw := make(map[string]string)
	checkCreatePatternMap(t, m, "a", NEW, raw)
	checkCreatePatternMap(t, m, "a.b", NEW, raw)
	checkCreatePatternMap(t, m, "a", UPDATE, raw)
	checkCreatePatternMap(t, m, "a.b", UPDATE, raw)
	checkCreatePatternMap(t, m, "a", REMOVE, raw)
	checkCreatePatternMap(t, m, "a", REMOVE_NO_OP, raw)
	checkCreatePatternMap(t, m, "a.b", REMOVE, raw)
	checkCreatePatternMap(t, m, "a.b", REMOVE_NO_OP, raw)
	checkCreatePatternMap(t, m, "a", NEW, raw)
	checkCreatePatternMap(t, m, "a.b", NEW, raw)
	checkCreatePatternMap(t, m, "a.c", NEW, raw)
	checkCreatePatternMap(t, m, "c.a", NEW, raw)
	checkCreatePatternMap(t, m, "*", NEW, raw)
	checkCreatePatternMap(t, m, "*.*", NEW, raw)
	checkCreatePatternMap(t, m, "a.*", NEW, raw)
	checkCreatePatternMap(t, m, "*.a", NEW, raw)
	checkCreatePatternMap(t, m, "**", NEW, raw)
	checkCreatePatternMap(t, m, "a.**", NEW, raw)
	checkCreatePatternMap(t, m, "a.**.b", NEW, raw)
	checkCreatePatternMap(t, m, "a.**.*.b", NEW, raw)
	checkCreatePatternMap(t, m, "a.**.**.b", NEW, raw)
	checkCreatePatternMap(t, m, "a.**.**.*.b", NEW, raw)
	checkCreatePatternMap(t, m, "a.**.*.**.b", NEW, raw)
	checkCreatePatternMap(t, m, "a.**.e.*.f.**.b", NEW, raw)
	checkCreatePatternMap(t, m, "**.b.c", NEW, raw)
	checkCreatePatternMap(t, m, "**.b.c.**", NEW, raw)

	checkMatchPatternMap(t, m, "a", raw)
	checkMatchPatternMap(t, m, "b", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "c.c", raw)
	checkMatchPatternMap(t, m, "a", raw)
	checkMatchPatternMap(t, m, "abc", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "a", raw)
	checkMatchPatternMap(t, m, "a.a", raw)
	checkMatchPatternMap(t, m, "a", raw)
	checkMatchPatternMap(t, m, "b.a", raw)
	checkMatchPatternMap(t, m, "a", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "abc", raw)
	checkMatchPatternMap(t, m, "a.b.cd", raw)
	checkMatchPatternMap(t, m, "a", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "a.b.c", raw)
	checkMatchPatternMap(t, m, "a.b.c.b", raw)
	checkMatchPatternMap(t, m, "a.b.c.b.c", raw)
	checkMatchPatternMap(t, m, "a.b.c.b", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "a.b", raw)
	checkMatchPatternMap(t, m, "a.c.b", raw)
	checkMatchPatternMap(t, m, "a.c.b", raw)
	checkMatchPatternMap(t, m, "a.c.b", raw)
	checkMatchPatternMap(t, m, "a.c.b", raw)
	checkMatchPatternMap(t, m, "a.e.f.b", raw)
	checkMatchPatternMap(t, m, "a.e.c.f.b", raw)
	checkMatchPatternMap(t, m, "a.x.e.c.f.b", raw)
	checkMatchPatternMap(t, m, "a.x.e.c.f.y.b", raw)
	checkMatchPatternMap(t, m, "a.x.x.e.c.f.y.b", raw)
	checkMatchPatternMap(t, m, "a.x.x.e.c.f.y.y.b", raw)
	checkMatchPatternMap(t, m, "a.x.x.e.c.f.y.y.c", raw)
	checkMatchPatternMap(t, m, "a.c.e.b.c", raw)
	checkMatchPatternMap(t, m, "a.c.e.c.c", raw)
	checkMatchPatternMap(t, m, "b.c", raw)
	checkMatchPatternMap(t, m, "a.c.b.c", raw)
	checkMatchPatternMap(t, m, "a.c.b.c.c", raw)
	checkMatchPatternMap(t, m, "b.c.b.c", raw)
}

func TestMatchRoom(t *testing.T) {
	utils.ShouldPanicWith(t, "pattern is required", func() {
		signal.MatchRoom("", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern .", func() {
		signal.MatchRoom(".", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern a.", func() {
		signal.MatchRoom("a.", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern .a", func() {
		signal.MatchRoom(".a", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern ..", func() {
		signal.MatchRoom("..", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern a..b", func() {
		signal.MatchRoom("a..b", "")
	})
	utils.ShouldPanicWith(t, "room is required", func() {
		signal.MatchRoom("*", "")
	})
	utils.ShouldPanicWith(t, "invalid room *", func() {
		signal.MatchRoom("*", "*")
	})
	utils.ShouldPanicWith(t, "invalid room *a*", func() {
		signal.MatchRoom("*", "*a*")
	})
	utils.ShouldPanicWith(t, "invalid room .", func() {
		signal.MatchRoom("*", ".")
	})
	utils.ShouldPanicWith(t, "invalid room a.", func() {
		signal.MatchRoom("*", "a.")
	})
	utils.ShouldPanicWith(t, "invalid room .a", func() {
		signal.MatchRoom("*", ".a")
	})
	utils.ShouldPanicWith(t, "invalid room a..b", func() {
		signal.MatchRoom("*", "a..b")
	})
	utils.ShouldPanicWith(t, "invalid room a...b", func() {
		signal.MatchRoom("*", "a...b")
	})
	utils.AssertTrue(t, signal.MatchRoom("a", "a"))
	utils.AssertFalse(t, signal.MatchRoom("a", "b"))
	utils.AssertTrue(t, signal.MatchRoom("a.b", "a.b"))
	utils.AssertFalse(t, signal.MatchRoom("a.c", "a.b"))
	utils.AssertFalse(t, signal.MatchRoom("c.a", "c.c"))
	utils.AssertTrue(t, signal.MatchRoom("*", "a"))
	utils.AssertTrue(t, signal.MatchRoom("*", "abc"))
	utils.AssertFalse(t, signal.MatchRoom("*", "a.b"))
	utils.AssertTrue(t, signal.MatchRoom("*.*", "a.b"))
	utils.AssertFalse(t, signal.MatchRoom("a.*", "a"))
	utils.AssertTrue(t, signal.MatchRoom("a.*", "a.a"))
	utils.AssertFalse(t, signal.MatchRoom("*.a", "a"))
	utils.AssertTrue(t, signal.MatchRoom("*.a", "b.a"))
	utils.AssertTrue(t, signal.MatchRoom("**", "a"))
	utils.AssertTrue(t, signal.MatchRoom("**", "a.b"))
	utils.AssertTrue(t, signal.MatchRoom("**", "abc"))
	utils.AssertTrue(t, signal.MatchRoom("**", "a.b.cd"))
	utils.AssertTrue(t, signal.MatchRoom("a.**", "a"))
	utils.AssertTrue(t, signal.MatchRoom("a.**", "a.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**", "a.b.c"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.b", "a.b.c.b"))
	utils.AssertFalse(t, signal.MatchRoom("a.**.b", "a.b.c.b.c"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.*.b", "a.b.c.b"))
	utils.AssertFalse(t, signal.MatchRoom("a.**.*.b", "a"))
	utils.AssertFalse(t, signal.MatchRoom("a.**.*.b", "a.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.**.b", "a.b"))
	utils.AssertFalse(t, signal.MatchRoom("a.**.**.*.b", "a.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.*.b", "a.c.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.**.*.b", "a.c.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.*.**.b", "a.c.b"))
	utils.AssertFalse(t, signal.MatchRoom("a.**.e.*.f.**.b", "a.c.b"))
	utils.AssertFalse(t, signal.MatchRoom("a.**.e.*.f.**.b", "a.e.f.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.e.*.f.**.b", "a.e.c.f.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.e.*.f.**.b", "a.x.e.c.f.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.e.*.f.**.b", "a.x.e.c.f.y.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.e.*.f.**.b", "a.x.x.e.c.f.y.b"))
	utils.AssertTrue(t, signal.MatchRoom("a.**.e.*.f.**.b", "a.x.x.e.c.f.y.y.b"))
	utils.AssertFalse(t, signal.MatchRoom("a.**.e.*.f.**.b", "a.x.x.e.c.f.y.y.c"))
	utils.AssertTrue(t, signal.MatchRoom("**.b.c", "a.c.e.b.c"))
	utils.AssertFalse(t, signal.MatchRoom("**.b.c", "a.c.e.c.c"))
	utils.AssertTrue(t, signal.MatchRoom("**.b.c.**", "b.c"))
	utils.AssertTrue(t, signal.MatchRoom("**.b.c.**", "a.c.b.c"))
	utils.AssertTrue(t, signal.MatchRoom("**.b.c.**", "a.c.b.c.c"))
	utils.AssertTrue(t, signal.MatchRoom("**.b.c.**", "b.c.b.c"))
}
