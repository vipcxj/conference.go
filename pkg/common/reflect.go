package common

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	ERR_SHALLOW_COPY_SRC_INVALID = "shallowCopy method got an invalid src argument"
	ERR_SHALLOW_COPY_SRC_MUST_BE_STRUCT = "shallowCopy only accept a struct or pointer to struct src argument"
	ERR_SHALLOW_COPY_TARGET_INVALID = "shallowCopy method got an invalid target argument"
	ERR_SHALLOW_COPY_TARGET_MUST_BE_STRUCT_POINTER = "shallowCopy only accept a pointer to struct target argument"
	ERR_SHALLOW_COPY_FIELD_TYPE_MISMATCH_PREFIX = "shallowCopy failed. fields type mismatched: "
)

func unwrapPointer(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	return v
}

func ShallowCopy(src interface{}, target interface{}) error {
	if src == nil || target == nil {
		return nil
	}
	srcValue := reflect.ValueOf(src)
	if !srcValue.IsValid() {
		return fmt.Errorf(ERR_SHALLOW_COPY_SRC_INVALID)
	}
	targetValue := reflect.ValueOf(target)
	if !targetValue.IsValid() {
		return fmt.Errorf(ERR_SHALLOW_COPY_TARGET_INVALID)
	}
	if targetValue.Kind() != reflect.Pointer {
		return fmt.Errorf(ERR_SHALLOW_COPY_TARGET_MUST_BE_STRUCT_POINTER)
	}
	return shallowCopy(srcValue, targetValue, "")
}

func shallowCopy(srcValue reflect.Value, targetValue reflect.Value, parent string) error {
	srcValue = unwrapPointer(srcValue)
	if srcValue.Kind() != reflect.Struct {
		return fmt.Errorf(ERR_SHALLOW_COPY_SRC_MUST_BE_STRUCT)
	}
	targetValue = unwrapPointer(targetValue)
	if targetValue.Kind() != reflect.Struct {
		return fmt.Errorf(ERR_SHALLOW_COPY_TARGET_MUST_BE_STRUCT_POINTER)
	}
	for i := 0; i < srcValue.NumField(); i++ {
		fieldName := srcValue.Type().Field(i).Name
		fieldValue := srcValue.Field(i)
		fieldType := fieldValue.Type()
		targetFieldValue := targetValue.FieldByName(fieldName)
		targetFieldType := targetFieldValue.Type()
		if fieldType != targetFieldType && fieldValue.Kind() == reflect.Pointer {
			fieldValue = fieldValue.Elem()
			fieldType = fieldValue.Type()
		}
		if targetFieldValue.IsValid() && targetFieldValue.CanSet() {
			if fieldType != targetFieldType {
				return fmt.Errorf("%s%s.%s", ERR_SHALLOW_COPY_FIELD_TYPE_MISMATCH_PREFIX, parent, fieldName)
			}
			if targetFieldValue.Kind() == reflect.Struct {
				shallowCopy(fieldValue, targetFieldValue, fmt.Sprintf("%s.%s", parent, fieldName))
			} else {
				targetFieldValue.Set(fieldValue)
			}
		}
	}
	return nil
}

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
		v = unwrapPointer(v)
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
