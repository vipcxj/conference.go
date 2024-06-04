package utils

import "reflect"

func IsNilInterface(i any) bool {
	if i == nil {
	   return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
	   return reflect.ValueOf(i).IsNil()
	}
	return false
 }