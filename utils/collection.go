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

func InMap[K comparable, T any](m map[K]T, tester func(T) bool) bool {
	for _, e := range m {
		if tester(e) {
			return true
		}
	}
	return false
}

func SliceMapChange[S ~[]T, T any](slice S, mapper func(T) (mapped T, remove bool)) S {
	if slice == nil {
		return nil
	}
	pos := 0
	lenIn := len(slice)
	for i := 0; i < lenIn; i++ {
		o, remove := mapper(slice[i])
		if !remove {
			slice[pos] = o
			pos++
		}
	}
	if pos > 0 {
		slice = slice[0:pos]
		if pos < lenIn / 2 {
			return slices.Clone(slice)
		} else {
			return slice
		}
	} else {
		return nil
	}
}

func SliceMapNew[S ~[]T, T any, O any](slice S, mapper func(T) (mapped O, remove bool)) []O {
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

func SliceAppendNoRepeat[S ~[]E, E comparable](slice S, values ...E) S {
	for _, value := range values {
		if !slices.Contains(slice, value) {
			slice = append(slice, value)
		}
	}
	return slice
}

func SortedSliceAppendNoRepeat[S ~[]E, E cmp.Ordered](slice S, values ...E) S {
	for _, value := range values {
		_, found := slices.BinarySearch(slice, value)
		if !found {
			slice = append(slice, value)
		}
	}
	slices.Sort(slice)
	return slice
}

func SliceAnyMatch[S ~[]T, T any](slice S, predicate func(T) bool) bool {
	for _, v := range slice {
		if predicate(v) {
			return true
		}
	}
	return false
}

func SliceAllMatch[S ~[]T, T any](slice S, predicate func(T) bool) bool {
	for _, v := range slice {
		if !predicate(v) {
			return false
		}
	}
	return true
}

func SliceToMap[S ~[]T, T any, K comparable, V any](slice S, mapper func(T, int) (key K, value V, remove bool), merger func(old V, new V) V) map[K]V {
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

func SliceRemoveByIndex[S ~[]T, T any](slice S, copy bool, index int) (S, bool) {
	l := len(slice)
	if l == 0 {
		return slice, false
	}
	if index < 0 || index >= l {
		return slice, false
	}
	if copy {
		removed := make(S, 0, l-1)
		removed = append(removed, slice[0:index]...)
		removed = append(removed, slice[index+1:]...)
		return removed, true
	} else {
		if index == 0 {
			return slice[1:], true
		}
		if index == l-1 {
			return slice[0 : l-1], true
		}
		return append(slice[0:index], slice[index+1:]...), true
	}
}

func SliceClone[S ~[]T, T any](slice S) S {
	clone := make(S, len(slice))
	copy(clone, slice)
	return clone
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

func SliceRemoveByIndexes[S ~[]T, T any](slice S, copy bool, indexes ...int) S {
	if len(indexes) == 0 {
		return slice
	}
	l := len(slice)
	if l == 0 {
		return slice
	}
	var out S
	if copy {
		out = make(S, 0)
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
	for i := len(indexes) - 1; i >= 0; i-- {
		index := indexes[i]
		if index < l {
			e = i
			break
		}
	}
	if e == -1 {
		return slice
	}
	skip := false
	for i, v := range slice {
		if skip {
			out = append(out, v)
		} else {
			index := indexes[p]
			if i != index {
				out = append(out, v)
			} else {
				p++
				for p <= e && index == indexes[p] {
					p++
				}
			}
			if p > e {
				skip = true
			}
		}
	}
	return out
}

func SliceRemoveByValue[S ~[]T, T comparable](slice S, copy bool, value T) S {
	l := len(slice)
	if l == 0 {
		return slice
	}
	var out S
	if copy {
		out = make(S, 0)
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

func SliceRemoveByValues[S ~[]T, T comparable](slice S, copy bool, values ...T) S {
	l := len(slice)
	if l == 0 {
		return slice
	}
	var out S
	if copy {
		out = make(S, 0)
	} else {
		out = slice[0:0]
	}
	for _, v := range slice {
		if !InSlice(values, v, nil) {
			out = append(out, v)
		}
	}
	return out
}

func SliceRemoveByValuesAndReturnRemoved[S ~[]T, T comparable](slice S, copy bool, values ...T) (out S, removed S) {
	l := len(slice)
	if l == 0 {
		out = slice
		removed = nil
		return
	}
	if copy {
		out = make(S, 0)
	} else {
		out = slice[0:0]
	}
	removed = make(S, 0)
	for _, v := range slice {
		if !InSlice(values, v, nil) {
			out = append(out, v)
		} else {
			removed = append(removed, v)
		}
	}
	return
}

func SliceRemoveIf[S ~[]T, T any](slice S, copy bool, cond func(v T) bool) S {
	l := len(slice)
	if l == 0 {
		return slice
	}
	var out S
	if copy {
		out = make(S, 0)
	} else {
		out = slice[0:0]
	}
	for _, v := range slice {
		if !cond(v) {
			out = append(out, v)
		}
	}
	return out
}

func SliceRemoveIfIgnoreOrder[S ~[]T, T any](slice S, cond func(v T) bool) S {
	l := len(slice)
	if l == 0 {
		return slice
	}
	removed := 0
	last := l - 1
	for i := 0; i <= last; i++ {
		v := slice[i]
		if cond(v) {
			for (last > i && cond(slice[last])) || last == i {
				last--
				removed++
			}
			if last > i {
				slice[i] = slice[last]
				last--
				removed++
			}
		}
	}
	return slice[0 : l-removed]
}

func MapValues[K comparable, T any](m map[K]T) []T {
	if m == nil {
		return nil
	}
	values := make([]T, len(m))
	i := 0
	for _, value := range m {
		values[i] = value
		i++
	}
	return values
}

func MapKeys[K comparable, T any](m map[K]T) []K {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys;
}

func MapValuesTo[K comparable, V any, T any](m map[K]V, mapper func(K, V) (mapped T, remove bool)) []T {
	if m == nil {
		return nil
	}
	values := make([]T, len(m))
	i := 0
	for key, value := range m {
		mapped, remove := mapper(key, value)
		if !remove {
			values[i] = mapped
			i++
		}
	}
	return values[0:i]
}

func MapPutMap[K comparable, V any](src map[K]V, target map[K]V) map[K]V {
	for k, v := range src {
		target[k] = v
	}
	return target
}

func MapAnyMatch[K comparable, T any](m map[K]T, predicate func(K, T) bool) bool {
	for key, value := range m {
		if predicate(key, value) {
			return true
		}
	}
	return false
}

func MapAllMatch[K comparable, T any](m map[K]T, predicate func(K, T) bool) bool {
	for key, value := range m {
		if !predicate(key, value) {
			return false
		}
	}
	return true
}

func MapFindFirst[K comparable, T any](m map[K]T, predicate func(K, T) bool) (K, T, bool) {
	for key, value := range m {
		if predicate(key, value) {
			return key, value, true
		}
	}
	var zk K
	var zt T
	return zk, zt, false
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
