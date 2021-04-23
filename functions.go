package jmespath

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type JpFunction func(arguments []interface{}) (interface{}, error)

type JpType string

const (
	JpUnknown     JpType = "unknown"
	JpNumber      JpType = "number"
	JpString      JpType = "string"
	JpArray       JpType = "array"
	JpObject      JpType = "object"
	JpArrayNumber JpType = "array[number]"
	JpArrayString JpType = "array[string]"
	JpExpref      JpType = "expref"
	JpAny         JpType = "any"
)

type FunctionEntry struct {
	Name      string
	Arguments []ArgSpec
	Handler   JpFunction
	HasExpRef bool
}

type ArgSpec struct {
	Types    []JpType
	variadic bool
}

type byExprString struct {
	intr     *treeInterpreter
	node     ASTNode
	items    []interface{}
	hasError bool
}

func (a *byExprString) Len() int {
	return len(a.items)
}
func (a *byExprString) Swap(i, j int) {
	a.items[i], a.items[j] = a.items[j], a.items[i]
}
func (a *byExprString) Less(i, j int) bool {
	first, err := a.intr.Execute(a.node, a.items[i])
	if err != nil {
		a.hasError = true
		// Return a dummy value.
		return true
	}
	ith, ok := first.(string)
	if !ok {
		a.hasError = true
		return true
	}
	second, err := a.intr.Execute(a.node, a.items[j])
	if err != nil {
		a.hasError = true
		// Return a dummy value.
		return true
	}
	jth, ok := second.(string)
	if !ok {
		a.hasError = true
		return true
	}
	return ith < jth
}

type byExprFloat struct {
	intr     *treeInterpreter
	node     ASTNode
	items    []interface{}
	hasError bool
}

func (a *byExprFloat) Len() int {
	return len(a.items)
}
func (a *byExprFloat) Swap(i, j int) {
	a.items[i], a.items[j] = a.items[j], a.items[i]
}
func (a *byExprFloat) Less(i, j int) bool {
	first, err := a.intr.Execute(a.node, a.items[i])
	if err != nil {
		a.hasError = true
		// Return a dummy value.
		return true
	}
	ith, ok := first.(float64)
	if !ok {
		a.hasError = true
		return true
	}
	second, err := a.intr.Execute(a.node, a.items[j])
	if err != nil {
		a.hasError = true
		// Return a dummy value.
		return true
	}
	jth, ok := second.(float64)
	if !ok {
		a.hasError = true
		return true
	}
	return ith < jth
}

type functionCaller struct {
	functionTable map[string]FunctionEntry
}

func newFunctionCaller() *functionCaller {
	caller := &functionCaller{}
	caller.functionTable = map[string]FunctionEntry{
		"length": {
			Name: "length",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString, JpArray, JpObject}},
			},
			Handler: jpfLength,
		},
		"starts_with": {
			Name: "starts_with",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfStartsWith,
		},
		"abs": {
			Name: "abs",
			Arguments: []ArgSpec{
				{Types: []JpType{JpNumber}},
			},
			Handler: jpfAbs,
		},
		"avg": {
			Name: "avg",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArrayNumber}},
			},
			Handler: jpfAvg,
		},
		"ceil": {
			Name: "ceil",
			Arguments: []ArgSpec{
				{Types: []JpType{JpNumber}},
			},
			Handler: jpfCeil,
		},
		"contains": {
			Name: "contains",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArray, JpString}},
				{Types: []JpType{JpAny}},
			},
			Handler: jpfContains,
		},
		"ends_with": {
			Name: "ends_with",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfEndsWith,
		},
		"floor": {
			Name: "floor",
			Arguments: []ArgSpec{
				{Types: []JpType{JpNumber}},
			},
			Handler: jpfFloor,
		},
		"map": {
			Name: "amp",
			Arguments: []ArgSpec{
				{Types: []JpType{JpExpref}},
				{Types: []JpType{JpArray}},
			},
			Handler:   jpfMap,
			HasExpRef: true,
		},
		"max": {
			Name: "max",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArrayNumber, JpArrayString}},
			},
			Handler: jpfMax,
		},
		"merge": {
			Name: "merge",
			Arguments: []ArgSpec{
				{Types: []JpType{JpObject}, variadic: true},
			},
			Handler: jpfMerge,
		},
		"max_by": {
			Name: "max_by",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArray}},
				{Types: []JpType{JpExpref}},
			},
			Handler:   jpfMaxBy,
			HasExpRef: true,
		},
		"sum": {
			Name: "sum",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArrayNumber}},
			},
			Handler: jpfSum,
		},
		"min": {
			Name: "min",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArrayNumber, JpArrayString}},
			},
			Handler: jpfMin,
		},
		"min_by": {
			Name: "min_by",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArray}},
				{Types: []JpType{JpExpref}},
			},
			Handler:   jpfMinBy,
			HasExpRef: true,
		},
		"type": {
			Name: "type",
			Arguments: []ArgSpec{
				{Types: []JpType{JpAny}},
			},
			Handler: jpfType,
		},
		"keys": {
			Name: "keys",
			Arguments: []ArgSpec{
				{Types: []JpType{JpObject}},
			},
			Handler: jpfKeys,
		},
		"values": {
			Name: "values",
			Arguments: []ArgSpec{
				{Types: []JpType{JpObject}},
			},
			Handler: jpfValues,
		},
		"sort": {
			Name: "sort",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArrayString, JpArrayNumber}},
			},
			Handler: jpfSort,
		},
		"sort_by": {
			Name: "sort_by",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArray}},
				{Types: []JpType{JpExpref}},
			},
			Handler:   jpfSortBy,
			HasExpRef: true,
		},
		"join": {
			Name: "join",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpArrayString}},
			},
			Handler: jpfJoin,
		},
		"reverse": {
			Name: "reverse",
			Arguments: []ArgSpec{
				{Types: []JpType{JpArray, JpString}},
			},
			Handler: jpfReverse,
		},
		"to_array": {
			Name: "to_array",
			Arguments: []ArgSpec{
				{Types: []JpType{JpAny}},
			},
			Handler: jpfToArray,
		},
		"to_string": {
			Name: "to_string",
			Arguments: []ArgSpec{
				{Types: []JpType{JpAny}},
			},
			Handler: jpfToString,
		},
		"to_number": {
			Name: "to_number",
			Arguments: []ArgSpec{
				{Types: []JpType{JpAny}},
			},
			Handler: jpfToNumber,
		},
		"not_null": {
			Name: "not_null",
			Arguments: []ArgSpec{
				{Types: []JpType{JpAny}, variadic: true},
			},
			Handler: jpfNotNull,
		},
	}
	return caller
}

func (e *FunctionEntry) resolveArgs(arguments []interface{}) ([]interface{}, error) {
	if len(e.Arguments) == 0 {
		return arguments, nil
	}
	if !e.Arguments[len(e.Arguments)-1].variadic {
		if len(e.Arguments) != len(arguments) {
			return nil, errors.New("incorrect number of args")
		}
		for i, spec := range e.Arguments {
			userArg := arguments[i]
			err := spec.typeCheck(userArg)
			if err != nil {
				return nil, err
			}
		}
		return arguments, nil
	}
	if len(arguments) < len(e.Arguments) {
		return nil, errors.New("Invalid arity.")
	}
	return arguments, nil
}

func (a *ArgSpec) typeCheck(arg interface{}) error {
	for _, t := range a.Types {
		switch t {
		case JpNumber:
			if _, ok := arg.(float64); ok {
				return nil
			}
		case JpString:
			if _, ok := arg.(string); ok {
				return nil
			}
		case JpArray:
			if isSliceType(arg) {
				return nil
			}
		case JpObject:
			if _, ok := arg.(map[string]interface{}); ok {
				return nil
			}
		case JpArrayNumber:
			if _, ok := toArrayNum(arg); ok {
				return nil
			}
		case JpArrayString:
			if _, ok := toArrayStr(arg); ok {
				return nil
			}
		case JpAny:
			return nil
		case JpExpref:
			if _, ok := arg.(expRef); ok {
				return nil
			}
		}
	}
	return fmt.Errorf("Invalid type for: %v, expected: %#v", arg, a.Types)
}

func (f *functionCaller) CallFunction(name string, arguments []interface{}, intr *treeInterpreter) (interface{}, error) {
	entry, ok := f.functionTable[name]
	if !ok {
		return nil, errors.New("unknown function: " + name)
	}
	resolvedArgs, err := entry.resolveArgs(arguments)
	if err != nil {
		return nil, err
	}
	if entry.HasExpRef {
		var extra []interface{}
		extra = append(extra, intr)
		resolvedArgs = append(extra, resolvedArgs...)
	}
	return entry.Handler(resolvedArgs)
}

func jpfAbs(arguments []interface{}) (interface{}, error) {
	num := arguments[0].(float64)
	return math.Abs(num), nil
}

func jpfLength(arguments []interface{}) (interface{}, error) {
	arg := arguments[0]
	if c, ok := arg.(string); ok {
		return float64(utf8.RuneCountInString(c)), nil
	} else if isSliceType(arg) {
		v := reflect.ValueOf(arg)
		return float64(v.Len()), nil
	} else if c, ok := arg.(map[string]interface{}); ok {
		return float64(len(c)), nil
	}
	return nil, errors.New("could not compute length()")
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
func jpfMap(arguments []interface{}) (interface{}, error) {
	intr := arguments[0].(*treeInterpreter)
	exp := arguments[1].(expRef)
	node := exp.ref
	arr := arguments[2].([]interface{})
	mapped := make([]interface{}, 0, len(arr))
	for _, value := range arr {
		current, err := intr.Execute(node, value)
		if err != nil {
			if _, ok := err.(NotFoundError); !ok {
				return nil, err
			}
		}
		mapped = append(mapped, current)
	}
	return mapped, nil
}
func jpfMax(arguments []interface{}) (interface{}, error) {
	if items, ok := toArrayNum(arguments[0]); ok {
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
	// Otherwise we're dealing with a max() of strings.
	items, _ := toArrayStr(arguments[0])
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
	intr := arguments[0].(*treeInterpreter)
	arr := arguments[1].([]interface{})
	exp := arguments[2].(expRef)
	node := exp.ref
	if len(arr) == 0 {
		return nil, nil
	} else if len(arr) == 1 {
		return arr[0], nil
	}
	start, err := intr.Execute(node, arr[0])
	if err != nil {
		if _, ok := err.(NotFoundError); !ok {
			return nil, err
		}
	}
	switch t := start.(type) {
	case float64:
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				if _, ok := err.(NotFoundError); !ok {
					return nil, err
				}
			}
			current, ok := result.(float64)
			if !ok {
				return nil, errors.New("invalid type, must be number")
			}
			if current > bestVal {
				bestVal = current
				bestItem = item
			}
		}
		return bestItem, nil
	case string:
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				if _, ok := err.(NotFoundError); !ok {
					return nil, err
				}
			}
			current, ok := result.(string)
			if !ok {
				return nil, errors.New("invalid type, must be string")
			}
			if current > bestVal {
				bestVal = current
				bestItem = item
			}
		}
		return bestItem, nil
	default:
		return nil, errors.New("invalid type, must be number of string")
	}
}
func jpfSum(arguments []interface{}) (interface{}, error) {
	items, _ := toArrayNum(arguments[0])
	sum := 0.0
	for _, item := range items {
		sum += item
	}
	return sum, nil
}

func jpfMin(arguments []interface{}) (interface{}, error) {
	if items, ok := toArrayNum(arguments[0]); ok {
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
	}
	items, _ := toArrayStr(arguments[0])
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
}

func jpfMinBy(arguments []interface{}) (interface{}, error) {
	intr := arguments[0].(*treeInterpreter)
	arr := arguments[1].([]interface{})
	exp := arguments[2].(expRef)
	node := exp.ref
	if len(arr) == 0 {
		return nil, nil
	} else if len(arr) == 1 {
		return arr[0], nil
	}
	start, err := intr.Execute(node, arr[0])
	if err != nil {
		if _, ok := err.(NotFoundError); !ok {
			return nil, err
		}
	}
	if t, ok := start.(float64); ok {
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				if _, ok := err.(NotFoundError); !ok {
					return nil, err
				}
			}
			current, ok := result.(float64)
			if !ok {
				return nil, errors.New("invalid type, must be number")
			}
			if current < bestVal {
				bestVal = current
				bestItem = item
			}
		}
		return bestItem, nil
	} else if t, ok := start.(string); ok {
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				if _, ok := err.(NotFoundError); !ok {
					return nil, err
				}
			}
			current, ok := result.(string)
			if !ok {
				return nil, errors.New("invalid type, must be string")
			}
			if current < bestVal {
				bestVal = current
				bestItem = item
			}
		}
		return bestItem, nil
	} else {
		return nil, errors.New("invalid type, must be number of string")
	}
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
	return nil, errors.New("unknown type")
}
func jpfKeys(arguments []interface{}) (interface{}, error) {
	arg := arguments[0].(map[string]interface{})
	collected := make([]interface{}, 0, len(arg))
	for key := range arg {
		collected = append(collected, key)
	}
	return collected, nil
}
func jpfValues(arguments []interface{}) (interface{}, error) {
	arg := arguments[0].(map[string]interface{})
	collected := make([]interface{}, 0, len(arg))
	for _, value := range arg {
		collected = append(collected, value)
	}
	return collected, nil
}
func jpfSort(arguments []interface{}) (interface{}, error) {
	if items, ok := toArrayNum(arguments[0]); ok {
		d := sort.Float64Slice(items)
		sort.Stable(d)
		final := make([]interface{}, len(d))
		for i, val := range d {
			final[i] = val
		}
		return final, nil
	}
	// Otherwise we're dealing with sort()'ing strings.
	items, _ := toArrayStr(arguments[0])
	d := sort.StringSlice(items)
	sort.Stable(d)
	final := make([]interface{}, len(d))
	for i, val := range d {
		final[i] = val
	}
	return final, nil
}
func jpfSortBy(arguments []interface{}) (interface{}, error) {
	intr := arguments[0].(*treeInterpreter)
	arr := arguments[1].([]interface{})
	exp := arguments[2].(expRef)
	node := exp.ref
	if len(arr) == 0 {
		return arr, nil
	} else if len(arr) == 1 {
		return arr, nil
	}
	start, err := intr.Execute(node, arr[0])
	if err != nil {
		if _, ok := err.(NotFoundError); !ok {
			return nil, err
		}
	}
	if _, ok := start.(float64); ok {
		sortable := &byExprFloat{intr, node, arr, false}
		sort.Stable(sortable)
		if sortable.hasError {
			return nil, errors.New("error in sort_by comparison")
		}
		return arr, nil
	} else if _, ok := start.(string); ok {
		sortable := &byExprString{intr, node, arr, false}
		sort.Stable(sortable)
		if sortable.hasError {
			return nil, errors.New("error in sort_by comparison")
		}
		return arr, nil
	} else {
		return nil, errors.New("invalid type, must be number of string")
	}
}
func jpfJoin(arguments []interface{}) (interface{}, error) {
	sep := arguments[0].(string)
	// We can't just do arguments[1].([]string), we have to
	// manually convert each item to a string.
	arrayStr := []string{}
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
	return arguments[:1:1], nil
}
func jpfToString(arguments []interface{}) (interface{}, error) {
	if v, ok := arguments[0].(string); ok {
		return v, nil
	}
	result, err := json.Marshal(arguments[0])
	if err != nil {
		if _, ok := err.(NotFoundError); !ok {
			return nil, err
		}
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
	return nil, errors.New("unknown type")
}
func jpfNotNull(arguments []interface{}) (interface{}, error) {
	for _, arg := range arguments {
		if arg != nil {
			return arg, nil
		}
	}
	return nil, nil
}
