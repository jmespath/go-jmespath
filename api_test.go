package jmespath

import (
	"encoding/json"
	"testing"

	"github.com/jmespath/go-jmespath/internal/testify/assert"
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

func TestToEntries(t *testing.T) {
	assert := assert.New(t)
	data := make(map[string]interface{})
	data["foo"] = "bar"
	data["baz"] = 42
	result, err := Search("to_entries(@)", data)
	assert.Nil(err)

	entries, ok := result.([]interface{})
	assert.True(ok)
	assert.Equal(2, len(entries))

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]interface{})
		assert.True(ok)
		_, hasKey := entryMap["key"]
		_, hasValue := entryMap["value"]
		assert.True(hasKey)
		assert.True(hasValue)
	}
}
