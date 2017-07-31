package jmespath

import (
	// "fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPointerObj(t *testing.T) {
	assert := assert.New(t)
	testval := float64(10)
	testvalPtr := &testval
	obj := map[string]interface{} {
		"keyPtr": testvalPtr,
	}
	result, err := Search("keyPtr==`10`", obj)
	assert.True(result.(bool))
	assert.Nil(err)
}

func TestPointerChain(t *testing.T) {
	assert := assert.New(t)
	v1 := interface{}(float64(10))
	v2 := &v1
	v3 := interface{}(v2)
	v4 := &v3
	obj := map[string]interface{} {
		"keyPtr": v4,
	}
	result, err := Search("keyPtr==`10`", obj)
	assert.True(result.(bool))
	assert.Nil(err)
}

func TestPointerObscured(t *testing.T) {
	assert := assert.New(t)
	obj := map[string]interface{} {
		"keyPtr": float64(10),
	}
	result, err := Search("keyPtr==`10`", interface{}(&obj))
	assert.True(result.(bool))
	assert.Nil(err)
}
