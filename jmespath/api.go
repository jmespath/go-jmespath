package jmespath

// NewLexer creates a new JMESPath lexer.
func NewLexer() *Lexer {
	lexer := newLexer()
	return lexer
}

// NewParser creates a new JMESPath parser.
func NewParser() *Parser {
	parser := newParser()
	return parser
}

// Search evaluates a JMESPath expression against input data and returns the result.
func Search(expression string, data interface{}) (interface{}, error) {
	intr := NewInterpreter()
	parser := NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	return intr.Execute(ast, data)
}
