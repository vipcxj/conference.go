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

func AssertEqualWP[T comparable](t *testing.T, expected T, real T, prefixMsg string) {
	t.Helper()
	if expected != real {
		t.Errorf("%sexpect %v, but got %v", prefixMsg, expected, real)
	}
}

func AssertEqual[T comparable](t *testing.T, expected T, real T) {
	t.Helper()
	AssertEqualWP(t, expected, real, "")
}

func AssertEqualSlice[T comparable, L []T](t *testing.T, expected L, real L) {
	t.Helper()
	AssertEqualSliceWP(t, expected, real, "")
}

func AssertEqualSliceWP[T comparable, L []T](t *testing.T, expected L, real L, prefixMsg string) {
	t.Helper()
	if len(expected) != len(real) {
		t.Errorf("%sexpect %v, but got %v", prefixMsg, expected, real)
		return
	}
	for i := 0; i < len(expected); i++ {
		if expected[i] != real[i] {
			t.Errorf("%sexpect %v, but got %v", prefixMsg, expected, real)
			return
		}
	}
}

func AssertEqualMap[K comparable, V comparable](t *testing.T, expected map[K]V, real map[K]V) {
	t.Helper()
	AssertEqualMapWP(t, expected, real, "")
}

func AssertEqualMapWP[K comparable, V comparable](t *testing.T, expected map[K]V, real map[K]V, prefixMsg string) {
	t.Helper()
	if len(expected) != len(real) {
		t.Errorf("%sexpect %v, but got %v", prefixMsg, expected, real)
		return
	}
	for k, v := range expected {
		r, found := real[k]
		if !found || r != v {
			t.Errorf("%sexpect %v, but got %v", prefixMsg, expected, real)
			return
		}
	}
}

func AssertTrue(t *testing.T, real bool) {
	t.Helper()
	AssertTrueWP(t, real, "")
}

func AssertTrueWP(t *testing.T, real bool, prefixMsg string) {
	t.Helper()
	AssertEqualWP(t, true, real, prefixMsg)
}

func AssertFalse(t *testing.T, real bool) {
	t.Helper()
	AssertFalseWP(t, real, "")
}

func AssertFalseWP(t *testing.T, real bool, prefixMsg string) {
	t.Helper()
	AssertEqualWP(t, false, real, prefixMsg)
}
