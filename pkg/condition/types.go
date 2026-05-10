// Package condition provides a SQL-like DSL for filtering Attributes.
//
// The condition language supports:
//   - Comparison: =, !=, >, >=, <, <=
//   - Pattern matching: LIKE (Go regex)
//   - Set membership: IN, NOT IN
//   - Existence: EXISTS
//   - String operators: CONTAINS, PREFIX, SUFFIX
//   - Range: BETWEEN
//   - Null check: IS NULL, IS NOT NULL
//   - Array operators: ANY_OF, ALL_OF, HAS_LENGTH
//   - Logical: AND, OR, NOT, parentheses for grouping
//
// Examples:
//
//	name LIKE "^task-" AND priority > 3
//	type IN ('bug', 'feature') AND status != 'done'
//	NOT (age > 33) AND tags ANY_OF ('urgent', 'critical')
package condition

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nidorx/orqen/pkg/utils"
	"github.com/nidorx/orqen/pkg/utils/tinylfu"
)

// ---- Token Types ----

type TokenType string

const (
	TokenEOF     TokenType = "EOF"
	TokenIllegal TokenType = "ILLEGAL"

	// Literals
	TokenIdent  TokenType = "IDENT"  // field names, keywords
	TokenString TokenType = "STRING" // 'quoted string'
	TokenNumber TokenType = "NUMBER" // 42, 3.14

	// Operators
	TokenEQ       TokenType = "=="
	TokenNE       TokenType = "!="
	TokenGT       TokenType = ">"
	TokenGTE      TokenType = ">="
	TokenLT       TokenType = "<"
	TokenLTE      TokenType = "<="
	TokenLIKE     TokenType = "LIKE"
	TokenIN       TokenType = "IN"
	TokenNOT      TokenType = "NOT"
	TokenEXISTS   TokenType = "EXISTS"
	TokenCONTAINS TokenType = "CONTAINS"
	TokenPREFIX   TokenType = "PREFIX"
	TokenSUFFIX   TokenType = "SUFFIX"
	TokenBETWEEN  TokenType = "BETWEEN"
	TokenAND      TokenType = "AND"
	TokenOR       TokenType = "OR"
	TokenIS       TokenType = "IS"
	TokenNULL     TokenType = "NULL"
	TokenANYOF    TokenType = "ANY_OF"
	TokenALLOF    TokenType = "ALL_OF"
	TokenHASLEN   TokenType = "HAS_LENGTH"

	// Delimiters
	TokenLParen TokenType = "("
	TokenRParen TokenType = ")"
	TokenComma  TokenType = ","
)

// Token represents a lexical unit.
type Token struct {
	Type    TokenType
	Literal string
}

// ---- Operator ----

// Op defines a comparison operator in the condition AST.
type Op string

const (
	OpEq        Op = "EQ"
	OpNe        Op = "NE"
	OpGt        Op = "GT"
	OpGte       Op = "GTE"
	OpLt        Op = "LT"
	OpLte       Op = "LTE"
	OpLike      Op = "LIKE"
	OpIn        Op = "IN"
	OpNotIn     Op = "NOT_IN"
	OpExists    Op = "EXISTS"
	OpContains  Op = "CONTAINS"
	OpPrefix    Op = "PREFIX"
	OpSuffix    Op = "SUFFIX"
	OpBetween   Op = "BETWEEN"
	OpIsNull    Op = "IS_NULL"
	OpIsNotNull Op = "IS_NOT_NULL"
	OpAnyOf     Op = "ANY_OF"
	OpAllOf     Op = "ALL_OF"
	OpHasLen    Op = "HAS_LENGTH"
	OpAnd       Op = "AND"
	OpOr        Op = "OR"
	OpNot       Op = "NOT"
)

// ---- AST Nodes ----

// Node is an interface for all AST nodes.
type Node interface {
	node()
	String() string
}

// ExprNode marks expression nodes.
type ExprNode interface {
	Node
	exprNode()
}

// ComparisonExpr represents: field OP value
type ComparisonExpr struct {
	Field string
	Op    Op
	Value any // single value, or []any for IN/BETWEEN
}

func (c *ComparisonExpr) exprNode() {}
func (c *ComparisonExpr) node()     {}
func (c *ComparisonExpr) String() string {
	val := formatValue(c.Op, c.Value)
	if val == "" {
		return fmt.Sprintf("%s %s", c.Field, opToSQL(c.Op))
	}
	return fmt.Sprintf("%s %s %s", c.Field, opToSQL(c.Op), val)
}

// opToSQL converts an operator to its SQL-like string representation.
func opToSQL(op Op) string {
	switch op {
	case OpEq:
		return "=="
	case OpNe:
		return "!="
	case OpGt:
		return ">"
	case OpGte:
		return ">="
	case OpLt:
		return "<"
	case OpLte:
		return "<="
	case OpLike:
		return "LIKE"
	case OpIn:
		return "IN"
	case OpNotIn:
		return "NOT IN"
	case OpExists:
		return "EXISTS"
	case OpContains:
		return "CONTAINS"
	case OpPrefix:
		return "PREFIX"
	case OpSuffix:
		return "SUFFIX"
	case OpBetween:
		return "BETWEEN"
	case OpIsNull:
		return "IS NULL"
	case OpIsNotNull:
		return "IS NOT NULL"
	case OpAnyOf:
		return "ANY_OF"
	case OpAllOf:
		return "ALL_OF"
	case OpHasLen:
		return "HAS_LENGTH"
	default:
		return string(op)
	}
}

// formatValue formats a value for canonical string output based on the operator.
func formatValue(op Op, value any) string {
	// Operators that take no value operand
	if op == OpExists || op == OpIsNull || op == OpIsNotNull {
		return ""
	}

	if value == nil {
		return "NULL"
	}

	switch op {
	case OpIn, OpNotIn, OpAnyOf, OpAllOf:
		if vals, ok := value.([]any); ok {
			parts := make([]string, len(vals))
			for i, v := range vals {
				parts[i] = formatSingleValue(v)
			}
			return "(" + strings.Join(parts, ", ") + ")"
		}
	case OpBetween:
		if vals, ok := value.([]any); ok && len(vals) == 2 {
			return fmt.Sprintf("%s AND %s", formatSingleValue(vals[0]), formatSingleValue(vals[1]))
		}
	}

	return formatSingleValue(value)
}

// formatSingleValue formats a single value for display in a condition string.
func formatSingleValue(v any) string {
	switch val := v.(type) {
	case string:
		return "'" + val + "'"
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.Itoa(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// LogicalExpr represents: AND/OR/NOT with child expressions
type LogicalExpr struct {
	Op       Op
	Children []ExprNode // for NOT, len(Children)==1; for AND/OR, len>=2
}

func (l *LogicalExpr) exprNode() {}
func (l *LogicalExpr) node()     {}
func (l *LogicalExpr) String() string {
	if l.Op == OpNot && len(l.Children) == 1 {
		return "NOT " + l.Children[0].String()
	}
	joiner := " " + string(l.Op) + " "
	var parts []string
	for _, c := range l.Children {
		parts = append(parts, c.String())
	}
	return "(" + strings.Join(parts, joiner) + ")"
}

var exprNodeCache = tinylfu.NewSyncCacheT[ExprNode](1000, 100000, 5*time.Minute)

// ---- Parse ----

// Parse parses a condition string into an AST.
// Returns an error if the string cannot be parsed.
func Parse(input string) (ExprNode, error) {
	return exprNodeCache.GetOrSet(utils.HashXxh64([]byte(input)), func() (ExprNode, error) {
		l := newLexer(input)
		tokens := l.tokenize()

		p := newParser(tokens)
		node, err := p.parse()
		if err != nil {
			return nil, err
		}

		if !p.isEOF() {
			return nil, fmt.Errorf("unexpected tokens after expression: %s", p.current().Literal)
		}

		return node, nil
	})
}

// ---- Evaluate ----

// Evaluate evaluates a condition AST against a map of attributes.
// Returns the boolean result.
func Evaluate(node ExprNode, attrs map[string]any) bool {
	switch n := node.(type) {
	case *ComparisonExpr:
		return evalComparison(n, attrs)
	case *LogicalExpr:
		return evalLogical(n, attrs)
	default:
		return false
	}
}

// ParseAndEvaluate is a convenience function that parses and evaluates in one step.
func ParseAndEvaluate(input string, attrs map[string]any) (bool, error) {
	node, err := Parse(input)
	if err != nil {
		return false, err
	}
	return Evaluate(node, attrs), nil
}
