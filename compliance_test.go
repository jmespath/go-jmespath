package jmespath

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/go-jmespath/internal/testify/assert"
)

type TestSuite struct {
	Given     interface{}
	TestCases []TestCase `json:"cases"`
	Comment   string
}
type TestCase struct {
	Comment    string
	Expression string
	Result     interface{}
	Error      string
}

var whiteListed = []string{
	"compliance/basic.json",
	"compliance/current.json",
	"compliance/escape.json",
	"compliance/filters.json",
	"compliance/functions.json",
	"compliance/identifiers.json",
	"compliance/indices.json",
	"compliance/literal.json",
	"compliance/multiselect.json",
	"compliance/ormatch.json",
	"compliance/pipe.json",
	"compliance/slice.json",
	"compliance/syntax.json",
	"compliance/unicode.json",
	"compliance/wildcard.json",
	"compliance/boolean.json",
}

func allowed(path string) bool {
	for _, el := range whiteListed {
		if el == path {
			return true
		}
	}
	return false
}

func TestCompliance(t *testing.T) {
	assert := assert.New(t)

	var complianceFiles []string
	err := filepath.Walk("compliance", func(path string, _ os.FileInfo, _ error) error {
		//if strings.HasSuffix(path, ".json") {
		if allowed(path) {
			complianceFiles = append(complianceFiles, path)
		}
		return nil
	})
	if assert.Nil(err) {
		for _, filename := range complianceFiles {
			runComplianceTest(assert, filename)
		}
	}
}

func TestOrOperatorHandlesErrorPath(t *testing.T) {
	expression := "outer.bad || outer.foo"
	givenRaw := []byte(`{
		"outer": {
		  "foo": "foo",
		  "bar": "bar",
		  "baz": "baz"
		}
	}`)

	var given interface{}
	err := json.Unmarshal(givenRaw, &given)
	assert.Nil(t, err)

	actual, err := Search(expression, given)
	if _, ok := err.(NotFoundError); ok {
		err = nil
		actual = nil
	}

	assert.Nil(t, err)
	assert.Equal(t, actual.(string), "foo")
}

func TestInvalidPathMustReturnError(t *testing.T) {
	expression := "outer.bad"
	givenRaw := []byte(`{
		"outer": {
		  "foo": "foo",
		  "bar": "bar",
		  "baz": "baz"
		}
	}`)

	var given interface{}
	var err error

	err = json.Unmarshal(givenRaw, &given)
	assert.Nil(t, err)

	_, err = Search(expression, given)
	assert.Error(t, err)

	_, ok := err.(NotFoundError)
	assert.True(t, ok)
}

func TestNullValueMustBeReturned(t *testing.T) {
	expression := "outer.foo"
	givenRaw := []byte(`{
		"outer": {
		  "foo": null
		}
	}`)

	var given interface{}

	err := json.Unmarshal(givenRaw, &given)
	assert.Nil(t, err)

	result, err := Search(expression, given)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestFilterInsideThePathHandlesErrorInPath(t *testing.T) {
	expression := "foo[?a==`1`].b.c"
	givenRaw := []byte(`{
		"foo": [
			{"a": 1, "b": {"c": "x"}},
			{"a": 1, "b": {"c": "y"}},
			{"a": 1, "b": {"c": "z"}},
			{"a": 2, "b": {"c": "z"}},
			{"a": 1, "baz": 2}
		]
	}`)

	expectedRaw := []byte(`["x", "y", "z"]`)

	var given, expected interface{}
	var err error

	err = json.Unmarshal(givenRaw, &given)
	assert.Nil(t, err)

	err = json.Unmarshal(expectedRaw, &expected)
	assert.Nil(t, err)

	actual, err := Search(expression, given)
	if _, ok := err.(NotFoundError); ok {
		err = nil
		actual = nil
	}

	assert.Nil(t, err)
	assert.Equal(t, actual, expected)
}

func TestUnaryNotFilterInsideThePathHandlesErrorInPath(t *testing.T) {
	expression := "foo[?!key]"
	givenRaw := []byte(`{
		"foo": [
		  {"key": true},
		  {"key": false},
		  {"key": []},
		  {"key": {}},
		  {"key": [0]},
		  {"key": {"a": "b"}},
		  {"key": 0},
		  {"key": 1},
		  {"key": null},
		  {"notkey": true}
		]
	  }`)

	expectedRaw := []byte(`[{"key": false}, {"key": []}, {"key": {}}, {"key": null}, {"notkey": true} ]`)

	var given, expected interface{}
	var err error

	err = json.Unmarshal(givenRaw, &given)
	assert.Nil(t, err)

	err = json.Unmarshal(expectedRaw, &expected)
	assert.Nil(t, err)

	actual, err := Search(expression, given)
	if _, ok := err.(NotFoundError); ok {
		err = nil
		actual = nil
	}

	assert.Nil(t, err)
	assert.Equal(t, actual, expected)
}

func TestEqualityWithNullRHS_MustHandleError(t *testing.T) {
	expression := "foo[?key == `null`]"
	givenRaw := []byte(`{
		"foo": [
		  {"key": true},
		  {"key": false},
		  {"key": []},
		  {"key": {}},
		  {"key": [0]},
		  {"key": {"a": "b"}},
		  {"key": 0},
		  {"key": 1},
		  {"key": null},
		  {"notkey": true}
		]
	  }`)

	expectedRaw := []byte(`[ {"key": null}, {"notkey": true} ]`)

	var given, expected interface{}
	var err error

	err = json.Unmarshal(givenRaw, &given)
	assert.Nil(t, err)

	err = json.Unmarshal(expectedRaw, &expected)
	assert.Nil(t, err)

	actual, err := Search(expression, given)
	if _, ok := err.(NotFoundError); ok {
		err = nil
		actual = nil
	}

	assert.Nil(t, err)
	assert.Equal(t, actual, expected)
}

func TestMapFunction_MustHandleError(t *testing.T) {
	expression := "map(&c, people)"
	givenRaw := []byte(`{
		"people": [
			 {"a": 10, "b": 1, "c": "z"},
			 {"a": 10, "b": 2, "c": null},
			 {"a": 10, "b": 3},
			 {"a": 10, "b": 4, "c": "z"},
			 {"a": 10, "b": 5, "c": null},
			 {"a": 10, "b": 6},
			 {"a": 10, "b": 7, "c": "z"},
			 {"a": 10, "b": 8, "c": null},
			 {"a": 10, "b": 9}
		],
		"empty": []
	  }`)

	expectedRaw := []byte(`["z", null, null, "z", null, null, "z", null, null]`)

	var given, expected interface{}
	var err error

	err = json.Unmarshal(givenRaw, &given)
	assert.Nil(t, err)

	err = json.Unmarshal(expectedRaw, &expected)
	assert.Nil(t, err)

	actual, err := Search(expression, given)
	if _, ok := err.(NotFoundError); ok {
		err = nil
		actual = nil
	}

	assert.Nil(t, err)
	assert.Equal(t, actual, expected)
}

func runComplianceTest(assert *assert.Assertions, filename string) {
	var testSuites []TestSuite
	data, err := ioutil.ReadFile(filename)
	if assert.Nil(err) {
		err := json.Unmarshal(data, &testSuites)
		if assert.Nil(err) {
			for _, testsuite := range testSuites {
				runTestSuite(assert, testsuite, filename)
			}
		}
	}
}

func runTestSuite(assert *assert.Assertions, testsuite TestSuite, filename string) {
	for _, testcase := range testsuite.TestCases {
		if testcase.Error != "" {
			// This is a test case that verifies we error out properly.
			runSyntaxTestCase(assert, testsuite.Given, testcase, filename)
		} else {
			runTestCase(assert, testsuite.Given, testcase, filename)
		}
	}
}

func runSyntaxTestCase(assert *assert.Assertions, given interface{}, testcase TestCase, filename string) {
	// Anything with an .Error means that we expect that JMESPath should return
	// an error when we try to evaluate the expression.
	_, err := Search(testcase.Expression, given)
	assert.NotNil(err, fmt.Sprintf("Expression: %s", testcase.Expression))
}

func runTestCase(assert *assert.Assertions, given interface{}, testcase TestCase, filename string) {
	lexer := NewLexer()
	var err error
	_, err = lexer.tokenize(testcase.Expression)
	if err != nil {
		errMsg := fmt.Sprintf("(%s) Could not lex expression: %s -- %s", filename, testcase.Expression, err.Error())
		assert.Fail(errMsg)
		return
	}
	parser := NewParser()
	_, err = parser.Parse(testcase.Expression)
	if err != nil {
		errMsg := fmt.Sprintf("(%s) Could not parse expression: %s -- %s", filename, testcase.Expression, err.Error())
		assert.Fail(errMsg)
		return
	}
	actual, err := Search(testcase.Expression, given)
	if _, ok := err.(NotFoundError); ok {
		err = nil
		actual = nil
	}

	if assert.Nil(err, fmt.Sprintf("Expression: %s", testcase.Expression)) {
		assert.Equal(testcase.Result, actual, fmt.Sprintf("Expression: %s", testcase.Expression))
	}
}
