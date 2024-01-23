package common_test

import (
	"testing"

	"github.com/vipcxj/conference.go/pkg/common"
	"github.com/vipcxj/conference.go/utils"
)

type TestStruct0 struct {
	V int
	PV *string
}

type TestStruct1 struct {
	A string
	B TestStruct0
	C *TestStruct0
	D []string
	E *[]int
	F []bool
}

type TestStruct2 struct {
	A string
	B TestStruct0
	C TestStruct0
	D []string
	E []int
	F []bool
}

func TestShallowCopy(t *testing.T) {
	s1 := "s1"
	s2 := "s2"
	pt := &TestStruct0 {
		V: 3,
		PV: &s2,
	}
	ea := []int{1, 2, 3}
	src := TestStruct1 {
		A: "a",
		B: TestStruct0 {
			V: 2,
			PV: &s1,
		},
		C: pt,
		D: []string{"a", "b", "c"},
		E: &ea,
	}
	var target TestStruct2
	err := common.ShallowCopy(src, &target)
	utils.AssertTrue(t, err == nil)
	utils.AssertEqual(t, src.A, target.A)
	utils.AssertEqual(t, src.B, target.B)
	utils.AssertEqual(t, *src.C, target.C)
	utils.AssertEqualSlice(t, src.D, target.D)
	utils.AssertEqualSlice(t, *src.E, target.E)
	utils.AssertEqualSlice(t, src.F, target.F)
}

func TestGetFieldByPath(t *testing.T) {
	obj := map[string]interface{}{
		"a": struct {
			B int
		}{1},
	}
	real, err := common.GetFieldByPath("a.B", obj)
	if err != nil {
		t.Error(err)
	}
	utils.AssertEqual(t, 1, real)
}
