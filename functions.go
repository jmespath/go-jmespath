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
	"unicode"
	"unicode/utf8"
)

type jpFunction func(arguments []interface{}) (interface{}, error)

type jpType string

const (
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
	hasExpRef bool
}

type argSpec struct {
	types    []jpType
	variadic bool
	optional bool
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
	functionTable map[string]functionEntry
}

func newFunctionCaller() *functionCaller {
	caller := &functionCaller{}
	caller.functionTable = map[string]functionEntry{
		"abs": {
			name: "abs",
			arguments: []argSpec{
				{types: []jpType{jpNumber}},
			},
			handler: jpfAbs,
		},
		"avg": {
			name: "avg",
			arguments: []argSpec{
				{types: []jpType{jpArrayNumber}},
			},
			handler: jpfAvg,
		},
		"ceil": {
			name: "ceil",
			arguments: []argSpec{
				{types: []jpType{jpNumber}},
			},
			handler: jpfCeil,
		},
		"contains": {
			name: "contains",
			arguments: []argSpec{
				{types: []jpType{jpArray, jpString}},
				{types: []jpType{jpAny}},
			},
			handler: jpfContains,
		},
		"ends_with": {
			name: "ends_with",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpString}},
			},
			handler: jpfEndsWith,
		},
		"find_first": {
			name: "find_first",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpString}},
				{types: []jpType{jpNumber}, optional: true},
				{types: []jpType{jpNumber}, optional: true},
			},
			handler: jpfFindFirst,
		},
		"find_last": {
			name: "find_last",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpString}},
				{types: []jpType{jpNumber}, optional: true},
				{types: []jpType{jpNumber}, optional: true},
			},
			handler: jpfFindLast,
		},
		"floor": {
			name: "floor",
			arguments: []argSpec{
				{types: []jpType{jpNumber}},
			},
			handler: jpfFloor,
		},
		"join": {
			name: "join",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpArrayString}},
			},
			handler: jpfJoin,
		},
		"keys": {
			name: "keys",
			arguments: []argSpec{
				{types: []jpType{jpObject}},
			},
			handler: jpfKeys,
		},
		"length": {
			name: "length",
			arguments: []argSpec{
				{types: []jpType{jpString, jpArray, jpObject}},
			},
			handler: jpfLength,
		},
		"lower": {
			name: "lower",
			arguments: []argSpec{
				{types: []jpType{jpString}},
			},
			handler: jpfLower,
		},
		"map": {
			name: "amp",
			arguments: []argSpec{
				{types: []jpType{jpExpref}},
				{types: []jpType{jpArray}},
			},
			handler:   jpfMap,
			hasExpRef: true,
		},
		"max": {
			name: "max",
			arguments: []argSpec{
				{types: []jpType{jpArrayNumber, jpArrayString}},
			},
			handler: jpfMax,
		},
		"max_by": {
			name: "max_by",
			arguments: []argSpec{
				{types: []jpType{jpArray}},
				{types: []jpType{jpExpref}},
			},
			handler:   jpfMaxBy,
			hasExpRef: true,
		},
		"merge": {
			name: "merge",
			arguments: []argSpec{
				{types: []jpType{jpObject}, variadic: true},
			},
			handler: jpfMerge,
		},
		"min": {
			name: "min",
			arguments: []argSpec{
				{types: []jpType{jpArrayNumber, jpArrayString}},
			},
			handler: jpfMin,
		},
		"min_by": {
			name: "min_by",
			arguments: []argSpec{
				{types: []jpType{jpArray}},
				{types: []jpType{jpExpref}},
			},
			handler:   jpfMinBy,
			hasExpRef: true,
		},
		"not_null": {
			name: "not_null",
			arguments: []argSpec{
				{types: []jpType{jpAny}, variadic: true},
			},
			handler: jpfNotNull,
		},
		"pad_left": {
			name: "pad_left",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpNumber}},
				{types: []jpType{jpString}, optional: true},
			},
			handler: jpfPadLeft,
		},
		"pad_right": {
			name: "pad_right",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpNumber}},
				{types: []jpType{jpString}, optional: true},
			},
			handler: jpfPadRight,
		},
		"replace": {
			name: "replace",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpString}},
				{types: []jpType{jpString}},
				{types: []jpType{jpNumber}, optional: true},
			},
			handler: jpfReplace,
		},
		"reverse": {
			name: "reverse",
			arguments: []argSpec{
				{types: []jpType{jpArray, jpString}},
			},
			handler: jpfReverse,
		},
		"sort": {
			name: "sort",
			arguments: []argSpec{
				{types: []jpType{jpArrayString, jpArrayNumber}},
			},
			handler: jpfSort,
		},
		"sort_by": {
			name: "sort_by",
			arguments: []argSpec{
				{types: []jpType{jpArray}},
				{types: []jpType{jpExpref}},
			},
			handler:   jpfSortBy,
			hasExpRef: true,
		},
		"starts_with": {
			name: "starts_with",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpString}},
			},
			handler: jpfStartsWith,
		},
		"sum": {
			name: "sum",
			arguments: []argSpec{
				{types: []jpType{jpArrayNumber}},
			},
			handler: jpfSum,
		},
		"to_array": {
			name: "to_array",
			arguments: []argSpec{
				{types: []jpType{jpAny}},
			},
			handler: jpfToArray,
		},
		"to_number": {
			name: "to_number",
			arguments: []argSpec{
				{types: []jpType{jpAny}},
			},
			handler: jpfToNumber,
		},
		"to_string": {
			name: "to_string",
			arguments: []argSpec{
				{types: []jpType{jpAny}},
			},
			handler: jpfToString,
		},
		"trim": {
			name: "trim",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpString}, optional: true},
			},
			handler: jpfTrim,
		},
		"trim_left": {
			name: "trim_left",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpString}, optional: true},
			},
			handler: jpfTrimLeft,
		},
		"trim_right": {
			name: "trim_right",
			arguments: []argSpec{
				{types: []jpType{jpString}},
				{types: []jpType{jpString}, optional: true},
			},
			handler: jpfTrimRight,
		},
		"type": {
			name: "type",
			arguments: []argSpec{
				{types: []jpType{jpAny}},
			},
			handler: jpfType,
		},
		"upper": {
			name: "upper",
			arguments: []argSpec{
				{types: []jpType{jpString}},
			},
			handler: jpfUpper,
		},
		"values": {
			name: "values",
			arguments: []argSpec{
				{types: []jpType{jpObject}},
			},
			handler: jpfValues,
		},
	}
	return caller
}

func (e *functionEntry) resolveArgs(arguments []interface{}) ([]interface{}, error) {
	if len(e.arguments) == 0 {
		return arguments, nil
	}

	variadic := isVariadic(e.arguments)
	minExpected := getMinExpected(e.arguments)
	maxExpected, hasMax := getMaxExpected(e.arguments)
	count := len(arguments)

	if count < minExpected {
		return nil, notEnoughArgumentsSupplied(e.name, count, minExpected, variadic)
	}

	if hasMax && count > maxExpected {
		return nil, tooManyArgumentsSupplied(e.name, count, maxExpected)
	}

	for i, spec := range e.arguments {
		if !spec.optional || i <= len(arguments)-1 {
			userArg := arguments[i]
			err := spec.typeCheck(userArg)
			if err != nil {
				return nil, err
			}
		}
	}
	lastIndex := len(e.arguments) - 1
	lastArg := e.arguments[lastIndex]
	if lastArg.variadic {
		for i := len(e.arguments) - 1; i < len(arguments); i++ {
			userArg := arguments[i]
			err := lastArg.typeCheck(userArg)
			if err != nil {
				return nil, err
			}
		}
	}
	return arguments, nil
}

func isVariadic(arguments []argSpec) bool {
	for _, spec := range arguments {
		if spec.variadic {
			return true
		}
	}
	return false
}
func getMinExpected(arguments []argSpec) int {
	expected := 0
	for _, spec := range arguments {
		if !spec.optional {
			expected += 1
		}
	}
	return expected
}
func getMaxExpected(arguments []argSpec) (int, bool) {
	if isVariadic(arguments) {
		return 0, false
	} else {
		return int(len(arguments)), true
	}
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
			if isSliceType(arg) {
				return nil
			}
		case jpObject:
			if _, ok := arg.(map[string]interface{}); ok {
				return nil
			}
		case jpArrayNumber:
			if _, ok := toArrayNum(arg); ok {
				return nil
			}
		case jpArrayString:
			if _, ok := toArrayStr(arg); ok {
				return nil
			}
		case jpAny:
			return nil
		case jpExpref:
			if _, ok := arg.(expRef); ok {
				return nil
			}
		}
	}
	return fmt.Errorf("Invalid type for: %v, expected: %#v", arg, a.types)
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
	if entry.hasExpRef {
		var extra []interface{}
		extra = append(extra, intr)
		resolvedArgs = append(extra, resolvedArgs...)
	}
	return entry.handler(resolvedArgs)
}

func jpfAbs(arguments []interface{}) (interface{}, error) {
	num := arguments[0].(float64)
	return math.Abs(num), nil
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
			return strings.Contains(searchStr, elStr), nil
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

func jpfFindImpl(name string, arguments []interface{}, find func(s string, substr string) int) (interface{}, error) {
	subject := arguments[0].(string)
	substr := arguments[1].(string)

	if len(subject) == 0 || len(substr) == 0 {
		return nil, nil
	}

	start := 0
	startSpecified := len(arguments) > 2
	if startSpecified {
		num, ok := toInteger(arguments[2])
		if !ok {
			return nil, notAnInteger(name, "start")
		}
		start = max(0, num)
	}
	end := len(subject)
	endSpecified := len(arguments) > 3
	if endSpecified {
		num, ok := toInteger(arguments[3])
		if !ok {
			return nil, notAnInteger(name, "end")
		}
		end = min(num, len(subject))
	}

	offset := find(subject[start:end], substr)

	if offset == -1 {
		return nil, nil
	}

	return float64(start + offset), nil
}

func jpfFindFirst(arguments []interface{}) (interface{}, error) {
	return jpfFindImpl("find_first", arguments, strings.Index)
}
func jpfFindLast(arguments []interface{}) (interface{}, error) {
	return jpfFindImpl("find_last", arguments, strings.LastIndex)
}

func jpfFloor(arguments []interface{}) (interface{}, error) {
	val := arguments[0].(float64)
	return math.Floor(val), nil
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

func jpfKeys(arguments []interface{}) (interface{}, error) {
	arg := arguments[0].(map[string]interface{})
	collected := make([]interface{}, 0, len(arg))
	for key := range arg {
		collected = append(collected, key)
	}
	return collected, nil
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

func jpfLower(arguments []interface{}) (interface{}, error) {
	return strings.ToLower(arguments[0].(string)), nil
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
			return nil, err
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
		return nil, err
	}
	switch t := start.(type) {
	case float64:
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				return nil, err
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
				return nil, err
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
		return nil, err
	}
	if t, ok := start.(float64); ok {
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				return nil, err
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
				return nil, err
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

func jpfNotNull(arguments []interface{}) (interface{}, error) {
	for _, arg := range arguments {
		if arg != nil {
			return arg, nil
		}
	}
	return nil, nil
}

func jpfPadImpl(
	name string,
	arguments []interface{},
	pad func(s string, width int, pad string) string) (interface{}, error) {

	s := arguments[0].(string)
	width, ok := toPositiveInteger(arguments[1])
	if !ok {
		return nil, notAPositiveInteger(name, "width")
	}
	chars := " "
	if len(arguments) > 2 {
		chars = arguments[2].(string)
		if len(chars) > 1 {
			return nil, errors.New(fmt.Sprintf("invalid value, the function '%s' expects its 'pad' argument to be a string of length 1", name))
		}
	}

	return pad(s, width, chars), nil
}

func jpfPadLeft(arguments []interface{}) (interface{}, error) {
	return jpfPadImpl("pad_left", arguments, padLeft)
}
func jpfPadRight(arguments []interface{}) (interface{}, error) {
	return jpfPadImpl("pad_right", arguments, padRight)
}
func padLeft(s string, width int, pad string) string {
	length := max(0, width-len(s))
	padding := strings.Repeat(pad, length)
	result := fmt.Sprintf("%s%s", padding, s)
	return result
}
func padRight(s string, width int, pad string) string {
	length := max(0, width-len(s))
	padding := strings.Repeat(pad, length)
	result := fmt.Sprintf("%s%s", s, padding)
	return result
}

func jpfReplace(arguments []interface{}) (interface{}, error) {
	subject := arguments[0].(string)
	old := arguments[1].(string)
	new := arguments[2].(string)
	count := -1
	if len(arguments) > 3 {
		num, ok := toPositiveInteger(arguments[3])
		if !ok {
			return nil, notAPositiveInteger("replace", "count")
		}
		count = num
	}

	return strings.Replace(subject, old, new, count), nil
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
		return nil, err
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

func jpfStartsWith(arguments []interface{}) (interface{}, error) {
	search := arguments[0].(string)
	prefix := arguments[1].(string)
	return strings.HasPrefix(search, prefix), nil
}

func jpfSum(arguments []interface{}) (interface{}, error) {
	items, _ := toArrayNum(arguments[0])
	sum := 0.0
	for _, item := range items {
		sum += item
	}
	return sum, nil
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
	return nil, errors.New("unknown type")
}

func jpfTrimImpl(
	arguments []interface{},
	trimSpace func(s string, predicate func(r rune) bool) string,
	trim func(s string, cutset string) string) (interface{}, error) {

	s := arguments[0].(string)
	cutset := ""
	if len(arguments) > 1 {
		cutset = arguments[1].(string)
	}

	if len(cutset) == 0 {
		return trimSpace(s, unicode.IsSpace), nil
	}
	return trim(s, cutset), nil
}
func jpfTrim(arguments []interface{}) (interface{}, error) {
	return jpfTrimImpl(arguments, strings.TrimFunc, strings.Trim)
}
func jpfTrimLeft(arguments []interface{}) (interface{}, error) {
	return jpfTrimImpl(arguments, strings.TrimLeftFunc, strings.TrimLeft)
}
func jpfTrimRight(arguments []interface{}) (interface{}, error) {
	return jpfTrimImpl(arguments, strings.TrimRightFunc, strings.TrimRight)
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

func jpfUpper(arguments []interface{}) (interface{}, error) {
	return strings.ToUpper(arguments[0].(string)), nil
}

func jpfValues(arguments []interface{}) (interface{}, error) {
	arg := arguments[0].(map[string]interface{})
	collected := make([]interface{}, 0, len(arg))
	for _, value := range arg {
		collected = append(collected, value)
	}
	return collected, nil
}
