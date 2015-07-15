package jmespath

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/jmespath/jmespath.go/jmespath/jputil"
)

type jpFunction func(arguments []interface{}) (interface{}, error)

type jpType string

const (
	jpUnknown     jpType = "unknown"
	jpNumber      jpType = "number"
	jpString      jpType = "string"
	jpArray       jpType = "array"
	jpObject      jpType = "object"
	jpArrayNumber jpType = "array[number]"
	jpArrayString jpType = "array[string]"
	jpExpref      jpType = "expref"
	jpAny         jpType = "any"
)

type functionEntry struct {
	name      string
	arguments []argSpec
	handler   jpFunction
}

type argSpec struct {
	types    []jpType
	variadic bool
}

var functionTable = map[string]functionEntry{
	"length": functionEntry{
		name: "length",
		arguments: []argSpec{
			argSpec{types: []jpType{jpString, jpArray, jpObject}},
		},
		handler: jpfLength,
	},
	"starts_with": functionEntry{
		name: "starts_with",
		arguments: []argSpec{
			argSpec{types: []jpType{jpString}},
			argSpec{types: []jpType{jpString}},
		},
		handler: jpfStartsWith,
	},
	"abs": functionEntry{
		name: "abs",
		arguments: []argSpec{
			argSpec{types: []jpType{jpNumber}},
		},
		handler: jpfAbs,
	},
	"avg": functionEntry{
		name: "avg",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArrayNumber}},
		},
		handler: jpfAvg,
	},
	"ceil": functionEntry{
		name: "ceil",
		arguments: []argSpec{
			argSpec{types: []jpType{jpNumber}},
		},
		handler: jpfCeil,
	},
	"contains": functionEntry{
		name: "contains",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArray, jpString}},
			argSpec{types: []jpType{jpAny}},
		},
		handler: jpfContains,
	},
	"ends_with": functionEntry{
		name: "ends_with",
		arguments: []argSpec{
			argSpec{types: []jpType{jpString}},
			argSpec{types: []jpType{jpString}},
		},
		handler: jpfEndsWith,
	},
	"floor": functionEntry{
		name: "floor",
		arguments: []argSpec{
			argSpec{types: []jpType{jpNumber}},
		},
		handler: jpfFloor,
	},
	"max": functionEntry{
		name: "max",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArrayNumber, jpArrayString}},
		},
		handler: jpfMax,
	},
	"merge": functionEntry{
		name: "merge",
		arguments: []argSpec{
			argSpec{types: []jpType{jpObject}, variadic: true},
		},
		handler: jpfMerge,
	},
	"max_by": functionEntry{
		name: "max_by",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArray}},
			argSpec{types: []jpType{jpExpref}},
		},
		handler: jpfMaxBy,
	},
	"sum": functionEntry{
		name: "sum",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArrayNumber}},
		},
		handler: jpfSum,
	},
	"min": functionEntry{
		name: "min",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArrayNumber, jpArrayString}},
		},
		handler: jpfMin,
	},
	"min_by": functionEntry{
		name: "min_by",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArray}},
			argSpec{types: []jpType{jpExpref}},
		},
		handler: jpfMinBy,
	},
	"type": functionEntry{
		name: "type",
		arguments: []argSpec{
			argSpec{types: []jpType{jpAny}},
		},
		handler: jpfType,
	},
	"keys": functionEntry{
		name: "keys",
		arguments: []argSpec{
			argSpec{types: []jpType{jpObject}},
		},
		handler: jpfKeys,
	},
	"values": functionEntry{
		name: "values",
		arguments: []argSpec{
			argSpec{types: []jpType{jpObject}},
		},
		handler: jpfValues,
	},
	"sort": functionEntry{
		name: "sort",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArrayString, jpArrayNumber}},
		},
		handler: jpfSort,
	},
	"sort_by": functionEntry{
		name: "sort_by",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArray}},
			argSpec{types: []jpType{jpExpref}},
		},
		handler: jpfSortBy,
	},
	"join": functionEntry{
		name: "join",
		arguments: []argSpec{
			argSpec{types: []jpType{jpString}},
			argSpec{types: []jpType{jpArrayString}},
		},
		handler: jpfJoin,
	},
	"reverse": functionEntry{
		name: "reverse",
		arguments: []argSpec{
			argSpec{types: []jpType{jpArray, jpString}},
		},
		handler: jpfReverse,
	},
	"to_array": functionEntry{
		name: "to_array",
		arguments: []argSpec{
			argSpec{types: []jpType{jpAny}},
		},
		handler: jpfToArray,
	},
	"to_string": functionEntry{
		name: "to_string",
		arguments: []argSpec{
			argSpec{types: []jpType{jpAny}},
		},
		handler: jpfToString,
	},
	"to_number": functionEntry{
		name: "to_number",
		arguments: []argSpec{
			argSpec{types: []jpType{jpAny}},
		},
		handler: jpfToNumber,
	},
	"not_null": functionEntry{
		name: "not_null",
		arguments: []argSpec{
			argSpec{types: []jpType{jpAny}, variadic: true},
		},
		handler: jpfNotNull,
	},
}

func (e *functionEntry) resolveArgs(arguments []interface{}) ([]interface{}, error) {
	if len(e.arguments) == 0 {
		return arguments, nil
	}
	if !e.arguments[len(e.arguments)-1].variadic {
		if len(e.arguments) != len(arguments) {
			return nil, errors.New("Incorrect number of args.")
		}
		for i, spec := range e.arguments {
			userArg := arguments[i]
			err := spec.typeCheck(userArg)
			if err != nil {
				return nil, err
			}
		}
		return arguments, nil
	}
	if len(arguments) < len(e.arguments) {
		return nil, errors.New("Invalid arity.")
	}
	return arguments, nil
}

func (a *argSpec) typeCheck(arg interface{}) error {
	for _, t := range a.types {
		switch t {
		case jpNumber:
			if _, ok := arg.(float64); ok {
				return nil
			}
		case jpString:
			if _, ok := arg.(string); ok {
				return nil
			}
		case jpArray:
			if _, ok := arg.([]interface{}); ok {
				return nil
			}
		case jpObject:
			if _, ok := arg.(map[string]interface{}); ok {
				return nil
			}
		case jpArrayNumber:
			if _, ok := jputil.ToArrayNum(arg); ok {
				return nil
			}
		case jpArrayString:
			if _, ok := jputil.ToArrayStr(arg); ok {
				return nil
			}
		case jpAny:
			return nil
		case jpExpref:
			if _, ok := arg.(ExpRef); ok {
				return nil
			}
		}
	}
	return errors.New(fmt.Sprintf("Invalid type for: %v, expected: %#v", arg, a.types))
}

func CallFunction(name string, arguments []interface{}) (interface{}, error) {
	entry, ok := functionTable[name]
	if !ok {
		return nil, errors.New("Unknown function: " + name)
	}
	resolvedArgs, err := entry.resolveArgs(arguments)
	if err != nil {
		return nil, err
	}
	return entry.handler(resolvedArgs)
}

func jpfAbs(arguments []interface{}) (interface{}, error) {
	num := arguments[0].(float64)
	return math.Abs(num), nil
}

func jpfLength(arguments []interface{}) (interface{}, error) {
	arg := arguments[0]
	if c, ok := arg.(string); ok {
		return float64(len(c)), nil
	} else if c, ok := arg.([]interface{}); ok {
		return float64(len(c)), nil
	} else if c, ok := arg.(map[string]interface{}); ok {
		return float64(len(c)), nil
	} else {
		return nil, errors.New("Could not compute length().")
	}
}

func jpfStartsWith(arguments []interface{}) (interface{}, error) {
	search := arguments[0].(string)
	prefix := arguments[1].(string)
	return strings.HasPrefix(search, prefix), nil
}

func jpfAvg(arguments []interface{}) (interface{}, error) {
	// We've already type checked the value so we can safely use
	// type assertions.
	args := arguments[0].([]interface{})
	length := float64(len(args))
	numerator := 0.0
	for _, n := range args {
		numerator += n.(float64)
	}
	return numerator / length, nil
}
func jpfCeil(arguments []interface{}) (interface{}, error) {
	val := arguments[0].(float64)
	return math.Ceil(val), nil
}
func jpfContains(arguments []interface{}) (interface{}, error) {
	search := arguments[0]
	el := arguments[1]
	if searchStr, ok := search.(string); ok {
		if elStr, ok := el.(string); ok {
			return strings.Index(searchStr, elStr) != -1, nil
		}
		return false, nil
	}
	// Otherwise this is a generic contains for []interface{}
	general := search.([]interface{})
	for _, item := range general {
		if item == el {
			return true, nil
		}
	}
	return false, nil
}
func jpfEndsWith(arguments []interface{}) (interface{}, error) {
	search := arguments[0].(string)
	suffix := arguments[1].(string)
	return strings.HasSuffix(search, suffix), nil
}
func jpfFloor(arguments []interface{}) (interface{}, error) {
	val := arguments[0].(float64)
	return math.Floor(val), nil
}
func jpfMax(arguments []interface{}) (interface{}, error) {
	if items, ok := jputil.ToArrayNum(arguments[0]); ok {
		if len(items) == 0 {
			return nil, nil
		}
		if len(items) == 1 {
			return items[0], nil
		}
		best := items[0]
		for _, item := range items[1:] {
			if item > best {
				best = item
			}
		}
		return best, nil
	} else {
		items, _ := jputil.ToArrayStr(arguments[0])
		if len(items) == 0 {
			return nil, nil
		}
		if len(items) == 1 {
			return items[0], nil
		}
		best := items[0]
		for _, item := range items[1:] {
			if item > best {
				best = item
			}
		}
		return best, nil
	}
}
func jpfMerge(arguments []interface{}) (interface{}, error) {
	final := make(map[string]interface{})
	for _, m := range arguments {
		mapped := m.(map[string]interface{})
		for key, value := range mapped {
			final[key] = value
		}
	}
	return final, nil
}
func jpfMaxBy(arguments []interface{}) (interface{}, error) {
	return nil, errors.New("Unimplemented")
}
func jpfSum(arguments []interface{}) (interface{}, error) {
	items, _ := jputil.ToArrayNum(arguments[0])
	sum := 0.0
	for _, item := range items {
		sum += item
	}
	return sum, nil
}

func jpfMin(arguments []interface{}) (interface{}, error) {
	if items, ok := jputil.ToArrayNum(arguments[0]); ok {
		if len(items) == 0 {
			return nil, nil
		}
		if len(items) == 1 {
			return items[0], nil
		}
		best := items[0]
		for _, item := range items[1:] {
			if item < best {
				best = item
			}
		}
		return best, nil
	} else {
		items, _ := jputil.ToArrayStr(arguments[0])
		if len(items) == 0 {
			return nil, nil
		}
		if len(items) == 1 {
			return items[0], nil
		}
		best := items[0]
		fmt.Printf("Best: %s\n", best)
		for _, item := range items[1:] {
			if item < best {
				fmt.Printf("New min: %s\n", item)
				best = item
			}
		}
		return best, nil
	}
}
func jpfMinBy(arguments []interface{}) (interface{}, error) {
	return nil, errors.New("Unimplemented min_by")
}
func jpfType(arguments []interface{}) (interface{}, error) {
	arg := arguments[0]
	if _, ok := arg.(float64); ok {
		return "number", nil
	}
	if _, ok := arg.(string); ok {
		return "string", nil
	}
	if _, ok := arg.([]interface{}); ok {
		return "array", nil
	}
	if _, ok := arg.(map[string]interface{}); ok {
		return "object", nil
	}
	if arg == nil {
		return "null", nil
	}
	if arg == true || arg == false {
		return "boolean", nil
	}
	return nil, errors.New("Unknown type")
}
func jpfKeys(arguments []interface{}) (interface{}, error) {
	arg := arguments[0].(map[string]interface{})
	collected := make([]interface{}, 0)
	for key, _ := range arg {
		collected = append(collected, key)
	}
	return collected, nil
}
func jpfValues(arguments []interface{}) (interface{}, error) {
	arg := arguments[0].(map[string]interface{})
	collected := make([]interface{}, 0)
	for _, value := range arg {
		collected = append(collected, value)
	}
	return collected, nil
}
func jpfSort(arguments []interface{}) (interface{}, error) {
	if items, ok := jputil.ToArrayNum(arguments[0]); ok {
		d := sort.Float64Slice(items)
		sort.Stable(d)
		final := make([]interface{}, len(d))
		for i, val := range d {
			final[i] = val
		}
		return final, nil
	} else {
		items, _ := jputil.ToArrayStr(arguments[0])
		d := sort.StringSlice(items)
		sort.Stable(d)
		final := make([]interface{}, len(d))
		for i, val := range d {
			final[i] = val
		}
		return final, nil
	}
}
func jpfSortBy(arguments []interface{}) (interface{}, error) {
	return nil, errors.New("Unimplemented sort_by")
}
func jpfJoin(arguments []interface{}) (interface{}, error) {
	sep := arguments[0].(string)
	// We can't just do arguments[1].([]string), we have to
	// manually convert each item to a string.
	arrayStr := make([]string, 0)
	for _, item := range arguments[1].([]interface{}) {
		arrayStr = append(arrayStr, item.(string))
	}
	return strings.Join(arrayStr, sep), nil
}
func jpfReverse(arguments []interface{}) (interface{}, error) {
	if s, ok := arguments[0].(string); ok {
		r := []rune(s)
		for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		return string(r), nil
	}
	items := arguments[0].([]interface{})
	length := len(items)
	reversed := make([]interface{}, length)
	for i, item := range items {
		reversed[length-(i+1)] = item
	}
	return reversed, nil
}
func jpfToArray(arguments []interface{}) (interface{}, error) {
	if _, ok := arguments[0].([]interface{}); ok {
		return arguments[0], nil
	}
	result := make([]interface{}, 1)
	result[0] = arguments[0]
	return result, nil
}
func jpfToString(arguments []interface{}) (interface{}, error) {
	if v, ok := arguments[0].(string); ok {
		return v, nil
	}
	result, err := json.Marshal(arguments[0])
	if err != nil {
		return nil, err
	}
	return string(result), nil
}
func jpfToNumber(arguments []interface{}) (interface{}, error) {
	arg := arguments[0]
	if v, ok := arg.(float64); ok {
		return v, nil
	}
	if v, ok := arg.(string); ok {
		conv, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, nil
		}
		return conv, nil
	}
	if _, ok := arg.([]interface{}); ok {
		return nil, nil
	}
	if _, ok := arg.(map[string]interface{}); ok {
		return nil, nil
	}
	if arg == nil {
		return nil, nil
	}
	if arg == true || arg == false {
		return nil, nil
	}
	return nil, errors.New("Unknown type")
}
func jpfNotNull(arguments []interface{}) (interface{}, error) {
	for _, arg := range arguments {
		if arg != nil {
			return arg, nil
		}
	}
	return nil, nil
}
