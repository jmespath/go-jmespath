package jmespath

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ASTNodeType int

//go:generate stringer -type ASTNodeType
const (
	ASTEmpty ASTNodeType = iota
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
	ASTAndExpression
	ASTNotExpression
	ASTPipe
	ASTProjection
	ASTSubexpression
	ASTSlice
	ASTValueProjection
)

// ASTNode represents the abstract syntax tree of a JMESPath expression.
type ASTNode struct {
	NodeType ASTNodeType
	Value    interface{}
	Children []ASTNode
}

func (node ASTNode) String() string {
	return node.PrettyPrint(0)
}

// PrettyPrint will pretty print the parsed AST.
// The AST is an implementation detail and this pretty print
// function is provided as a convenience method to help with
// debugging.  You should not rely on its output as the internal
// structure of the AST may change at any time.
func (node ASTNode) PrettyPrint(indent int) string {
	spaces := strings.Repeat(" ", indent)
	output := fmt.Sprintf("%s%s {\n", spaces, node.NodeType)
	nextIndent := indent + 2
	if node.Value != nil {
		if converted, ok := node.Value.(fmt.Stringer); ok {
			// Account for things like comparator nodes
			// that are enums with a String() method.
			output += fmt.Sprintf("%svalue: %s\n", strings.Repeat(" ", nextIndent), converted.String())
		} else {
			output += fmt.Sprintf("%svalue: %#v\n", strings.Repeat(" ", nextIndent), node.Value)
		}
	}
	lastIndex := len(node.Children)
	if lastIndex > 0 {
		output += fmt.Sprintf("%schildren: {\n", strings.Repeat(" ", nextIndent))
		childIndent := nextIndent + 2
		for _, elem := range node.Children {
			output += elem.PrettyPrint(childIndent)
		}
	}
	output += fmt.Sprintf("%s}\n", spaces)
	return output
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
	tOr:                 2,
	tAnd:                3,
	tEQ:                 5,
	tLT:                 5,
	tLTE:                5,
	tGT:                 5,
	tGTE:                5,
	tNE:                 5,
	tFlatten:            9,
	tStar:               20,
	tFilter:             21,
	tDot:                40,
	tNot:                45,
	tLbrace:             50,
	tLbracket:           55,
	tLparen:             60,
}

// Parser holds state about the current expression being parsed.
type Parser struct {
	expression string
	tokens     []token
	index      int
}

// NewParser creates a new JMESPath parser.
func NewParser() *Parser {
	p := Parser{}
	return &p
}

// Parse will compile a JMESPath expression.
func (p *Parser) Parse(expression string) (ASTNode, error) {
	lexer := NewLexer()
	p.expression = expression
	p.index = 0
	tokens, err := lexer.tokenize(expression)
	if err != nil {
		return ASTNode{}, err
	}
	p.tokens = tokens
	parsed, err := p.parseExpression(0)
	if err != nil {
		return ASTNode{}, err
	}
	if p.current() != tEOF {
		return ASTNode{}, p.syntaxError(fmt.Sprintf(
			"Unexpected token at the end of the expression: %s", p.current()))
	}
	return parsed, nil
}

func (p *Parser) parseExpression(bindingPower int) (ASTNode, error) {
	var err error
	leftToken := p.lookaheadToken(0)
	p.advance()
	leftNode, err := p.nud(leftToken)
	if err != nil {
		return ASTNode{}, err
	}
	currentToken := p.current()
	for bindingPower < bindingPowers[currentToken] {
		p.advance()
		leftNode, err = p.led(currentToken, leftNode)
		if err != nil {
			return ASTNode{}, err
		}
		currentToken = p.current()
	}
	return leftNode, nil
}

func (p *Parser) parseIndexExpression() (ASTNode, error) {
	if p.lookahead(0) == tColon || p.lookahead(1) == tColon {
		return p.parseSliceExpression()
	}
	indexStr := p.lookaheadToken(0).value
	parsedInt, err := strconv.Atoi(indexStr)
	if err != nil {
		return ASTNode{}, err
	}
	indexNode := ASTNode{NodeType: ASTIndex, Value: parsedInt}
	p.advance()
	if err := p.match(tRbracket); err != nil {
		return ASTNode{}, err
	}
	return indexNode, nil
}

func (p *Parser) parseSliceExpression() (ASTNode, error) {
	parts := []*int{nil, nil, nil}
	index := 0
	current := p.current()
	for current != tRbracket && index < 3 {
		if current == tColon {
			index++
			p.advance()
		} else if current == tNumber {
			parsedInt, err := strconv.Atoi(p.lookaheadToken(0).value)
			if err != nil {
				return ASTNode{}, err
			}
			parts[index] = &parsedInt
			p.advance()
		} else {
			return ASTNode{}, p.syntaxError(
				"Expected tColon or tNumber" + ", received: " + p.current().String())
		}
		current = p.current()
	}
	if err := p.match(tRbracket); err != nil {
		return ASTNode{}, err
	}
	return ASTNode{
		NodeType: ASTSlice,
		Value:    parts,
	}, nil
}

func (p *Parser) match(tokenType tokType) error {
	if p.current() == tokenType {
		p.advance()
		return nil
	}
	return p.syntaxError("Expected " + tokenType.String() + ", received: " + p.current().String())
}

func (p *Parser) led(tokenType tokType, node ASTNode) (ASTNode, error) {
	switch tokenType {
	case tDot:
		if p.current() != tStar {
			right, err := p.parseDotRHS(bindingPowers[tDot])
			return ASTNode{
				NodeType: ASTSubexpression,
				Children: []ASTNode{node, right},
			}, err
		}
		p.advance()
		right, err := p.parseProjectionRHS(bindingPowers[tDot])
		return ASTNode{
			NodeType: ASTValueProjection,
			Children: []ASTNode{node, right},
		}, err
	case tPipe:
		right, err := p.parseExpression(bindingPowers[tPipe])
		return ASTNode{NodeType: ASTPipe, Children: []ASTNode{node, right}}, err
	case tOr:
		right, err := p.parseExpression(bindingPowers[tOr])
		return ASTNode{NodeType: ASTOrExpression, Children: []ASTNode{node, right}}, err
	case tAnd:
		right, err := p.parseExpression(bindingPowers[tAnd])
		return ASTNode{NodeType: ASTAndExpression, Children: []ASTNode{node, right}}, err
	case tLparen:
		name := node.Value
		var args []ASTNode
		for p.current() != tRparen {
			expression, err := p.parseExpression(0)
			if err != nil {
				return ASTNode{}, err
			}
			if p.current() == tComma {
				if err := p.match(tComma); err != nil {
					return ASTNode{}, err
				}
			}
			args = append(args, expression)
		}
		if err := p.match(tRparen); err != nil {
			return ASTNode{}, err
		}
		return ASTNode{
			NodeType: ASTFunctionExpression,
			Value:    name,
			Children: args,
		}, nil
	case tFilter:
		return p.parseFilter(node)
	case tFlatten:
		left := ASTNode{NodeType: ASTFlatten, Children: []ASTNode{node}}
		right, err := p.parseProjectionRHS(bindingPowers[tFlatten])
		return ASTNode{
			NodeType: ASTProjection,
			Children: []ASTNode{left, right},
		}, err
	case tEQ, tNE, tGT, tGTE, tLT, tLTE:
		right, err := p.parseExpression(bindingPowers[tokenType])
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{
			NodeType: ASTComparator,
			Value:    tokenType,
			Children: []ASTNode{node, right},
		}, nil
	case tLbracket:
		tokenType := p.current()
		var right ASTNode
		var err error
		if tokenType == tNumber || tokenType == tColon {
			right, err = p.parseIndexExpression()
			if err != nil {
				return ASTNode{}, err
			}
			return p.projectIfSlice(node, right)
		}
		// Otherwise this is a projection.
		if err := p.match(tStar); err != nil {
			return ASTNode{}, err
		}
		if err := p.match(tRbracket); err != nil {
			return ASTNode{}, err
		}
		right, err = p.parseProjectionRHS(bindingPowers[tStar])
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{
			NodeType: ASTProjection,
			Children: []ASTNode{node, right},
		}, nil
	}
	return ASTNode{}, p.syntaxError("Unexpected token: " + tokenType.String())
}

func (p *Parser) nud(token token) (ASTNode, error) {
	switch token.tokenType {
	case tJSONLiteral:
		var parsed interface{}
		err := json.Unmarshal([]byte(token.value), &parsed)
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{NodeType: ASTLiteral, Value: parsed}, nil
	case tStringLiteral:
		return ASTNode{NodeType: ASTLiteral, Value: token.value}, nil
	case tUnquotedIdentifier:
		return ASTNode{
			NodeType: ASTField,
			Value:    token.value,
		}, nil
	case tQuotedIdentifier:
		node := ASTNode{NodeType: ASTField, Value: token.value}
		if p.current() == tLparen {
			return ASTNode{}, p.syntaxErrorToken("Can't have quoted identifier as function Name.", token)
		}
		return node, nil
	case tStar:
		left := ASTNode{NodeType: ASTIdentity}
		var right ASTNode
		var err error
		if p.current() == tRbracket {
			right = ASTNode{NodeType: ASTIdentity}
		} else {
			right, err = p.parseProjectionRHS(bindingPowers[tStar])
		}
		return ASTNode{NodeType: ASTValueProjection, Children: []ASTNode{left, right}}, err
	case tFilter:
		return p.parseFilter(ASTNode{NodeType: ASTIdentity})
	case tLbrace:
		return p.parseMultiSelectHash()
	case tFlatten:
		left := ASTNode{
			NodeType: ASTFlatten,
			Children: []ASTNode{{NodeType: ASTIdentity}},
		}
		right, err := p.parseProjectionRHS(bindingPowers[tFlatten])
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{NodeType: ASTProjection, Children: []ASTNode{left, right}}, nil
	case tLbracket:
		tokenType := p.current()
		//var right ASTNode
		if tokenType == tNumber || tokenType == tColon {
			right, err := p.parseIndexExpression()
			if err != nil {
				return ASTNode{}, nil
			}
			return p.projectIfSlice(ASTNode{NodeType: ASTIdentity}, right)
		} else if tokenType == tStar && p.lookahead(1) == tRbracket {
			p.advance()
			p.advance()
			right, err := p.parseProjectionRHS(bindingPowers[tStar])
			if err != nil {
				return ASTNode{}, err
			}
			return ASTNode{
				NodeType: ASTProjection,
				Children: []ASTNode{{NodeType: ASTIdentity}, right},
			}, nil
		} else {
			return p.parseMultiSelectList()
		}
	case tCurrent:
		return ASTNode{NodeType: ASTCurrentNode}, nil
	case tExpref:
		expression, err := p.parseExpression(bindingPowers[tExpref])
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{NodeType: ASTExpRef, Children: []ASTNode{expression}}, nil
	case tNot:
		expression, err := p.parseExpression(bindingPowers[tNot])
		if err != nil {
			return ASTNode{}, err
		}
		return ASTNode{NodeType: ASTNotExpression, Children: []ASTNode{expression}}, nil
	case tLparen:
		expression, err := p.parseExpression(0)
		if err != nil {
			return ASTNode{}, err
		}
		if err := p.match(tRparen); err != nil {
			return ASTNode{}, err
		}
		return expression, nil
	case tEOF:
		return ASTNode{}, p.syntaxErrorToken("Incomplete expression", token)
	}

	return ASTNode{}, p.syntaxErrorToken("Invalid token: "+token.tokenType.String(), token)
}

func (p *Parser) parseMultiSelectList() (ASTNode, error) {
	var expressions []ASTNode
	for {
		expression, err := p.parseExpression(0)
		if err != nil {
			return ASTNode{}, err
		}
		expressions = append(expressions, expression)
		if p.current() == tRbracket {
			break
		}
		err = p.match(tComma)
		if err != nil {
			return ASTNode{}, err
		}
	}
	err := p.match(tRbracket)
	if err != nil {
		return ASTNode{}, err
	}
	return ASTNode{
		NodeType: ASTMultiSelectList,
		Children: expressions,
	}, nil
}

func (p *Parser) parseMultiSelectHash() (ASTNode, error) {
	var children []ASTNode
	for {
		keyToken := p.lookaheadToken(0)
		if err := p.match(tUnquotedIdentifier); err != nil {
			if err := p.match(tQuotedIdentifier); err != nil {
				return ASTNode{}, p.syntaxError("Expected tQuotedIdentifier or tUnquotedIdentifier")
			}
		}
		keyName := keyToken.value
		err := p.match(tColon)
		if err != nil {
			return ASTNode{}, err
		}
		value, err := p.parseExpression(0)
		if err != nil {
			return ASTNode{}, err
		}
		node := ASTNode{
			NodeType: ASTKeyValPair,
			Value:    keyName,
			Children: []ASTNode{value},
		}
		children = append(children, node)
		if p.current() == tComma {
			err := p.match(tComma)
			if err != nil {
				return ASTNode{}, nil
			}
		} else if p.current() == tRbrace {
			err := p.match(tRbrace)
			if err != nil {
				return ASTNode{}, nil
			}
			break
		}
	}
	return ASTNode{
		NodeType: ASTMultiSelectHash,
		Children: children,
	}, nil
}

func (p *Parser) projectIfSlice(left ASTNode, right ASTNode) (ASTNode, error) {
	indexExpr := ASTNode{
		NodeType: ASTIndexExpression,
		Children: []ASTNode{left, right},
	}
	if right.NodeType == ASTSlice {
		right, err := p.parseProjectionRHS(bindingPowers[tStar])
		return ASTNode{
			NodeType: ASTProjection,
			Children: []ASTNode{indexExpr, right},
		}, err
	}
	return indexExpr, nil
}
func (p *Parser) parseFilter(node ASTNode) (ASTNode, error) {
	var right, condition ASTNode
	var err error
	condition, err = p.parseExpression(0)
	if err != nil {
		return ASTNode{}, err
	}
	if err := p.match(tRbracket); err != nil {
		return ASTNode{}, err
	}
	if p.current() == tFlatten {
		right = ASTNode{NodeType: ASTIdentity}
	} else {
		right, err = p.parseProjectionRHS(bindingPowers[tFilter])
		if err != nil {
			return ASTNode{}, err
		}
	}

	return ASTNode{
		NodeType: ASTFilterProjection,
		Children: []ASTNode{node, right, condition},
	}, nil
}

func (p *Parser) parseDotRHS(bindingPower int) (ASTNode, error) {
	lookahead := p.current()
	if tokensOneOf([]tokType{tQuotedIdentifier, tUnquotedIdentifier, tStar}, lookahead) {
		return p.parseExpression(bindingPower)
	} else if lookahead == tLbracket {
		if err := p.match(tLbracket); err != nil {
			return ASTNode{}, err
		}
		return p.parseMultiSelectList()
	} else if lookahead == tLbrace {
		if err := p.match(tLbrace); err != nil {
			return ASTNode{}, err
		}
		return p.parseMultiSelectHash()
	}
	return ASTNode{}, p.syntaxError("Expected identifier, lbracket, or lbrace")
}

func (p *Parser) parseProjectionRHS(bindingPower int) (ASTNode, error) {
	current := p.current()
	if bindingPowers[current] < 10 {
		return ASTNode{NodeType: ASTIdentity}, nil
	} else if current == tLbracket {
		return p.parseExpression(bindingPower)
	} else if current == tFilter {
		return p.parseExpression(bindingPower)
	} else if current == tDot {
		err := p.match(tDot)
		if err != nil {
			return ASTNode{}, err
		}
		return p.parseDotRHS(bindingPower)
	} else {
		return ASTNode{}, p.syntaxError("Error")
	}
}

func (p *Parser) lookahead(number int) tokType {
	return p.lookaheadToken(number).tokenType
}

func (p *Parser) current() tokType {
	return p.lookahead(0)
}

func (p *Parser) lookaheadToken(number int) token {
	return p.tokens[p.index+number]
}

func (p *Parser) advance() {
	p.index++
}

func tokensOneOf(elements []tokType, token tokType) bool {
	for _, elem := range elements {
		if elem == token {
			return true
		}
	}
	return false
}

func (p *Parser) syntaxError(msg string) SyntaxError {
	return SyntaxError{
		msg:        msg,
		Expression: p.expression,
		Offset:     p.lookaheadToken(0).position,
	}
}

// Create a SyntaxError based on the provided token.
// This differs from syntaxError() which creates a SyntaxError
// based on the current lookahead token.
func (p *Parser) syntaxErrorToken(msg string, t token) SyntaxError {
	return SyntaxError{
		msg:        msg,
		Expression: p.expression,
		Offset:     t.position,
	}
}
