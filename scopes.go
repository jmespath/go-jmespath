package jmespath

type scopes struct {
	stack []map[string]interface{} // The stack of scope JSON objects.
}

// newScopes creates a new instance of JMESPath scopes.
func newScopes() *scopes {
	stack := []map[string]interface{}{}
	scopes := scopes{stack: stack}
	return &scopes
}

func (scopes *scopes) pushScope(item map[string]interface{}) {
	scopes.stack = append(scopes.stack, item)
}

func (scopes *scopes) popScope() map[string]interface{} {
	if len(scopes.stack) == 0 {
		panic("unable to pop empty scopes stack")
	}
	result := scopes.stack[len(scopes.stack)-1]
	scopes.stack = scopes.stack[:len(scopes.stack)-1]
	return result
}

func (scopes *scopes) getValue(identifier string) (interface{}, bool) {
	stack := scopes.stack
	for i := len(stack) - 1; i >= 0; i-- {
		scope := stack[i]
		if item, ok := scope[identifier]; ok {
			return item, true
		}
	}
	return nil, false
}
