package jmespath

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type astNodeType int

//go:generate stringer -type astNodeType
const (
	ASTEmpty astNodeType = iota
	ASTComparator
	ASTCurrentNode
	ASTExpRef
	ASTFunctionExpression
	ASTField
	ASTFilterProjection
	ASTFlatten
	ASTIdentity
	ASTIndex
	ASTIndexExpression
	ASTKeyValPair
	ASTLiteral
	ASTMultiSelectHash
	ASTMultiSelectList
	ASTOrExpression
	ASTPipe
	ASTProjection
	ASTSubexpression
	ASTSlice
	ASTValueProjection
)

type ASTNode struct {
	nodeType astNodeType
	value    interface{}
	children []ASTNode
}

func (node ASTNode) String() string {
	var value string
	if node.value == nil {
		value = "<nil>"
	} else {
		value = fmt.Sprintf("%s", node.value)
	}
	return fmt.Sprintf("ASTNode{type: %s val:%s children:%s}", node.nodeType, value, node.children)
}

var bindingPowers = map[tokType]int{
	tEOF:                0,
	tUnquotedIdentifier: 0,
	tQuotedIdentifier:   0,
	tRbracket:           0,
	tRparen:             0,
	tComma:              0,
	tRbrace:             0,
	tNumber:             0,
	tCurrent:            0,
	tExpref:             0,
	tColon:              0,
	tPipe:               1,
	tEQ:                 2,
	tLT:                 2,
	tLTE:                2,
	tGT:                 2,
	tGTE:                2,
	tNE:                 2,
	tOr:                 5,
	tFlatten:            6,
	tStar:               20,
	tFilter:             21,
	tDot:                40,
	tLbrace:             50,
	tLbracket:           55,
	tLparen:             60,
}

type Parser struct {
	expression string
	tokens     []token
	index      int
}

func newParser() *Parser {
	parser := Parser{}
	return &parser
}

func (parser *Parser) Parse(expression string) (ASTNode, error) {
	lexer := NewLexer()
	parser.expression = expression
	parser.index = 0
	tokens, err := lexer.Tokenize(expression)
	if err != nil {
		return ASTNode{}, err
	}
	parser.tokens = tokens
	parsed, err := parser.parseExpression(0)
	if err != nil {
		return ASTNode{}, err
	}
	if parser.current() != tEOF {
		return ASTNode{}, parser.syntaxError(fmt.Sprintf("Unexpected remaining token: %s", parser.current()))
	}
	return parsed, nil
}

func (parser *Parser) parseExpression(bindingPower int) (ASTNode, error) {
	var err error
	leftToken := parser.lookaheadToken(0)
	parser.advance()
	leftNode, err := parser.nud(leftToken)
	if err != nil {
		return ASTNode{}, err
	}
	currentToken := parser.current()
	for bindingPower < bindingPowers[currentToken] {
		parser.advance()
		leftNode, err = parser.led(currentToken, leftNode)
		if err != nil {
			return ASTNode{}, err
		}
		currentToken = parser.current()
	}
	return leftNode, nil
}

func (parser *Parser) parseIndexExpression() (ASTNode, error) {
	if parser.lookahead(0) == tColon || parser.lookahead(1) == tColon {
		return parser.parseSliceExpression()
	}
	indexStr := parser.lookaheadToken(0).value
	parsedInt, err := strconv.Atoi(indexStr)
	if err != nil {
		return ASTNode{}, err
	}
	indexNode := ASTNode{nodeType: ASTIndex, value: parsedInt}
	parser.advance()
	if err := parser.match(tRbracket); err != nil {
		return ASTNode{}, err
	}
	return indexNode, nil
}

func (parser *Parser) parseSliceExpression() (ASTNode, error) {
	// TODO: This isn't quite correct.  We need to differentiate
	// between "not set" and "user specified" 0, as that affects
	// how the slice is interpreted.
	parts := []*int{nil, nil, nil}
	index := 0
	current := parser.current()
	for current != tRbracket && index < 3 {
		if current == tColon {
			index++
			parser.advance()
		} else if current == tNumber {
			parsedInt, err := strconv.Atoi(parser.lookaheadToken(0).value)
			if err != nil {
				return ASTNode{}, err
			}
			parts[index] = &parsedInt
			parser.advance()
		} else {
			return ASTNode{}, parser.syntaxError("Syntax error")
		}
		current = parser.current()
	}
	if err := parser.match(tRbracket); err != nil {
		return ASTNode{}, err
	}
	return ASTNode{
		nodeType: ASTSlice,
		value:    parts,
	}, nil
}

func (parser *Parser) match(tokenType tokType) error {
	if parser.current() == tokenType {
		parser.advance()
		return nil
	}
	return parser.syntaxError("Expected " + tokenType.String() + ", received: " + parser.current().String())
}

func (parser *Parser) led(tokenType tokType, node ASTNode) (ASTNode, error) {
	switch tokenType {
	case tDot:
		if parser.current() != tStar {
			right, err := parser.parseDotRHS(bindingPowers[tDot])
			return ASTNode{
				nodeType: ASTSubexpression,
				children: []ASTNode{node, right},
			}, err
		}
		parser.advance()
		right, err := parser.parseProjectionRHS(bindingPowers[tDot])
		return ASTNode{
			nodeType: ASTValueProjection,
			children: []ASTNode{node, right},
		}, err
	case tPipe:
		right, err := parser.parseExpression(bindingPowers[tPipe])
		return ASTNode{nodeType: ASTPipe, children: []ASTNode{node, right}}, err
	case tOr:
		right, err := parser.parseExpression(bindingPowers[tOr])
		return ASTNode{nodeType: ASTOrExpression, children: []ASTNode{node, right}}, err
	case tLparen:
		name := node.value
		var args []ASTNode
		for parser.current() != tRparen {
			expression, _ := parser.parseExpression(0)
			if parser.current() == tComma {
				if err := parser.match(tComma); err != nil {
					return ASTNode{}, err
				}
			}
			args = append(args, expression)
		}
		if err := parser.match(tRparen); err != nil {
			return ASTNode{}, err
		}
		return ASTNode{
			nodeType: ASTFunctionExpression,
			value:    name,
			children: args,
		}, nil
	case tFilter:
		return parser.parseFilter(node)
	case tFlatten:
		left := ASTNode{nodeType: ASTFlatten, children: []ASTNode{node}}
		right, err := parser.parseProjectionRHS(bindingPowers[tFlatten])
		return ASTNode{
			nodeType: ASTProjection,
			children: []ASTNode{left, right},
		}, err
	case tEQ, tNE, tGT, tGTE, tLT, tLTE:
		right, err := parser.parseExpression(bindingPowers[tokenType])
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{
			nodeType: ASTComparator,
			value:    tokenType,
			children: []ASTNode{node, right},
		}, nil
	case tLbracket:
		tokenType := parser.current()
		var right ASTNode
		var err error
		if tokenType == tNumber || tokenType == tColon {
			right, err = parser.parseIndexExpression()
			return parser.projectIfSlice(node, right)
		}
		// Otherwise this is a projection.
		if err := parser.match(tStar); err != nil {
			return ASTNode{}, err
		}
		if err := parser.match(tRbracket); err != nil {
			return ASTNode{}, err
		}
		right, err = parser.parseProjectionRHS(bindingPowers[tStar])
		return ASTNode{
			nodeType: ASTProjection,
			children: []ASTNode{node, right},
		}, err
	}
	return ASTNode{}, parser.syntaxError("Unexpected token: " + tokenType.String())
}

func (parser *Parser) nud(token token) (ASTNode, error) {
	switch token.tokenType {
	case tJSONLiteral:
		var parsed interface{}
		err := json.Unmarshal([]byte(token.value), &parsed)
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{nodeType: ASTLiteral, value: parsed}, nil
	case tStringLiteral:
		return ASTNode{nodeType: ASTLiteral, value: token.value}, nil
	case tUnquotedIdentifier:
		return ASTNode{
			nodeType: ASTField,
			value:    token.value,
		}, nil
	case tQuotedIdentifier:
		node := ASTNode{nodeType: ASTField, value: token.value}
		if parser.current() == tLparen {
			return ASTNode{}, parser.syntaxError("Can't have quoted identifier as function name.")
		}
		return node, nil
	case tStar:
		left := ASTNode{nodeType: ASTIdentity}
		var right ASTNode
		var err error
		if parser.current() == tRbracket {
			right = ASTNode{nodeType: ASTIdentity}
		} else {
			right, err = parser.parseProjectionRHS(bindingPowers[tStar])
		}
		return ASTNode{nodeType: ASTValueProjection, children: []ASTNode{left, right}}, err
	case tFilter:
		return parser.parseFilter(ASTNode{nodeType: ASTIdentity})
	case tLbrace:
		return parser.parseMultiSelectHash()
	case tFlatten:
		left := ASTNode{
			nodeType: ASTFlatten,
			children: []ASTNode{ASTNode{nodeType: ASTIdentity}},
		}
		right, err := parser.parseProjectionRHS(bindingPowers[tFlatten])
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{nodeType: ASTProjection, children: []ASTNode{left, right}}, nil
	case tLbracket:
		tokenType := parser.current()
		//var right ASTNode
		if tokenType == tNumber || tokenType == tColon {
			right, err := parser.parseIndexExpression()
			if err != nil {
				return ASTNode{}, nil
			}
			return parser.projectIfSlice(ASTNode{nodeType: ASTIdentity}, right)
		} else if tokenType == tStar && parser.lookahead(1) == tRbracket {
			parser.advance()
			parser.advance()
			right, err := parser.parseProjectionRHS(bindingPowers[tStar])
			if err != nil {
				return ASTNode{}, nil
			}
			return ASTNode{
				nodeType: ASTProjection,
				children: []ASTNode{ASTNode{nodeType: ASTIdentity}, right},
			}, nil
		} else {
			return parser.parseMultiSelectList()
		}
	case tCurrent:
		return ASTNode{nodeType: ASTCurrentNode}, nil
	case tExpref:
		expression, err := parser.parseExpression(bindingPowers[tExpref])
		return ASTNode{nodeType: ASTExpRef, children: []ASTNode{expression}}, err
	case tEOF:
		return ASTNode{}, SyntaxError{msg: "Incomplete expression", Expression: parser.expression, Offset: token.position}
	}

	return ASTNode{}, parser.syntaxError("Invalid token")
}

func (parser *Parser) parseMultiSelectList() (ASTNode, error) {
	var expressions []ASTNode
	for {
		expression, err := parser.parseExpression(0)
		if err != nil {
			return ASTNode{}, err
		}
		expressions = append(expressions, expression)
		if parser.current() == tRbracket {
			break
		}
		err = parser.match(tComma)
		if err != nil {
			return ASTNode{}, err
		}
	}
	err := parser.match(tRbracket)
	return ASTNode{
		nodeType: ASTMultiSelectList,
		children: expressions,
	}, err
}

func (parser *Parser) parseMultiSelectHash() (ASTNode, error) {
	var children []ASTNode
	for {
		keyToken := parser.lookaheadToken(0)
		if err := parser.match(tUnquotedIdentifier); err != nil {
			if err := parser.match(tQuotedIdentifier); err != nil {
				return ASTNode{}, parser.syntaxError("Expected tQuotedIdentifier or tUnquotedIdentifier")
			}
		}
		keyName := keyToken.value
		err := parser.match(tColon)
		if err != nil {
			return ASTNode{}, err
		}
		value, err := parser.parseExpression(0)
		if err != nil {
			return ASTNode{}, err
		}
		node := ASTNode{
			nodeType: ASTKeyValPair,
			value:    keyName,
			children: []ASTNode{value},
		}
		children = append(children, node)
		if parser.current() == tComma {
			err := parser.match(tComma)
			if err != nil {
				return ASTNode{}, nil
			}
		} else if parser.current() == tRbrace {
			err := parser.match(tRbrace)
			if err != nil {
				return ASTNode{}, nil
			}
			break
		}
	}
	return ASTNode{
		nodeType: ASTMultiSelectHash,
		children: children,
	}, nil
}

func (parser *Parser) projectIfSlice(left ASTNode, right ASTNode) (ASTNode, error) {
	indexExpr := ASTNode{
		nodeType: ASTIndexExpression,
		children: []ASTNode{left, right},
	}
	if right.nodeType == ASTSlice {
		right, err := parser.parseProjectionRHS(bindingPowers[tStar])
		return ASTNode{
			nodeType: ASTProjection,
			children: []ASTNode{indexExpr, right},
		}, err
	}
	return indexExpr, nil
}
func (parser *Parser) parseFilter(node ASTNode) (ASTNode, error) {
	var right, condition ASTNode
	var err error
	condition, err = parser.parseExpression(0)
	if err != nil {
		return ASTNode{}, err
	}
	parser.match(tRbracket)
	if parser.current() == tFlatten {
		right = ASTNode{nodeType: ASTIdentity}
	} else {
		right, err = parser.parseProjectionRHS(bindingPowers[tFilter])
		if err != nil {
			return ASTNode{}, err
		}
	}

	return ASTNode{
		nodeType: ASTFilterProjection,
		children: []ASTNode{node, right, condition},
	}, nil
}

func (parser *Parser) parseDotRHS(bindingPower int) (ASTNode, error) {
	lookahead := parser.current()
	if tokensOneOf([]tokType{tQuotedIdentifier, tUnquotedIdentifier, tStar}, lookahead) {
		return parser.parseExpression(bindingPower)
	} else if lookahead == tLbracket {
		parser.match(tLbracket)
		return parser.parseMultiSelectList()
	} else if lookahead == tLbrace {
		parser.match(tLbrace)
		return parser.parseMultiSelectHash()
	}
	return ASTNode{}, parser.syntaxError("Expected identifier, lbracket, or lbrace")
}

func (parser *Parser) parseProjectionRHS(bindingPower int) (ASTNode, error) {
	current := parser.current()
	if bindingPowers[current] < 10 {
		return ASTNode{nodeType: ASTIdentity}, nil
	} else if current == tLbracket {
		return parser.parseExpression(bindingPower)
	} else if current == tFilter {
		return parser.parseExpression(bindingPower)
	} else if current == tDot {
		err := parser.match(tDot)
		if err != nil {
			return ASTNode{}, err
		}
		return parser.parseDotRHS(bindingPower)
	} else {
		return ASTNode{}, parser.syntaxError("Error")
	}
}

func (parser *Parser) lookahead(number int) tokType {
	return parser.lookaheadToken(number).tokenType
}

func (parser *Parser) current() tokType {
	return parser.lookahead(0)
}

func (parser *Parser) lookaheadToken(number int) token {
	return parser.tokens[parser.index+number]
}

func (parser *Parser) advance() {
	parser.index++
}

func tokensOneOf(elements []tokType, token tokType) bool {
	for _, elem := range elements {
		if elem == token {
			return true
		}
	}
	return false
}

func (parser *Parser) syntaxError(msg string) SyntaxError {
	return SyntaxError{
		msg:        msg,
		Expression: parser.expression,
		Offset:     parser.lookaheadToken(0).position,
	}
}
