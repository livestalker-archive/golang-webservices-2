package main

import (
	"errors"
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	if reflect.TypeOf(out).Elem().Kind() != reflect.Struct && reflect.TypeOf(out).Elem().Kind() != reflect.Slice {
		return errors.New("out must be struct")
	}
	st := reflect.TypeOf(out).Elem()
	sv := reflect.ValueOf(out).Elem()
	if reflect.TypeOf(out).Elem().Kind() == reflect.Struct {
		n := st.NumField()
		fillStructFromMap(st, sv, n, data, out)
	}
	if reflect.TypeOf(out).Elem().Kind() == reflect.Slice {
		fillStructFromSlice(st, sv, data, out)
	}
	return nil
}

func fillStructFromMap(st reflect.Type, sv reflect.Value, n int, data interface{}, out interface{}) reflect.Value {
	if sv.Type().Kind() == reflect.Ptr {
		sv = sv.Elem()
	}
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
				switch nestedV := v.(type) {
				case map[string]interface{}:
					fillStructFromMap(st.Field(i).Type, sv.Field(i), st.Field(i).Type.NumField(), m[fName], out)
				case []interface{}:
					for ix, _ := range nestedV {
						newEl := reflect.New(st.Field(i).Type.Elem())
						tmp := fillStructFromMap(st.Field(i).Type.Elem(), newEl, newEl.Elem().NumField(), nestedV[ix], out)
						sv.Field(i).Set(reflect.Append(sv.Field(i), tmp))
					}
				}
			}
		}
	}
	return sv
}

func fillStructFromSlice(st reflect.Type, sv reflect.Value, data interface{}, out interface{}) reflect.Value {
	for ix, _ := range data.([]interface{}) {
		newEl := reflect.New(st.Elem())
		tmp := fillStructFromMap(st.Elem(), newEl, newEl.Elem().NumField(), data.([]interface{})[ix], out)
		sv.Set(reflect.Append(sv, tmp))
		fmt.Println(sv)
	}
	return reflect.ValueOf(1)
}
