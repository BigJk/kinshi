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
