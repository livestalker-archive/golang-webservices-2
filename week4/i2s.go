package main

import (
	"errors"
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	if reflect.TypeOf(out).Kind() != reflect.Ptr {
		return errors.New("out must be pointer")
	}
	if reflect.TypeOf(data).Kind() != reflect.Map && reflect.TypeOf(out).Elem().Kind() != reflect.Struct {
		err := checkTypes(reflect.TypeOf(data).Kind(), reflect.TypeOf(out).Elem().Kind())
		if err != nil {
			return err
		}
	}
	if reflect.TypeOf(data).Kind() == reflect.Slice && reflect.TypeOf(out).Elem().Kind() == reflect.Struct {
		return errors.New("Error")
	}
	st := reflect.TypeOf(out).Elem()
	sv := reflect.ValueOf(out).Elem()
	if reflect.TypeOf(out).Elem().Kind() == reflect.Struct {
		n := st.NumField()
		_, err := fillStructFromMap(st, sv, n, data, out)
		return err
	}
	if reflect.TypeOf(out).Elem().Kind() == reflect.Slice {
		_, err := fillStructFromSlice(st, sv, data, out)
		return err
	}
	return nil
}

func fillStructFromMap(st reflect.Type, sv reflect.Value, n int, data interface{}, out interface{}) (reflect.Value, error) {
	if sv.Type().Kind() == reflect.Ptr {
		sv = sv.Elem()
	}
	for i := 0; i < n; i++ {
		fName := st.Field(i).Name
		if m, ok := data.(map[string]interface{}); ok {
			err := checkTypes(st.Field(i).Type.Kind(), reflect.TypeOf(m[fName]).Kind())
			if err != nil {
				return reflect.ValueOf(0), err
			}
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
						tmp, _ := fillStructFromMap(st.Field(i).Type.Elem(), newEl, newEl.Elem().NumField(), nestedV[ix], out)
						sv.Field(i).Set(reflect.Append(sv.Field(i), tmp))
					}
				}
			}
		}
	}
	return sv, nil
}

func fillStructFromSlice(st reflect.Type, sv reflect.Value, data interface{}, out interface{}) (reflect.Value, error) {
	for ix, _ := range data.([]interface{}) {
		newEl := reflect.New(st.Elem())
		tmp, _ := fillStructFromMap(st.Elem(), newEl, newEl.Elem().NumField(), data.([]interface{})[ix], out)
		sv.Set(reflect.Append(sv, tmp))
		fmt.Println(sv)
	}
	return reflect.ValueOf(0), nil
}

func checkTypes(t1 reflect.Kind, t2 reflect.Kind) error {
	if t1 == reflect.Int && t2 == reflect.Float64 {
		return nil
	}
	if t1 == reflect.Struct && t2 == reflect.Map {
		return nil
	}
	if t1 != t2 {
		return errors.New("Error types")
	}
	return nil
}
