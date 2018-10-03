package main

import (
	"errors"
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	if reflect.TypeOf(out).Elem().Kind() != reflect.Struct {
		return errors.New("out ust be struct")
	}
	st := reflect.TypeOf(out).Elem()
	sv := reflect.ValueOf(out).Elem()
	fmt.Println(sv)
	n := st.NumField()
	fillStruct(st, sv, n, data, out)
	return nil
}

func fillStruct(st reflect.Type, sv reflect.Value, n int, data interface{}, out interface{}) {
	for i := 0; i < n; i++ {
		fName := st.Field(i).Name
		if m, ok := data.(map[string]interface{}); ok {
			switch v := m[fName].(type) {
			case string:
				sv.Field(i).SetString(v)
			case float64:
				sv.Field(i).SetInt(int64(v))
			case bool:
				sv.Field(i).SetBool(v)
			case interface{}:
				fillStruct(st.Field(i).Type, sv.Field(i), st.Field(i).Type.NumField(), data, out)
			}
		}
	}
}
