package signal_test

import (
	"testing"

	"github.com/vipcxj/conference.go/signal"
	"github.com/vipcxj/conference.go/utils"
)

func TestMatchRoom(t *testing.T) {
	utils.ShouldPanicWith(t, "pattern is required", func ()  {
		signal.MatchRoom("", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern .", func ()  {
		signal.MatchRoom(".", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern a.", func ()  {
		signal.MatchRoom("a.", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern .a", func ()  {
		signal.MatchRoom(".a", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern ..", func ()  {
		signal.MatchRoom("..", "")
	})
	utils.ShouldPanicWith(t, "invalid pattern a..b", func ()  {
		signal.MatchRoom("a..b", "")
	})
	utils.ShouldPanicWith(t, "room is required", func ()  {
		signal.MatchRoom("*", "")
	})
	utils.ShouldPanicWith(t, "invalid room *", func ()  {
		signal.MatchRoom("*", "*")
	})
	utils.ShouldPanicWith(t, "invalid room *a*", func ()  {
		signal.MatchRoom("*", "*a*")
	})
	utils.ShouldPanicWith(t, "invalid room .", func ()  {
		signal.MatchRoom("*", ".")
	})
	utils.ShouldPanicWith(t, "invalid room a.", func ()  {
		signal.MatchRoom("*", "a.")
	})
	utils.ShouldPanicWith(t, "invalid room .a", func ()  {
		signal.MatchRoom("*", ".a")
	})
	utils.ShouldPanicWith(t, "invalid room a..b", func ()  {
		signal.MatchRoom("*", "a..b")
	})
	utils.ShouldPanicWith(t, "invalid room a...b", func ()  {
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
	utils.AssertFalse(t, signal.MatchRoom("a.**.*.b", "a.b"))
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