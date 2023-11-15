package utils

import (
	"fmt"
	"testing"
)

func ShouldPanic(t *testing.T, f func()) {
	t.Helper()
    defer func() { _ = recover() }()
    f()
    t.Errorf("should have panicked here")
}

func ShouldPanicWith(t *testing.T, msg string, f func()) {
    t.Helper()
    defer func() {
		err := recover()
		if err == nil {
			t.Errorf("should have panicked here")
		} else {
			switch e := err.(type) {
			case error:
				if e.Error() != msg {
					t.Errorf("should have panicked with msg %s, but got %s", msg, e.Error())
				}
			default:
				real_msg := fmt.Sprintf("%v", e)
				if real_msg != msg {
					t.Errorf("should have panicked with msg %s, but got %s", msg, real_msg)
				}
			}
		}
	}()
    f()
}

func AssertEqual[T comparable](t *testing.T, expected T, real T) {
	t.Helper()
	if expected != real {
		t.Errorf("expect %v, but got %v", expected, real)
	}
}

func AssertTrue(t *testing.T, real bool) {
	t.Helper()
	AssertEqual(t, true, real)
}

func AssertFalse(t *testing.T, real bool) {
	t.Helper()
	AssertEqual(t, false, real)
}