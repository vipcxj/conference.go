package utils

import (
	"cmp"
	"slices"

	"github.com/vipcxj/conference.go/errors"
)

func SimpleEqualer[T comparable](t1, t2 T) bool {
	return t1 == t2
}

func InSlice[T comparable](slice []T, target T, comparer func(T, T) bool) bool {
	if comparer == nil {
		comparer = SimpleEqualer
	}
	for _, e := range slice {
		if comparer(e, target) {
			return true
		}
	}
	return false
}

func InSlice2[T any](slice []T, tester func(T) bool) bool {
	for _, e := range slice {
		if tester(e) {
			return true
		}
	}
	return false
}

func IndexOf[T comparable](slice []T, target T, comparer func(T, T) bool) int {
	if comparer == nil {
		comparer = SimpleEqualer
	}
	for i, e := range slice {
		if comparer(e, target) {
			return i
		}
	}
	return -1
}

func RemoveByIndexFromSlice[T any](slice []T, copy bool, index int) ([]T, bool) {
	lenSlice := len(slice)
	if index < 0 || index >= lenSlice {
		return slice, false
	}
	if copy {
		removed := make([]T, 0, lenSlice-1)
		removed = append(removed, slice[0:index]...)
		removed = append(removed, slice[index+1:]...)
		return removed, true
	} else {
		if index == 0 {
			return slice[1:], true
		}
		if index == lenSlice-1 {
			return slice[0 : lenSlice-1], true
		}
		return append(slice[0:index], slice[index+1:]...), true
	}
}

func ComparableCompare[T cmp.Ordered](t1, t2 T) int {
	if t1 == t2 {
		return 0
	} else if t1 < t2 {
		return -1
	} else {
		return 1
	}
}

func RemoveByIndexesFromSlice[T any](slice []T, copy bool, indexes ...int) []T {
	if len(indexes) == 0 {
		return slice
	}
	var out []T
	if copy {
		out = make([]T, 0)
	} else {
		out = slice[0:0]
	}
	slices.SortFunc(indexes, ComparableCompare)
	p := 0
	for i, index := range indexes {
		if index >= 0 {
			p = i
			break
		}
	}
	e := -1
	lenSlice := len(slice)
	for i := len(indexes) - 1; i >= 0; i -- {
		index := indexes[i]
		if index < lenSlice {
			e = i
		}
	}
	if e == -1 {
		return slice
	}
	
	for i, v := range slice {
		index := indexes[p]
		if i != index {
			out = append(out, v)
		} else {
			p ++
		}
		if p > e {
			break
		}
	}
	return out
}

func RemoveByValueFromSlice[T comparable](slice []T, copy bool, value T) []T {
	var out []T
	if copy {
		out = make([]T, 0)
	} else {
		out = slice[0:0]
	}
	for _, v := range slice {
		if v != value {
			out = append(out, v)
		}
	}
	return out
}

func InMap[K comparable, T any](m map[K]T, tester func(T) bool) bool {
	for _, e := range m {
		if tester(e) {
			return true
		}
	}
	return false
}

func MapSlice[T any, O any](slice []T, mapper func(T) (mapped O, remove bool)) []O {
	if slice == nil {
		return nil
	}
	out := make([]O, len(slice))
	pos := 0
	for i := 0; i < len(slice); i++ {
		o, remove := mapper(slice[i])
		if !remove {
			out[pos] = o
			pos++
		}
	}
	return out[0:pos]
}

func SliceToMap[T any, K comparable, V any](slice []T, mapper func(T, int) (key K, value V, remove bool), merger func(old V, new V) V) map[K]V {
	if slice == nil {
		return nil
	}
	if merger == nil {
		merger = func(v1, v2 V) V {
			return v2
		}
	}
	out := map[K]V{}
	for i, t := range slice {
		k, v, remove := mapper(t, i)
		if !remove {
			old, ok := out[k]
			if ok {
				out[k] = merger(old, v)
			} else {
				out[k] = v
			}
		}
	}
	return out
}

func XOrBytes(bytes1 []byte, bytes2 []byte, out []byte) {
	if out == nil {
		panic(errors.FatalError("to xor two bytes array, the output must not be nil"))
	}
	len1 := len(bytes1)
	len2 := len(bytes2)
	if len1 != len2 {
		panic(errors.FatalError("to xor two bytes array, the length of bytes 1 must be equal to bytes 2"))
	}
	if len(out) < len1 {
		panic(errors.FatalError("to xor two bytes array, the length of the output must be greater or equal to the length of them"))
	}
	for i := 0; i < len1; i++ {
		out[i] = bytes1[i] ^ bytes2[i]
	}
}
