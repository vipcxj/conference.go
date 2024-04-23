package utils_test

import (
	"testing"

	"github.com/vipcxj/conference.go/utils"
)

func TestSliceRemoveByIndex(t *testing.T) {
	s := []int {3, 1, 5, 2, 2, 6, 2, 1}
	real, removed := utils.SliceRemoveByIndex(s, true, -1)
	utils.AssertEqualSlice(t, []int {3, 1, 5, 2, 2, 6, 2, 1}, real)
	utils.AssertEqual(t, false, removed)
	real, removed = utils.SliceRemoveByIndex(s, false, -1)
	utils.AssertEqualSlice(t, []int {3, 1, 5, 2, 2, 6, 2, 1}, real)
	utils.AssertEqual(t, false, removed)
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	real, removed = utils.SliceRemoveByIndex(s, true, 0)
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, real)
	utils.AssertEqual(t, true, removed)
	real, removed = utils.SliceRemoveByIndex(s, false, 0)
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, real)
	utils.AssertEqual(t, true, removed)
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	real, removed = utils.SliceRemoveByIndex(s, true, 1)
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2, 1}, real)
	utils.AssertEqual(t, true, removed)
	real, removed = utils.SliceRemoveByIndex(s, false, 1)
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2, 1}, real)
	utils.AssertEqual(t, true, removed)
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	real, removed = utils.SliceRemoveByIndex(s, false, 7)
	utils.AssertEqualSlice(t, []int {3, 1, 5, 2, 2, 6, 2}, real)
	utils.AssertEqual(t, true, removed)
	real, removed = utils.SliceRemoveByIndex(s, true, 7)
	utils.AssertEqualSlice(t, []int {3, 1, 5, 2, 2, 6, 2}, real)
	utils.AssertEqual(t, true, removed)
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	real, removed = utils.SliceRemoveByIndex(s, false, 8)
	utils.AssertEqualSlice(t, []int {3, 1, 5, 2, 2, 6, 2, 1}, real)
	utils.AssertEqual(t, false, removed)
	real, removed = utils.SliceRemoveByIndex(s, true, 8)
	utils.AssertEqualSlice(t, []int {3, 1, 5, 2, 2, 6, 2, 1}, real)
	utils.AssertEqual(t, false, removed)
}

func TestSliceRemoveByIndexes(t *testing.T) {
	s := []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByIndexes(s, true, -1))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByIndexes(s, false, -1))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByIndexes(s, true, 0))
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByIndexes(s, false, 0))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByIndexes(s, true, -1, 0))
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByIndexes(s, false, -1, 0))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByIndexes(s, true, 0, -1))
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByIndexes(s, false, 0, -1))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2}, utils.SliceRemoveByIndexes(s, true, 1, 7))
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2}, utils.SliceRemoveByIndexes(s, false, 1, 7))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveByIndexes(s, true, 3, 4, 6))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveByIndexes(s, false, 3, 4, 6))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveByIndexes(s, true, 6, 3, 4))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveByIndexes(s, false, 6, 3, 4))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveByIndexes(s, true, 3, 4, 5, 6))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveByIndexes(s, false, 3, 4, 5, 6))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveByIndexes(s, true, 6, 5, 4, 3))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveByIndexes(s, false, 6, 5, 4, 3))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveByIndexes(s, true, 6, 3, 5, 4, 4, 3))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveByIndexes(s, false, 6, 3, 5, 4, 4, 3))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {2, 2, 6, 2}, utils.SliceRemoveByIndexes(s, true, 0, 1, 2, 7))
	utils.AssertEqualSlice(t, []int {2, 2, 6, 2}, utils.SliceRemoveByIndexes(s, false, 0, 1, 2, 7))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {2, 2, 6, 2}, utils.SliceRemoveByIndexes(s, true, -1, 0, 1, 2, 7, 100))
	utils.AssertEqualSlice(t, []int {2, 2, 6, 2}, utils.SliceRemoveByIndexes(s, false, -1, 0, 1, 2, 7, 100))
}

func TestSliceRemoveByValue(t *testing.T) {
	s := []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByValue(s, true, 3))
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByValue(s, false, 3))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2}, utils.SliceRemoveByValue(s, true, 1))
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2}, utils.SliceRemoveByValue(s, false, 1))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 2, 2, 6, 2, 1}, utils.SliceRemoveByValue(s, true, 5))
	utils.AssertEqualSlice(t, []int {3, 1, 2, 2, 6, 2, 1}, utils.SliceRemoveByValue(s, false, 5))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveByValue(s, true, 2))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveByValue(s, false, 2))
}

func TestSliceRemoveByValues(t *testing.T) {
	s := []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByValues(s, true, 3))
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByValues(s, false, 3))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByValues(s, true, 3, 3))
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveByValues(s, false, 3, 3))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2}, utils.SliceRemoveByValues(s, true, 1))
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2}, utils.SliceRemoveByValues(s, false, 1))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveByValues(s, true, 2))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveByValues(s, false, 2))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveByValues(s, true, 2, 6))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveByValues(s, false, 2, 6))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {2, 2, 6, 2}, utils.SliceRemoveByValues(s, true, 5, 1, 3))
	utils.AssertEqualSlice(t, []int {2, 2, 6, 2}, utils.SliceRemoveByValues(s, false, 5, 1, 3))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	utils.AssertEqualSlice(t, []int {6}, utils.SliceRemoveByValues(s, true, 5, 1, 3, 2))
	utils.AssertEqualSlice(t, []int {6}, utils.SliceRemoveByValues(s, false, 5, 1, 3, 2))
}

func TestSliceRemoveIf(t *testing.T) {
	s := []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond := func (v int) bool {
		return v == 3
	}
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveIf(s, true, cond))
	utils.AssertEqualSlice(t, []int {1, 5, 2, 2, 6, 2, 1}, utils.SliceRemoveIf(s, false, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 1
	}
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2}, utils.SliceRemoveIf(s, true, cond))
	utils.AssertEqualSlice(t, []int {3, 5, 2, 2, 6, 2}, utils.SliceRemoveIf(s, false, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 2
	}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveIf(s, true, cond))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 6, 1}, utils.SliceRemoveIf(s, false, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 2 || v == 6
	}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveIf(s, true, cond))
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveIf(s, false, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 5 || v == 1 || v == 3
	}
	utils.AssertEqualSlice(t, []int {2, 2, 6, 2}, utils.SliceRemoveIf(s, true, cond))
	utils.AssertEqualSlice(t, []int {2, 2, 6, 2}, utils.SliceRemoveIf(s, false, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 5 || v == 1 || v == 3 || v == 2
	}
	utils.AssertEqualSlice(t, []int {6}, utils.SliceRemoveIf(s, true, cond))
	utils.AssertEqualSlice(t, []int {6}, utils.SliceRemoveIf(s, false, cond))
}

func TestSliceRemoveIfIgnoreOrder(t *testing.T) {
	s := []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond := func (v int) bool {
		return v == 3
	}
	utils.AssertEqualSlice(t, []int {1, 1, 5, 2, 2, 6, 2}, utils.SliceRemoveIfIgnoreOrder(s, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 1
	}
	utils.AssertEqualSlice(t, []int {3, 2, 5, 2, 2, 6}, utils.SliceRemoveIfIgnoreOrder(s, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 2
	}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1, 6}, utils.SliceRemoveIfIgnoreOrder(s, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 2 || v == 6
	}
	utils.AssertEqualSlice(t, []int {3, 1, 5, 1}, utils.SliceRemoveIfIgnoreOrder(s, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 5 || v == 1 || v == 3
	}
	utils.AssertEqualSlice(t, []int {2, 6, 2, 2}, utils.SliceRemoveIfIgnoreOrder(s, cond))
	s = []int {3, 1, 5, 2, 2, 6, 2, 1}
	cond = func (v int) bool {
		return v == 5 || v == 1 || v == 3 || v == 2
	}
	utils.AssertEqualSlice(t, []int {6}, utils.SliceRemoveIfIgnoreOrder(s, cond))
}