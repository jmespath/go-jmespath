package jputil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlicePositiveStep(t *testing.T) {
	assert := assert.New(t)
	input := make([]interface{}, 5)
	input[0] = 0
	input[1] = 1
	input[2] = 2
	input[3] = 3
	input[4] = 4
	result, err := Slice(input, []SliceParam{SliceParam{0, true}, SliceParam{3, true}, SliceParam{1, true}})
	assert.Nil(err)
	assert.Equal(input[:3], result)
}
