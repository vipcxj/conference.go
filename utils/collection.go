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