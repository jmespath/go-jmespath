package jmespath

import (
	"github.com/jmespath/go-jmespath/internal/testify/assert"
	"strings"
	"testing"
)

func TestUserDefinedFunctions(t *testing.T) {
	searcher, err := Compile("icontains(@, 'Bar')")
	if !assert.NoError(t, err) {
		return
	}

	err = searcher.RegisterFunction("icontains", "string|array[string],string", false, func(args []interface{}) (interface{}, error) {
		needle := strings.ToLower(args[1].(string))
		if haystack, ok := args[0].(string); ok {
			return strings.Contains(strings.ToLower(haystack), needle), nil
		}
		array, _ := toArrayStr(args[0])
		for _, el := range array {
			if strings.ToLower(el) == needle {
				return true, nil
			}
		}
		return false, nil
	})
	if !assert.NoError(t, err) {
		return
	}

	actual, err := searcher.Search("fooBARbaz")
	if assert.NoError(t, err) {
		assert.Equal(t, true, actual)
	}

	actual, err = searcher.Search([]interface{}{"foo", "BAR", "baz"})
	if assert.NoError(t, err) {
		assert.Equal(t, true, actual)
	}
}

func TestExpressionEvaluator(t *testing.T) {
	searcher, err := Compile("my_map(&id, @)")
	if !assert.NoError(t, err) {
		return
	}

	err = searcher.RegisterFunction("my_map", "expref,array", false, func(args []interface{}) (interface{}, error) {
		evaluator := NewExpressionEvaluator(args[0], args[1])
		arr := args[2].([]interface{})
		mapped := make([]interface{}, 0, len(arr))
		for _, value := range arr {
			current, err := evaluator(value)
			if err != nil {
				return nil, err
			}
			mapped = append(mapped, current)
		}
		return mapped, nil
	})

	if !assert.NoError(t, err) {
		return
	}

	actual, err := searcher.Search([]interface{}{
		map[string]interface{}{
			"id":    1,
			"value": "a",
		},
		map[string]interface{}{
			"id":    2,
			"value": "b",
		},
		map[string]interface{}{
			"id":    3,
			"value": "c",
		},
	})
	if assert.NoError(t, err) {
		assert.Equal(t, []interface{}{1, 2, 3}, actual)
	}
}
