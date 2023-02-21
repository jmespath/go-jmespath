package jmespath

import (
	"testing"

	"github.com/jmespath/go-jmespath/internal/testify/assert"
)

func TestScopesMissing(t *testing.T) {
	assert := assert.New(t)
	scopes := newScopes()

	_, found := scopes.getValue("foo")
	assert.False(found)
}

func TestScopesRoot(t *testing.T) {
	assert := assert.New(t)
	scopes := newScopes()
	scopes.pushScope(map[string]interface{}{"foo": "bar"})

	value, found := scopes.getValue("foo")
	assert.True(found)
	assert.Equal("bar", value.(string))
}

func TestScopesNested(t *testing.T) {
	assert := assert.New(t)
	scopes := newScopes()
	scopes.pushScope(map[string]interface{}{"foo": "bar", "qux": "quux"})
	scopes.pushScope(map[string]interface{}{"foo": "baz"})

	value, found := scopes.getValue("foo")
	assert.True(found)
	assert.Equal("baz", value.(string))

	value, found = scopes.getValue("qux")
	assert.True(found)
	assert.Equal("quux", value.(string))
}

//
//     def test_Scope_nested(self):
//         self._scopes.pushScope({'foo': 'bar'})
//         self._scopes.pushScope({'foo': 'baz'})
//         value = self._scopes.getValue('foo')
//         self.assertEqual('baz', value)
