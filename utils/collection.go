package utils

import (
	"github.com/vipcxj/conference.go/errors"
)

func InSlice[T comparable](slice []T, target T, comparer func(T, T) bool) bool {
	if comparer == nil {
		comparer = func(t1, t2 T) bool {
			return t1 == t2
		}
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

func InMap[K comparable, T any](m map[K]T,  tester func(T) bool) bool {
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

func SliceToMap[T any, K comparable, V any](slice []T, mapper func(T) (key K, value V, remove bool), merger func(old V, new V) V) map[K]V {
	if slice == nil {
		return nil
	}
	if merger == nil {
		merger = func(v1, v2 V) V {
			return v2
		}
	}
	out := map[K]V{}
	for _, t := range slice {
		k, v, remove := mapper(t)
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
	for i := 0; i < len1; i ++ {
		out[i] = bytes1[i] ^ bytes2[i]
	}
}