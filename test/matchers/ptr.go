package matchers

import (
	"fmt"
	"reflect"
)

// deref attempts to deref a pointer to a value, returning the value and an error if the type of passed
// object is not matching expected type.
func deref[T any](obj any) (T, error) {
	objValue := reflect.ValueOf(obj)
	if objValue.Kind() == reflect.Ptr {
		objValue = objValue.Elem()
	}

	if !objValue.IsValid() {
		return *new(T), fmt.Errorf("invalid value for type: %T", obj)
	}

	castObj, ok := objValue.Interface().(T)
	if !ok {
		return *new(T), fmt.Errorf("failed to cast %T to %T", obj, objValue.Interface())
	}

	return castObj, nil
}
