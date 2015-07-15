package jmespath

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var lexingTests = []struct {
	expression string
	expected   []token
}{
	{"*", []token{token{tStar, "*", 0, 1}}},
	{".", []token{token{tDot, ".", 0, 1}}},
	{"[?", []token{token{tFilter, "[?", 0, 2}}},
	{"[]", []token{token{tFlatten, "[]", 0, 2}}},
	{"(", []token{token{tLparen, "(", 0, 1}}},
	{")", []token{token{tRparen, ")", 0, 1}}},
	{"[", []token{token{tLbracket, "[", 0, 1}}},
	{"]", []token{token{tRbracket, "]", 0, 1}}},
	{"{", []token{token{tLbrace, "{", 0, 1}}},
	{"}", []token{token{tRbrace, "}", 0, 1}}},
	{"||", []token{token{tOr, "||", 0, 2}}},
	{"|", []token{token{tPipe, "|", 0, 1}}},
	{"29", []token{token{tNumber, "29", 0, 2}}},
	{"2", []token{token{tNumber, "2", 0, 1}}},
	{"0", []token{token{tNumber, "0", 0, 1}}},
	{"-20", []token{token{tNumber, "-20", 0, 3}}},
	{"foo", []token{token{tUnquotedIdentifier, "foo", 0, 3}}},
	{`"bar"`, []token{token{tQuotedIdentifier, "bar", 0, 3}}},
	// Escaping the delimiter
	{`"bar\"baz"`, []token{token{tQuotedIdentifier, `bar"baz`, 0, 7}}},
	{",", []token{token{tComma, ",", 0, 1}}},
	{":", []token{token{tColon, ":", 0, 1}}},
	{"<", []token{token{tLT, "<", 0, 1}}},
	{"<=", []token{token{tLTE, "<=", 0, 2}}},
	{">", []token{token{tGT, ">", 0, 1}}},
	{">=", []token{token{tGTE, ">=", 0, 2}}},
	{"==", []token{token{tEQ, "==", 0, 2}}},
	{"!=", []token{token{tNE, "!=", 0, 2}}},
	{"`[0, 1, 2]`", []token{token{tJSONLiteral, "[0, 1, 2]", 1, 9}}},
	{"'foo'", []token{token{tStringLiteral, "foo", 1, 3}}},
	{"'a'", []token{token{tStringLiteral, "a", 1, 1}}},
	{`'foo\'bar'`, []token{token{tStringLiteral, "foo'bar", 1, 7}}},
	{"@", []token{token{tCurrent, "@", 0, 1}}},
	{"&", []token{token{tExpref, "&", 0, 1}}},
	// Quoted identifier unicode escape sequences
	{`"\u2713"`, []token{token{tQuotedIdentifier, "✓", 0, 3}}},
	{`"\\"`, []token{token{tQuotedIdentifier, `\`, 0, 1}}},
	{"`\"foo\"`", []token{token{tJSONLiteral, "\"foo\"", 1, 5}}},
	// Combinations of tokens.
	{"foo.bar", []token{
		token{tUnquotedIdentifier, "foo", 0, 3},
		token{tDot, ".", 3, 1},
		token{tUnquotedIdentifier, "bar", 4, 3},
	}},
	{"foo[0]", []token{
		token{tUnquotedIdentifier, "foo", 0, 3},
		token{tLbracket, "[", 3, 1},
		token{tNumber, "0", 4, 1},
		token{tRbracket, "]", 5, 1},
	}},
	{"foo[?a<b]", []token{
		token{tUnquotedIdentifier, "foo", 0, 3},
		token{tFilter, "[?", 3, 2},
		token{tUnquotedIdentifier, "a", 5, 1},
		token{tLT, "<", 6, 1},
		token{tUnquotedIdentifier, "b", 7, 1},
		token{tRbracket, "]", 8, 1},
	}},
}

func TestCanLexTokens(t *testing.T) {
	assert := assert.New(t)
	lexer := NewLexer()
	for _, tt := range lexingTests {
		tokens, err := lexer.Tokenize(tt.expression)
		if assert.Nil(err) {
			errMsg := fmt.Sprintf("Mismatch expected number of tokens: (expected: %s, actual: %s)",
				tt.expected, tokens)
			tt.expected = append(tt.expected, token{tEOF, "", len(tt.expression), 0})
			if assert.Equal(len(tt.expected), len(tokens), errMsg) {
				for i, token := range tokens {
					expected := tt.expected[i]
					assert.Equal(expected, token, "Token not equal")
				}
			}
		}
	}
}

var lexingErrorTests = []struct {
	expression string
	msg        string
}{
	{"'foo", "Missing closing single quote"},
	{"[?foo==bar?]", "Unknown char '?'"},
}

func TestLexingErrors(t *testing.T) {
	assert := assert.New(t)
	lexer := NewLexer()
	for _, tt := range lexingErrorTests {
		_, err := lexer.Tokenize(tt.expression)
		assert.NotNil(err, fmt.Sprintf("Expected lexing error: %s", tt.msg))
	}
}
