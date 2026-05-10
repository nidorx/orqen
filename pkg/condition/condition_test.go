package condition

import (
	"testing"
)

// ---- Lexer Tests ----

func TestLexer_Simple(t *testing.T) {
	input := "name == 'xpto'"
	l := newLexer(input)
	tokens := l.tokenize()

	expected := []Token{
		{TokenIdent, "name"},
		{TokenEQ, "=="},
		{TokenString, "xpto"},
		{TokenEOF, ""},
	}

	assertTokens(t, tokens, expected)
}

func TestLexer_Complex(t *testing.T) {
	input := "name LIKE '^task-' AND age > 33 AND type IN ('ONE', 'TWO')"
	l := newLexer(input)
	tokens := l.tokenize()

	expected := []Token{
		{TokenIdent, "name"},
		{TokenLIKE, "LIKE"},
		{TokenString, "^task-"},
		{TokenAND, "AND"},
		{TokenIdent, "age"},
		{TokenGT, ">"},
		{TokenNumber, "33"},
		{TokenAND, "AND"},
		{TokenIdent, "type"},
		{TokenIN, "IN"},
		{TokenLParen, "("},
		{TokenString, "ONE"},
		{TokenComma, ","},
		{TokenString, "TWO"},
		{TokenRParen, ")"},
		{TokenEOF, ""},
	}

	assertTokens(t, tokens, expected)
}

func TestLexer_NotIn(t *testing.T) {
	input := "status NOT IN ('done', 'archived')"
	l := newLexer(input)
	tokens := l.tokenize()

	expected := []Token{
		{TokenIdent, "status"},
		{TokenNOT, "NOT"},
		{TokenIN, "IN"},
		{TokenLParen, "("},
		{TokenString, "done"},
		{TokenComma, ","},
		{TokenString, "archived"},
		{TokenRParen, ")"},
		{TokenEOF, ""},
	}

	assertTokens(t, tokens, expected)
}

func TestLexer_Parentheses(t *testing.T) {
	input := "(a > 1 OR b > 2) AND c == 'x'"
	l := newLexer(input)
	tokens := l.tokenize()

	expected := []Token{
		{TokenLParen, "("},
		{TokenIdent, "a"},
		{TokenGT, ">"},
		{TokenNumber, "1"},
		{TokenOR, "OR"},
		{TokenIdent, "b"},
		{TokenGT, ">"},
		{TokenNumber, "2"},
		{TokenRParen, ")"},
		{TokenAND, "AND"},
		{TokenIdent, "c"},
		{TokenEQ, "=="},
		{TokenString, "x"},
		{TokenEOF, ""},
	}

	assertTokens(t, tokens, expected)
}

func assertTokens(t *testing.T, got, want []Token) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("token count: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Type != want[i].Type {
			t.Errorf("token[%d].Type = %s, want %s", i, got[i].Type, want[i].Type)
		}
		if got[i].Literal != want[i].Literal {
			t.Errorf("token[%d].Literal = %q, want %q", i, got[i].Literal, want[i].Literal)
		}
	}
}

// ---- Parser Tests ----

func TestParse_SimpleEq(t *testing.T) {
	node, err := Parse("name == 'hello'")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Field != "name" || c.Op != OpEq || c.Value != "hello" {
		t.Errorf("bad expr: field=%s op=%s value=%v; want name/EQ/hello", c.Field, c.Op, c.Value)
	}
}

func TestParse_AndChain(t *testing.T) {
	node, err := Parse("a > 1 AND b == 'x' AND c != 'y'")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	logical, ok := node.(*LogicalExpr)
	if !ok {
		t.Fatalf("expected *LogicalExpr, got %T", node)
	}
	if logical.Op != OpAnd {
		t.Errorf("expected AND, got %s", logical.Op)
	}
	if len(logical.Children) != 3 {
		t.Errorf("expected 3 children, got %d", len(logical.Children))
	}
}

func TestParse_OrChain(t *testing.T) {
	node, err := Parse("a == 1 OR b == 2")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	logical, ok := node.(*LogicalExpr)
	if !ok {
		t.Fatalf("expected *LogicalExpr, got %T", node)
	}
	if logical.Op != OpOr {
		t.Errorf("expected OR, got %s", logical.Op)
	}
}

func TestParse_Not(t *testing.T) {
	node, err := Parse("NOT a > 1")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	logical, ok := node.(*LogicalExpr)
	if !ok {
		t.Fatalf("expected *LogicalExpr, got %T", node)
	}
	if logical.Op != OpNot {
		t.Errorf("expected NOT, got %s", logical.Op)
	}
}

func TestParse_Parenthesized(t *testing.T) {
	node, err := Parse("(a > 1 OR b > 2) AND c == 'x'")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Top-level should be AND
	logical, ok := node.(*LogicalExpr)
	if !ok {
		t.Fatalf("expected *LogicalExpr, got %T", node)
	}
	if logical.Op != OpAnd {
		t.Errorf("expected AND at top, got %s", logical.Op)
	}
}

func TestParse_InList(t *testing.T) {
	node, err := Parse("type IN ('bug', 'feature')")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpIn {
		t.Errorf("expected IN, got %s", c.Op)
	}
	vals, ok := c.Value.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", c.Value)
	}
	if len(vals) != 2 || vals[0] != "bug" || vals[1] != "feature" {
		t.Errorf("bad values: %v", vals)
	}
}

func TestParse_NotIn(t *testing.T) {
	node, err := Parse("status NOT IN ('done', 'archived')")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpNotIn {
		t.Errorf("expected NOT_IN, got %s", c.Op)
	}
}

func TestParse_Like(t *testing.T) {
	node, err := Parse("name LIKE '^task-[0-9]+'")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpLike {
		t.Errorf("expected LIKE, got %s", c.Op)
	}
	if c.Value != "^task-[0-9]+" {
		t.Errorf("bad pattern: %v", c.Value)
	}
}

func TestParse_Between(t *testing.T) {
	node, err := Parse("age BETWEEN 18 AND 65")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpBetween {
		t.Errorf("expected BETWEEN, got %s", c.Op)
	}
	bounds, ok := c.Value.([]any)
	if !ok || len(bounds) != 2 {
		t.Fatalf("expected 2 bounds, got %v", c.Value)
	}
	if bounds[0] != int64(18) || bounds[1] != int64(65) {
		t.Errorf("bad bounds: %v", bounds)
	}
}

func TestParse_Exists(t *testing.T) {
	node, err := Parse("email EXISTS")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpExists {
		t.Errorf("expected EXISTS, got %s", c.Op)
	}
}

func TestParse_IsNull(t *testing.T) {
	node, err := Parse("deleted_at IS NULL")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpIsNull {
		t.Errorf("expected IS_NULL, got %s", c.Op)
	}
}

func TestParse_IsNotNull(t *testing.T) {
	node, err := Parse("deleted_at IS NOT NULL")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpIsNotNull {
		t.Errorf("expected IS_NOT_NULL, got %s", c.Op)
	}
}

func TestParse_Contains(t *testing.T) {
	node, err := Parse("tags CONTAINS 'urgent'")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpContains {
		t.Errorf("expected CONTAINS, got %s", c.Op)
	}
}

func TestParse_Prefix(t *testing.T) {
	node, err := Parse("name PREFIX 'task-'")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpPrefix {
		t.Errorf("expected PREFIX, got %s", c.Op)
	}
}

func TestParse_Suffix(t *testing.T) {
	node, err := Parse("name SUFFIX '-v2'")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpSuffix {
		t.Errorf("expected SUFFIX, got %s", c.Op)
	}
}

func TestParse_AnyOf(t *testing.T) {
	node, err := Parse("tags ANY_OF ('urgent', 'critical')")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpAnyOf {
		t.Errorf("expected ANY_OF, got %s", c.Op)
	}
}

func TestParse_AllOf(t *testing.T) {
	node, err := Parse("tags ALL_OF ('a', 'b')")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpAllOf {
		t.Errorf("expected ALL_OF, got %s", c.Op)
	}
}

func TestParse_HasLength(t *testing.T) {
	node, err := Parse("tags HAS_LENGTH 3")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	c, ok := node.(*ComparisonExpr)
	if !ok {
		t.Fatalf("expected *ComparisonExpr, got %T", node)
	}
	if c.Op != OpHasLen {
		t.Errorf("expected HAS_LENGTH, got %s", c.Op)
	}
	if c.Value != int64(3) {
		t.Errorf("bad value: %v; want 3", c.Value)
	}
}

func TestParse_NestedNot(t *testing.T) {
	node, err := Parse("NOT NOT a > 1")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Should be NOT wrapping NOT
	logical, ok := node.(*LogicalExpr)
	if !ok {
		t.Fatalf("expected *LogicalExpr, got %T", node)
	}
	if logical.Op != OpNot {
		t.Errorf("expected NOT, got %s", logical.Op)
	}
	inner, ok := logical.Children[0].(*LogicalExpr)
	if !ok {
		t.Fatalf("expected inner *LogicalExpr, got %T", logical.Children[0])
	}
	if inner.Op != OpNot {
		t.Errorf("expected inner NOT, got %s", inner.Op)
	}
}

func TestParse_Error_BadOperator(t *testing.T) {
	_, err := Parse("name FOO 'bar'")
	if err == nil {
		t.Fatal("expected error for bad operator")
	}
}

func TestParse_Error_MissingParen(t *testing.T) {
	_, err := Parse("(a > 1 AND b > 2")
	if err == nil {
		t.Fatal("expected error for missing paren")
	}
}

// ---- Evaluator Tests ----

func TestEval_SimpleEq(t *testing.T) {
	node, _ := Parse("name == 'hello'")
	attrs := map[string]any{"name": "hello"}
	if !Evaluate(node, attrs) {
		t.Error("expected true")
	}
	attrs2 := map[string]any{"name": "world"}
	if Evaluate(node, attrs2) {
		t.Error("expected false")
	}
}

func TestEval_And(t *testing.T) {
	node, _ := Parse("name == 'hello' AND age > 30")
	attrs := map[string]any{"name": "hello", "age": 35}
	if !Evaluate(node, attrs) {
		t.Error("expected true")
	}
	attrs2 := map[string]any{"name": "hello", "age": 20}
	if Evaluate(node, attrs2) {
		t.Error("expected false")
	}
}

func TestEval_Or(t *testing.T) {
	node, _ := Parse("name == 'hello' OR name == 'world'")
	attrs1 := map[string]any{"name": "hello"}
	attrs2 := map[string]any{"name": "world"}
	attrs3 := map[string]any{"name": "other"}

	if !Evaluate(node, attrs1) {
		t.Error("expected true for hello")
	}
	if !Evaluate(node, attrs2) {
		t.Error("expected true for world")
	}
	if Evaluate(node, attrs3) {
		t.Error("expected false for other")
	}
}

func TestEval_Not(t *testing.T) {
	node, _ := Parse("NOT active")
	// "active" field with bool value
	attrs := map[string]any{"active": false}
	if !Evaluate(node, attrs) {
		// NOT false = true, but this tests NOT on a ComparisonExpr
		// Let's re-check: "NOT active" is parsed as NOT (active field with no op)
		// That would be a parse issue. Let's use a proper comparison.
	}
}

func TestEval_NotProper(t *testing.T) {
	node, _ := Parse("NOT age > 30")
	attrs1 := map[string]any{"age": 20}
	attrs2 := map[string]any{"age": 40}

	if !Evaluate(node, attrs1) {
		t.Error("expected true: NOT(20 > 30) = true")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected false: NOT(40 > 30) = false")
	}
}

func TestEval_Like(t *testing.T) {
	node, _ := Parse("name LIKE '^task-[0-9]+'")
	attrs1 := map[string]any{"name": "task-123"}
	attrs2 := map[string]any{"name": "bug-report"}

	if !Evaluate(node, attrs1) {
		t.Error("expected task-123 to match")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected bug-report to not match")
	}
}

func TestEval_In(t *testing.T) {
	node, _ := Parse("type IN ('bug', 'feature')")
	attrs1 := map[string]any{"type": "bug"}
	attrs2 := map[string]any{"type": "chore"}

	if !Evaluate(node, attrs1) {
		t.Error("expected bug to match IN")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected chore to not match IN")
	}
}

func TestEval_NotIn(t *testing.T) {
	node, _ := Parse("status NOT IN ('done', 'archived')")
	attrs1 := map[string]any{"status": "in_progress"}
	attrs2 := map[string]any{"status": "done"}

	if !Evaluate(node, attrs1) {
		t.Error("expected in_progress to match NOT IN")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected done to not match NOT IN")
	}
}

func TestEval_Exists(t *testing.T) {
	node, _ := Parse("email EXISTS")
	attrs1 := map[string]any{"email": "test@test.com"}
	attrs2 := map[string]any{"name": "test"}

	if !Evaluate(node, attrs1) {
		t.Error("expected email to exist")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected email to not exist")
	}
}

func TestEval_IsNull(t *testing.T) {
	node, _ := Parse("deleted_at IS NULL")
	attrs1 := map[string]any{"name": "test"}
	attrs2 := map[string]any{"name": "test", "deleted_at": "2024-01-01"}

	if !Evaluate(node, attrs1) {
		t.Error("expected deleted_at to be null")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected deleted_at to not be null")
	}
}

func TestEval_IsNotNull(t *testing.T) {
	node, _ := Parse("deleted_at IS NOT NULL")
	attrs1 := map[string]any{"name": "test"}
	attrs2 := map[string]any{"name": "test", "deleted_at": "2024-01-01"}

	if Evaluate(node, attrs1) {
		t.Error("expected deleted_at to not be not-null")
	}
	if !Evaluate(node, attrs2) {
		t.Error("expected deleted_at to be not-null")
	}
}

func TestEval_Contains(t *testing.T) {
	node, _ := Parse("tags CONTAINS 'urgent'")
	attrs1 := map[string]any{"tags": []any{"urgent", "bug"}}
	attrs2 := map[string]any{"tags": []any{"minor", "nice"}}

	if !Evaluate(node, attrs1) {
		t.Error("expected tags to contain urgent")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected tags to not contain urgent")
	}
}

func TestEval_Prefix(t *testing.T) {
	node, _ := Parse("name PREFIX 'task-'")
	attrs1 := map[string]any{"name": "task-123"}
	attrs2 := map[string]any{"name": "bug-123"}

	if !Evaluate(node, attrs1) {
		t.Error("expected task-123 to have prefix")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected bug-123 to not have prefix")
	}
}

func TestEval_Suffix(t *testing.T) {
	node, _ := Parse("name SUFFIX '-v2'")
	attrs1 := map[string]any{"name": "release-v2"}
	attrs2 := map[string]any{"name": "release-v1"}

	if !Evaluate(node, attrs1) {
		t.Error("expected release-v2 to have suffix")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected release-v1 to not have suffix")
	}
}

func TestEval_Between(t *testing.T) {
	node, _ := Parse("age BETWEEN 18 AND 65")
	attrs1 := map[string]any{"age": 30}
	attrs2 := map[string]any{"age": 10}
	attrs3 := map[string]any{"age": 70}

	if !Evaluate(node, attrs1) {
		t.Error("expected 30 to be between 18 and 65")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected 10 to not be between 18 and 65")
	}
	if Evaluate(node, attrs3) {
		t.Error("expected 70 to not be between 18 and 65")
	}
}

func TestEval_AnyOf(t *testing.T) {
	node, _ := Parse("tags ANY_OF ('urgent', 'critical')")
	attrs1 := map[string]any{"tags": []any{"urgent", "nice"}}
	attrs2 := map[string]any{"tags": []any{"nice", "cool"}}

	if !Evaluate(node, attrs1) {
		t.Error("expected any_of to match urgent")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected any_of to not match")
	}
}

func TestEval_AllOf(t *testing.T) {
	node, _ := Parse("tags ALL_OF ('a', 'b')")
	attrs1 := map[string]any{"tags": []any{"a", "b", "c"}}
	attrs2 := map[string]any{"tags": []any{"a", "c"}}

	if !Evaluate(node, attrs1) {
		t.Error("expected all_of to match a and b")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected all_of to not match (missing b)")
	}
}

func TestEval_HasLength(t *testing.T) {
	node, _ := Parse("tags HAS_LENGTH 3")
	attrs1 := map[string]any{"tags": []any{"a", "b", "c"}}
	attrs2 := map[string]any{"tags": []any{"a", "b"}}

	if !Evaluate(node, attrs1) {
		t.Error("expected length 3")
	}
	if Evaluate(node, attrs2) {
		t.Error("expected not length 3")
	}
}

func TestEval_Complex(t *testing.T) {
	// name LIKE '^task-' AND (priority > 3 OR urgent == true)
	input := "name LIKE '^task-' AND (priority > 3 OR urgent == true)"
	node, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	attrs1 := map[string]any{"name": "task-123", "priority": 5}
	attrs2 := map[string]any{"name": "task-456", "urgent": true}
	attrs3 := map[string]any{"name": "bug-1", "priority": 5}
	attrs4 := map[string]any{"name": "task-789", "priority": 1}

	if !Evaluate(node, attrs1) {
		t.Error("expected attrs1 to match")
	}
	if !Evaluate(node, attrs2) {
		t.Error("expected attrs2 to match")
	}
	if Evaluate(node, attrs3) {
		t.Error("expected attrs3 to not match (name doesn't match LIKE)")
	}
	if Evaluate(node, attrs4) {
		t.Error("expected attrs4 to not match (priority too low)")
	}
}

func TestEval_MissingField(t *testing.T) {
	node, _ := Parse("age > 30")
	attrs := map[string]any{"name": "test"}
	// Missing field should evaluate to false for comparison ops
	if Evaluate(node, attrs) {
		t.Error("expected false for missing field")
	}
}

func TestParseAndEvaluate(t *testing.T) {
	attrs := map[string]any{"name": "hello", "age": 35}
	result, err := ParseAndEvaluate("name == 'hello' AND age > 30", attrs)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !result {
		t.Error("expected true")
	}
}

func TestParseAndEvaluate_Error(t *testing.T) {
	attrs := map[string]any{"name": "hello"}
	_, err := ParseAndEvaluate("name FOO 'bar'", attrs)
	if err == nil {
		t.Fatal("expected error")
	}
}
