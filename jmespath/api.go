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
