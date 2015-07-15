package jputil

import (
	"errors"
	"reflect"
)

func IsFalse(value interface{}) bool {
	// A value is considered "false" like in JMESPath if its:
	// - An empty string, array, hash.
	// - The boolean value false.
	// - nil
	if value == nil {
		return true
	} else if value == false {
		return true
	} else if aSlice, ok := value.([]interface{}); ok && len(aSlice) == 0 {
		return true
	} else if aMap, ok := value.(map[string]interface{}); ok && len(aMap) == 0 {
		return true
	} else if aStr, ok := value.(string); ok && len(aStr) == 0 {
		return true
	}
	return false
}

func ObjsEqual(left interface{}, right interface{}) bool {
	if (left == nil) || (right == nil) {
		return left == right
	}
	if reflect.DeepEqual(left, right) {
		return true
	}
	return false
}

type SliceParam struct {
	N         int
	Specified bool
}

// Slice supports [start:stop:step] style slicing.
func Slice(slice []interface{}, parts []SliceParam) ([]interface{}, error) {
	computed, err := computeSliceParams(len(slice), parts)
	if err != nil {
		return nil, err
	}
	start, stop, step := computed[0], computed[1], computed[2]
	result := make([]interface{}, 0)
	if step > 0 {
		for i := start; i < stop; i += step {
			result = append(result, slice[i])
		}
	} else {
		for i := start; i > stop; i += step {
			result = append(result, slice[i])
		}
	}
	return result, nil
}

func computeSliceParams(length int, parts []SliceParam) ([]int, error) {
	var start, stop, step int
	if !parts[2].Specified {
		step = 1
	} else if parts[2].N == 0 {
		return nil, errors.New("Invalid slice, step cannot be 0")
	} else {
		step = parts[2].N
	}
	var stepValueNegative bool
	if step < 0 {
		stepValueNegative = true
	} else {
		stepValueNegative = false
	}

	if !parts[0].Specified {
		if stepValueNegative {
			start = length - 1
		} else {
			start = 0
		}
	} else {
		start = capSlice(length, parts[0].N, step)
	}

	if !parts[1].Specified {
		if stepValueNegative {
			stop = -1
		} else {
			stop = length
		}
	} else {
		stop = capSlice(length, parts[1].N, step)
	}
	return []int{start, stop, step}, nil
}

func capSlice(length int, actual int, step int) int {
	if actual < 0 {
		actual += length
		if actual < 0 {
			if step < 0 {
				actual = -1
			} else {
				actual = 0
			}
		}
	} else if actual >= length {
		if step < 0 {
			actual = length - 1
		} else {
			actual = length
		}
	}
	return actual
}
