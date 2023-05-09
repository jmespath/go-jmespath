package jmespath

import (
	"reflect"
	"strings"
)

type objectKind int

const (
	objectKindNone objectKind = iota
	objectKindStruct
	objectKindMapStringInterface
	objectKindMapStringOther
)

func getObjectKind(value interface{}) (objectKind, reflect.Value) {
	rv := reflect.Indirect(reflect.ValueOf(value))
	if rv.Kind() == reflect.Struct {
		return objectKindStruct, rv
	}
	if rv.Kind() == reflect.Map {
		rt := rv.Type()
		if rt.Key().Kind() == reflect.String {
			if rt.Elem().Kind() == reflect.Interface {
				return objectKindMapStringInterface, rv
			}
			return objectKindMapStringOther, rv
		}
	}
	return objectKindNone, rv
}

func isObject(value interface{}) bool {
	kind, _ := getObjectKind(value)
	return kind != objectKindNone
}

func toObject(value interface{}) map[string]interface{} {
	kind, rv := getObjectKind(value)
	switch kind {
	case objectKindStruct:
		// This does not flatten fields from anonymous embedded structs into the top-level struct
		// the way the encoding/json package does, as this is quite complicated. These fields can
		// still be accessed by specifying the full path to the embedded field. See the typeFields()
		// function in https://go.dev/src/encoding/json/encode.go if you feel the need to do add
		// flattening functionality.
		ret := make(map[string]interface{})
		rt := rv.Type()
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if f.IsExported() {
				var key string
				switch t := f.Tag.Get("json"); t {
				case "", "-":
					key = f.Name
				default:
					if i := strings.IndexByte(t, ','); i >= 0 {
						if i == 0 {
							key = f.Name
						} else {
							key = t[:i]
						}
					} else {
						key = t
					}
				}
				ret[key] = rv.Field(i).Interface()
			}
		}
		return ret
	case objectKindMapStringInterface:
		return value.(map[string]interface{})
	case objectKindMapStringOther:
		ret := make(map[string]interface{})
		iter := rv.MapRange()
		for iter.Next() {
			ret[iter.Key().String()] = iter.Value().Interface()
		}
		return ret
	default:
		return nil
	}
}
