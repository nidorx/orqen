package condition

import (
	"fmt"
	"strconv"
)

type parser struct {
	tokens []Token
	pos    int
}

func newParser(tokens []Token) *parser {
	return &parser{tokens: tokens, pos: 0}
}

func (p *parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{TokenEOF, ""}
	}
	return p.tokens[p.pos]
}

func (p *parser) peek() Token {
	next := p.pos + 1
	if next >= len(p.tokens) {
		return Token{TokenEOF, ""}
	}
	return p.tokens[next]
}

func (p *parser) advance() {
	p.pos++
}

func (p *parser) isEOF() bool {
	return p.pos >= len(p.tokens) || p.current().Type == TokenEOF
}

func (p *parser) expect(tokType TokenType) (Token, error) {
	t := p.current()
	if t.Type != tokType {
		return t, fmt.Errorf("expected %s, got %s (%q)", tokType, t.Type, t.Literal)
	}
	p.advance()
	return t, nil
}

// parse is the entry point. Parses OR expressions (lowest precedence).
func (p *parser) parse() (ExprNode, error) {
	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	return node, nil
}

// parseOr handles: expr OR expr OR expr ...
func (p *parser) parseOr() (ExprNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	var children []ExprNode
	children = append(children, left)

	for p.current().Type == TokenOR {
		p.advance() // consume OR
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		children = append(children, right)
	}

	if len(children) == 1 {
		return children[0], nil
	}

	return &LogicalExpr{Op: OpOr, Children: children}, nil
}

// parseAnd handles: expr AND expr AND expr ...
func (p *parser) parseAnd() (ExprNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	var children []ExprNode
	children = append(children, left)

	for p.current().Type == TokenAND {
		p.advance() // consume AND
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		children = append(children, right)
	}

	if len(children) == 1 {
		return children[0], nil
	}

	return &LogicalExpr{Op: OpAnd, Children: children}, nil
}

// parseNot handles: NOT expr
func (p *parser) parseNot() (ExprNode, error) {
	if p.current().Type == TokenNOT {
		p.advance()                // consume NOT
		child, err := p.parseNot() // right-associative: NOT NOT x
		if err != nil {
			return nil, err
		}
		return &LogicalExpr{Op: OpNot, Children: []ExprNode{child}}, nil
	}
	return p.parsePrimary()
}

// parsePrimary handles parenthesized expressions or comparison expressions.
func (p *parser) parsePrimary() (ExprNode, error) {
	if p.current().Type == TokenLParen {
		p.advance() // consume (
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(TokenRParen)
		if err != nil {
			return nil, err
		}
		return node, nil
	}

	return p.parseComparison()
}

// parseComparison handles: field OP value
func (p *parser) parseComparison() (ExprNode, error) {
	// Field name
	fieldTok := p.current()
	if fieldTok.Type != TokenIdent {
		return nil, fmt.Errorf("expected field name, got %s (%q)", fieldTok.Type, fieldTok.Literal)
	}
	field := fieldTok.Literal
	p.advance()

	// Check for EXISTS keyword as prefix: EXISTS field
	// Actually, EXISTS is used as: field EXISTS (no value needed)
	// But also supports: EXISTS(field) — let's do: field EXISTS

	// Operator
	opTok := p.current()

	// Handle: field EXISTS (no value)
	if opTok.Type == TokenEXISTS {
		p.advance()
		return &ComparisonExpr{Field: field, Op: OpExists, Value: true}, nil
	}

	// Handle: field IS NULL / field IS NOT NULL
	if opTok.Type == TokenIS {
		p.advance()
		if p.current().Type == TokenNOT {
			p.advance()
			if _, err := p.expect(TokenNULL); err != nil {
				return nil, fmt.Errorf("after IS NOT, expected NULL: %w", err)
			}
			return &ComparisonExpr{Field: field, Op: OpIsNotNull, Value: nil}, nil
		}
		if _, err := p.expect(TokenNULL); err != nil {
			return nil, fmt.Errorf("after IS, expected NULL: %w", err)
		}
		return &ComparisonExpr{Field: field, Op: OpIsNull, Value: nil}, nil
	}

	// Handle: field NOT IN (...)
	isNot := false
	if opTok.Type == TokenNOT {
		p.advance()
		isNot = true
		opTok = p.current()
	}

	if isNot && opTok.Type == TokenIN {
		p.advance()
		values, err := p.parseValueList()
		if err != nil {
			return nil, err
		}
		return &ComparisonExpr{Field: field, Op: OpNotIn, Value: values}, nil
	}

	// Standard binary operators
	var op Op
	switch opTok.Type {
	case TokenEQ:
		op = OpEq
	case TokenNE:
		op = OpNe
	case TokenGT:
		op = OpGt
	case TokenGTE:
		op = OpGte
	case TokenLT:
		op = OpLt
	case TokenLTE:
		op = OpLte
	case TokenLIKE:
		op = OpLike
	case TokenIN:
		p.advance()
		values, err := p.parseValueList()
		if err != nil {
			return nil, err
		}
		return &ComparisonExpr{Field: field, Op: OpIn, Value: values}, nil
	case TokenCONTAINS:
		op = OpContains
	case TokenPREFIX:
		op = OpPrefix
	case TokenSUFFIX:
		op = OpSuffix
	case TokenBETWEEN:
		p.advance()
		low, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenAND); err != nil {
			return nil, fmt.Errorf("BETWEEN requires AND: %w", err)
		}
		high, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &ComparisonExpr{Field: field, Op: OpBetween, Value: []any{low, high}}, nil
	case TokenANYOF:
		p.advance()
		values, err := p.parseValueList()
		if err != nil {
			return nil, err
		}
		return &ComparisonExpr{Field: field, Op: OpAnyOf, Value: values}, nil
	case TokenALLOF:
		p.advance()
		values, err := p.parseValueList()
		if err != nil {
			return nil, err
		}
		return &ComparisonExpr{Field: field, Op: OpAllOf, Value: values}, nil
	case TokenHASLEN:
		op = OpHasLen
	default:
		return nil, fmt.Errorf("expected operator, got %s (%q)", opTok.Type, opTok.Literal)
	}

	p.advance() // consume operator

	// Parse value
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	return &ComparisonExpr{Field: field, Op: op, Value: value}, nil
}

// parseValueList parses a parenthesized comma-separated list: ('a', 'b', 3)
func (p *parser) parseValueList() ([]any, error) {
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	var values []any

	// Empty list is valid
	if p.current().Type == TokenRParen {
		p.advance()
		return values, nil
	}

	for {
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		values = append(values, v)

		if p.current().Type == TokenComma {
			p.advance()
			continue
		}
		break
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return values, nil
}

// parseValue parses a single value: string, number, or identifier.
func (p *parser) parseValue() (any, error) {
	tok := p.current()
	switch tok.Type {
	case TokenString:
		p.advance()
		return tok.Literal, nil
	case TokenNumber:
		p.advance()
		// Try int first, then float
		if i, err := strconv.ParseInt(tok.Literal, 10, 64); err == nil {
			return i, nil
		}
		f, err := strconv.ParseFloat(tok.Literal, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", tok.Literal, err)
		}
		return f, nil
	case TokenIdent:
		p.advance()
		// Recognize boolean and null literals
		switch tok.Literal {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null", "NULL":
			return nil, nil
		}
		return tok.Literal, nil
	default:
		return nil, fmt.Errorf("expected value, got %s (%q)", tok.Type, tok.Literal)
	}
}
