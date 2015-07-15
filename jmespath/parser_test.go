package jmespath

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var parsingErrorTests = []struct {
	expression string
	msg        string
}{
	{"foo.", "Incopmlete expression"},
	{"[foo", "Incopmlete expression"},
	{"]", "Invalid"},
	{")", "Invalid"},
	{"}", "Invalid"},
	{"foo..bar", "Invalid"},
	{`foo."bar`, "Forwards lexer errors"},
	{`{foo: bar`, "Incomplete expression"},
	{`{foo bar}`, "Invalid"},
	{`[foo bar]`, "Invalid"},
	{`foo@`, "Invalid"},
}

func TestParsingErrors(t *testing.T) {
	assert := assert.New(t)
	parser := NewParser()
	for _, tt := range parsingErrorTests {
		_, err := parser.Parse(tt.expression)
		assert.NotNil(err, fmt.Sprintf("Expected parsing error: %s, for expression: %s", tt.msg, tt.expression))
	}
}
