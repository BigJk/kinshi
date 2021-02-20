package kinshi

import (
	"fmt"
	"reflect"
)

func getTypeName(s interface{}) string {
	if reflect.TypeOf(s).Kind() == reflect.Ptr {
		return reflect.TypeOf(s).Elem().Name()
	}
	return reflect.TypeOf(s).Name()
}

func hasType(s interface{}, t interface{}) error {
	if reflect.TypeOf(s).Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target wasn't a struct")
	}

	var name string
	if reflect.TypeOf(t).Kind() == reflect.Ptr {
		name = reflect.TypeOf(t).Elem().Name()
	} else {
		name = reflect.TypeOf(t).Name()
	}

	if !reflect.ValueOf(s).Elem().FieldByName(name).IsValid() {
		return fmt.Errorf("struct doesn't have type")
	}

	return nil
}

func findType(s interface{}, out interface{}) error {
	if reflect.TypeOf(s).Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target wasn't a struct")
	}

	if reflect.TypeOf(out).Kind() != reflect.Ptr {
		return fmt.Errorf("out wasn't a pointer")
	}

	searchElem := reflect.TypeOf(out).Elem()
	foundVal := reflect.ValueOf(s).Elem().FieldByName(searchElem.Name())
	if !foundVal.IsValid() {
		return ErrNotFound
	}

	reflect.ValueOf(out).Elem().Set(foundVal)
	return nil
}

func setType(s interface{}, in interface{}) error {
	if reflect.TypeOf(s).Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target wasn't a struct")
	}

	var valIn reflect.Value
	if reflect.TypeOf(in).Kind() == reflect.Ptr {
		valIn = reflect.ValueOf(in).Elem()
	} else {
		valIn = reflect.ValueOf(in)
	}

	searchElem := reflect.TypeOf(in).Elem()
	foundVal := reflect.ValueOf(s).Elem().FieldByName(searchElem.Name())
	if !foundVal.IsValid() {
		return ErrNotFound
	}

	foundVal.Set(valIn)
	return nil
}
