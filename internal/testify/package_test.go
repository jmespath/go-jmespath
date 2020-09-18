package testify

import (
	"testing"

	"github.com/jmespath/go-jmespath/internal/testify/assert"
)

func TestImports(t *testing.T) {
	if assert.Equal(t, 1, 1) != true {
		t.Error("Something is wrong.")
	}
}
