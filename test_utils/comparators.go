package test_utils

import (
	"fmt"
	"reflect"
	"time"
)

// DeepEqual performs deep equality comparison with custom handling for time fields
func DeepEqual(expected, actual interface{}) error {
	return deepEqual(reflect.ValueOf(expected), reflect.ValueOf(actual), "")
}

func deepEqual(expected, actual reflect.Value, path string) error {
	// Handle nil values
	if !expected.IsValid() || !actual.IsValid() {
		if expected.IsValid() != actual.IsValid() {
			return fmt.Errorf("%s: one value is nil, the other is not", path)
		}
		return nil
	}

	// Handle different types
	if expected.Type() != actual.Type() {
		return fmt.Errorf("%s: type mismatch: expected %s, got %s", path, expected.Type(), actual.Type())
	}

	// Handle pointers
	if expected.Kind() == reflect.Ptr {
		if expected.IsNil() != actual.IsNil() {
			return fmt.Errorf("%s: pointer nil mismatch: expected %v, got %v", path, expected.IsNil(), actual.IsNil())
		}
		if expected.IsNil() {
			return nil
		}
		return deepEqual(expected.Elem(), actual.Elem(), path)
	}

	// Handle time.Time with tolerance
	if expected.Type() == reflect.TypeOf(time.Time{}) {
		return compareTime(expected.Interface().(time.Time), actual.Interface().(time.Time), path)
	}

	// Handle structs
	if expected.Kind() == reflect.Struct {
		for i := 0; i < expected.NumField(); i++ {
			field := expected.Type().Field(i)
			fieldPath := path + "." + field.Name
			if fieldPath[0] == '.' {
				fieldPath = fieldPath[1:]
			}

			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			if err := deepEqual(expected.Field(i), actual.Field(i), fieldPath); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle slices
	if expected.Kind() == reflect.Slice {
		if expected.Len() != actual.Len() {
			return fmt.Errorf("%s: slice length mismatch: expected %d, got %d", path, expected.Len(), actual.Len())
		}
		for i := 0; i < expected.Len(); i++ {
			if err := deepEqual(expected.Index(i), actual.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle maps
	if expected.Kind() == reflect.Map {
		if expected.Len() != actual.Len() {
			return fmt.Errorf("%s: map length mismatch: expected %d, got %d", path, expected.Len(), actual.Len())
		}
		for _, key := range expected.MapKeys() {
			expectedValue := expected.MapIndex(key)
			actualValue := actual.MapIndex(key)
			if !actualValue.IsValid() {
				return fmt.Errorf("%s: missing key %v in actual map", path, key.Interface())
			}
			if err := deepEqual(expectedValue, actualValue, fmt.Sprintf("%s[%v]", path, key.Interface())); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle basic types
	if !reflect.DeepEqual(expected.Interface(), actual.Interface()) {
		return fmt.Errorf("%s: value mismatch: expected %v, got %v", path, expected.Interface(), actual.Interface())
	}

	return nil
}

// compareTime compares two time.Time values with tolerance
func compareTime(expected, actual time.Time, path string) error {
	const tolerance = time.Second

	if expected.IsZero() != actual.IsZero() {
		return fmt.Errorf("%s: time zero mismatch: expected %v, got %v", path, expected.IsZero(), actual.IsZero())
	}

	if expected.IsZero() {
		return nil
	}

	diff := expected.Sub(actual)
	if diff < -tolerance || diff > tolerance {
		return fmt.Errorf("%s: time mismatch: expected %v, got %v (diff: %v)", path, expected, actual, diff)
	}

	return nil
}

// EqualIgnoringFields compares two structs for equality while ignoring specified fields
func EqualIgnoringFields(expected, actual interface{}, ignoreFields ...string) error {
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	if expectedValue.Type() != actualValue.Type() {
		return fmt.Errorf("type mismatch: expected %s, got %s", expectedValue.Type(), actualValue.Type())
	}

	if expectedValue.Kind() != reflect.Struct {
		return fmt.Errorf("both values must be structs, got %s", expectedValue.Kind())
	}

	ignoreMap := make(map[string]bool)
	for _, field := range ignoreFields {
		ignoreMap[field] = true
	}

	for i := 0; i < expectedValue.NumField(); i++ {
		field := expectedValue.Type().Field(i)
		fieldName := field.Name

		if ignoreMap[fieldName] {
			continue
		}

		if !field.IsExported() {
			continue
		}

		if err := deepEqual(expectedValue.Field(i), actualValue.Field(i), fieldName); err != nil {
			return err
		}
	}

	return nil
}

// EqualIgnoringTime compares two structs for equality while ignoring all time fields
func EqualIgnoringTime(expected, actual interface{}) error {
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	if expectedValue.Type() != actualValue.Type() {
		return fmt.Errorf("type mismatch: expected %s, got %s", expectedValue.Type(), actualValue.Type())
	}

	if expectedValue.Kind() != reflect.Struct {
		return fmt.Errorf("both values must be structs, got %s", expectedValue.Kind())
	}

	for i := 0; i < expectedValue.NumField(); i++ {
		field := expectedValue.Type().Field(i)
		fieldName := field.Name

		if !field.IsExported() {
			continue
		}

		expectedField := expectedValue.Field(i)
		actualField := actualValue.Field(i)

		// Skip time fields
		if expectedField.Type() == reflect.TypeOf(time.Time{}) {
			continue
		}

		if err := deepEqual(expectedField, actualField, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// ContainsSlice checks if a slice contains an element (with deep comparison)
func ContainsSlice(slice, element interface{}) (bool, error) {
	sliceValue := reflect.ValueOf(slice)
	elementValue := reflect.ValueOf(element)

	if sliceValue.Kind() != reflect.Slice {
		return false, fmt.Errorf("expected slice, got %T", slice)
	}

	for i := 0; i < sliceValue.Len(); i++ {
		if err := deepEqual(sliceValue.Index(i), elementValue, fmt.Sprintf("slice[%d]", i)); err == nil {
			return true, nil
		}
	}

	return false, nil
}

// SlicesEqual compares two slices for equality (with deep comparison)
func SlicesEqual(expected, actual interface{}) error {
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	if expectedValue.Kind() != reflect.Slice || actualValue.Kind() != reflect.Slice {
		return fmt.Errorf("both values must be slices, got %s and %s", expectedValue.Kind(), actualValue.Kind())
	}

	if expectedValue.Len() != actualValue.Len() {
		return fmt.Errorf("slice length mismatch: expected %d, got %d", expectedValue.Len(), actualValue.Len())
	}

	for i := 0; i < expectedValue.Len(); i++ {
		if err := deepEqual(expectedValue.Index(i), actualValue.Index(i), fmt.Sprintf("slice[%d]", i)); err != nil {
			return err
		}
	}

	return nil
}

// MapsEqual compares two maps for equality (with deep comparison)
func MapsEqual(expected, actual interface{}) error {
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	if expectedValue.Kind() != reflect.Map || actualValue.Kind() != reflect.Map {
		return fmt.Errorf("both values must be maps, got %s and %s", expectedValue.Kind(), actualValue.Kind())
	}

	if expectedValue.Len() != actualValue.Len() {
		return fmt.Errorf("map length mismatch: expected %d, got %d", expectedValue.Len(), actualValue.Len())
	}

	for _, key := range expectedValue.MapKeys() {
		expectedMapValue := expectedValue.MapIndex(key)
		actualMapValue := actualValue.MapIndex(key)
		if !actualMapValue.IsValid() {
			return fmt.Errorf("missing key %v in actual map", key.Interface())
		}
		if err := deepEqual(expectedMapValue, actualMapValue, fmt.Sprintf("map[%v]", key.Interface())); err != nil {
			return err
		}
	}

	return nil
}

// CompareFields compares specific fields between two structs
func CompareFields(expected, actual interface{}, fields ...string) error {
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	if expectedValue.Type() != actualValue.Type() {
		return fmt.Errorf("type mismatch: expected %s, got %s", expectedValue.Type(), actualValue.Type())
	}

	if expectedValue.Kind() != reflect.Struct {
		return fmt.Errorf("both values must be structs, got %s", expectedValue.Kind())
	}

	for _, fieldName := range fields {
		expectedField := expectedValue.FieldByName(fieldName)
		actualField := actualValue.FieldByName(fieldName)

		if !expectedField.IsValid() {
			return fmt.Errorf("field %s not found in expected struct", fieldName)
		}

		if !actualField.IsValid() {
			return fmt.Errorf("field %s not found in actual struct", fieldName)
		}

		if err := deepEqual(expectedField, actualField, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// CompareNumericRanges compares numeric values within a tolerance
func CompareNumericRanges(expected, actual, tolerance float64, fieldName string) error {
	diff := expected - actual
	if diff < -tolerance || diff > tolerance {
		return fmt.Errorf("%s: numeric value %v is outside tolerance %v of expected %v (diff: %v)",
			fieldName, actual, tolerance, expected, diff)
	}
	return nil
}

// CompareStringLists compares string slices ignoring order
func CompareStringLists(expected, actual []string) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("string list length mismatch: expected %d, got %d", len(expected), len(actual))
	}

	expectedMap := make(map[string]bool)
	for _, s := range expected {
		expectedMap[s] = true
	}

	actualMap := make(map[string]bool)
	for _, s := range actual {
		actualMap[s] = true
	}

	for s := range expectedMap {
		if !actualMap[s] {
			return fmt.Errorf("string '%s' missing from actual list", s)
		}
	}

	for s := range actualMap {
		if !expectedMap[s] {
			return fmt.Errorf("string '%s' unexpected in actual list", s)
		}
	}

	return nil
}

// ComparePartialStruct compares only the non-zero fields of expected with actual
func ComparePartialStruct(expected, actual interface{}) error {
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	if expectedValue.Type() != actualValue.Type() {
		return fmt.Errorf("type mismatch: expected %s, got %s", expectedValue.Type(), actualValue.Type())
	}

	if expectedValue.Kind() != reflect.Struct {
		return fmt.Errorf("both values must be structs, got %s", expectedValue.Kind())
	}

	for i := 0; i < expectedValue.NumField(); i++ {
		field := expectedValue.Type().Field(i)
		fieldName := field.Name

		if !field.IsExported() {
			continue
		}

		expectedField := expectedValue.Field(i)
		actualField := actualValue.Field(i)

		// Skip zero values in expected
		if isZero(expectedField) {
			continue
		}

		if err := deepEqual(expectedField, actualField, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// isZero checks if a value is the zero value for its type
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Array, reflect.String:
		return v.Len() == 0
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.Ptr:
		return v.IsNil()
	case reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return v.Interface().(time.Time).IsZero()
		}
		// For other structs, check if all fields are zero
		for i := 0; i < v.NumField(); i++ {
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// CompareWithCustomComparator uses a custom comparator function for specific fields
func CompareWithCustomComparator(expected, actual interface{}, comparator func(fieldName string, expected, actual interface{}) error) error {
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	if expectedValue.Type() != actualValue.Type() {
		return fmt.Errorf("type mismatch: expected %s, got %s", expectedValue.Type(), actualValue.Type())
	}

	if expectedValue.Kind() != reflect.Struct {
		return fmt.Errorf("both values must be structs, got %s", expectedValue.Kind())
	}

	for i := 0; i < expectedValue.NumField(); i++ {
		field := expectedValue.Type().Field(i)
		fieldName := field.Name

		if !field.IsExported() {
			continue
		}

		expectedField := expectedValue.Field(i)
		actualField := actualValue.Field(i)

		if comparator != nil {
			if err := comparator(fieldName, expectedField.Interface(), actualField.Interface()); err != nil {
				return err
			}
		} else {
			if err := deepEqual(expectedField, actualField, fieldName); err != nil {
				return err
			}
		}
	}

	return nil
}
