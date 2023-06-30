package jmespath

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kyverno/go-jmespath/internal/testify/assert"
)

func TestValidUncompiledExpressionSearches(t *testing.T) {
	assert := assert.New(t)
	var j = []byte(`{"foo": {"bar": {"baz": [0, 1, 2, 3, 4]}}}`)
	var d interface{}
	err := json.Unmarshal(j, &d)
	assert.Nil(err)
	result, err := Search("foo.bar.baz[2]", d)
	assert.Nil(err)
	assert.Equal(2.0, result)
}

func TestValidPrecompiledExpressionSearches(t *testing.T) {
	assert := assert.New(t)
	data := make(map[string]interface{})
	data["foo"] = "bar"
	precompiled, err := Compile("foo")
	assert.Nil(err)
	result, err := precompiled.Search(data)
	assert.Nil(err)
	assert.Equal("bar", result)
}

func TestInvalidPrecompileErrors(t *testing.T) {
	assert := assert.New(t)
	_, err := Compile("not a valid expression")
	assert.NotNil(err)
}

func TestInvalidMustCompilePanics(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()
	MustCompile("not a valid expression")
}

func TestCustomFunction(t *testing.T) {
	assert := assert.New(t)
	data := make(map[string]interface{})
	data["foo"] = "BAR"
	precompiled, err := Compile("to_lower(foo)")
	precompiled.Register(FunctionEntry{
		Name: "to_lower",
		Arguments: []ArgSpec{
			{Types: []JpType{JpString}},
		},
		Handler: func(arguments []interface{}) (interface{}, error) {
			s := arguments[0].(string)
			return strings.ToLower(s), nil
		},
	})
	assert.Nil(err)
	result, err := precompiled.Search(data)
	assert.Nil(err)
	assert.Equal("bar", result)
}
