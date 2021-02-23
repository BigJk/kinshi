package kinshi

import (
	"fmt"
	"reflect"
)

func getTypeName(s interface{}) string {
	t := reflect.TypeOf(s)
	if t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	}
	return t.Name()
}

func fetchPtrOfType(s interface{}, typeName string) (interface{}, error) {
	if reflect.TypeOf(s).Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("target wasn't a struct")
	}

	foundVal := reflect.ValueOf(s).Elem().FieldByName(typeName)
	if !foundVal.IsValid() {
		return nil, ErrNotFound
	}

	return foundVal.Addr().Interface(), nil
}
