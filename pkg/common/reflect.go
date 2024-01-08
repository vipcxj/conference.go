package common

import (
	"fmt"
	"reflect"
	"strings"
)

func GetFieldByPath(path string, object interface{}) (interface{}, error) {
	keySlice := strings.Split(path, ".")
	p := ""
	t := reflect.TypeOf(object)
	v := reflect.ValueOf(object)
	for i, key := range keySlice {
		if p == "" {
			p = key
		} else {
			p = fmt.Sprintf("%s.%s", p, key)
		}
		for v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Map {
			if t.Key().Kind() != reflect.String {
				return nil, fmt.Errorf("unable to get field %s of %v, %s is a map, but its key is not string", path, object, p)
			}
			keyValue := reflect.ValueOf(key)
			v = v.MapIndex(keyValue)
			if !v.IsValid() {
				return nil, fmt.Errorf("unable to get field %s of %v", path, object)
			}
			t = v.Type()
		} else if v.Kind() == reflect.Struct {
			v = v.FieldByName(key)
			t = v.Type()
		} else if v.Kind() == reflect.Interface {
			tmp := v.Interface()
			b := reflect.ValueOf(tmp)
			if b.Kind() != reflect.Interface {
				keys := keySlice[i:]
				return GetFieldByPath(strings.Join(keys, "."), tmp)
			} else {
				return nil, fmt.Errorf("unable to get field %s of %v", path, object)
			}
		} else {
			return nil, fmt.Errorf("unable to get field %s of %v", path, object)
		}
	}
	if v.CanInterface() {
		return v.Interface(), nil
	} else {
		return nil, nil
	}
}
