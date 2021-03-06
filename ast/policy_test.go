// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestModuleJSONRoundTrip(t *testing.T) {
	mod := MustParseModule(`
	package a.b.c
	import data.x.y as z
	import data.u.i
	p = [1,2,{"foo":3}] :- r[x] = 1, not q[x]
	r[y] = v :- i[1] = y, v = i[2]
	q[x] :- a=[true,false,null,{"x":[1,2,3]}], a[i] = x
	t = true :- xs = [{"x": a[i].a} | a[i].n = "bob", b[x]]
	`)

	bs, err := json.Marshal(mod)
	if err != nil {
		panic(err)
	}

	roundtrip := &Module{}

	err = json.Unmarshal(bs, roundtrip)
	if err != nil {
		panic(err)
	}

	if !roundtrip.Equal(mod) {
		t.Errorf("Expected roundtripped module to be equal to original:\nExpected:\n\n%v\n\nGot:\n\n%v\n", mod, roundtrip)
	}
}

func TestPackageEquals(t *testing.T) {
	pkg1 := &Package{Path: RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")).Value.(Ref)}
	pkg2 := &Package{Path: RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")).Value.(Ref)}
	pkg3 := &Package{Path: RefTerm(VarTerm("foo"), StringTerm("qux"), StringTerm("baz")).Value.(Ref)}
	assertPackagesEqual(t, pkg1, pkg1)
	assertPackagesEqual(t, pkg1, pkg2)
	assertPackagesNotEqual(t, pkg1, pkg3)
	assertPackagesNotEqual(t, pkg2, pkg3)
}

func TestPackageString(t *testing.T) {
	pkg1 := &Package{Path: RefTerm(VarTerm("foo"), StringTerm("bar"), StringTerm("baz")).Value.(Ref)}
	result1 := pkg1.String()
	expected1 := "package bar.baz"
	if result1 != expected1 {
		t.Errorf("Expected %v but got %v", expected1, result1)
	}
}

func TestImportEquals(t *testing.T) {
	imp1 := &Import{Path: VarTerm("foo"), Alias: Var("bar")}
	imp11 := &Import{Path: VarTerm("foo"), Alias: Var("bar")}
	imp2 := &Import{Path: VarTerm("foo")}
	imp3 := &Import{Path: RefTerm(VarTerm("bar"), VarTerm("baz"), VarTerm("qux")), Alias: Var("corge")}
	imp33 := &Import{Path: RefTerm(VarTerm("bar"), VarTerm("baz"), VarTerm("qux")), Alias: Var("corge")}
	imp4 := &Import{Path: RefTerm(VarTerm("bar"), VarTerm("baz"), VarTerm("qux"))}
	assertImportsEqual(t, imp1, imp1)
	assertImportsEqual(t, imp1, imp11)
	assertImportsEqual(t, imp3, imp3)
	assertImportsEqual(t, imp3, imp33)
	imps := []*Import{imp1, imp2, imp3, imp4}
	for i := range imps {
		for j := range imps {
			if i != j {
				assertImportsNotEqual(t, imps[i], imps[j])
			}
		}
	}
}

func TestImportString(t *testing.T) {
	imp1 := &Import{Path: VarTerm("foo"), Alias: Var("bar")}
	imp2 := &Import{Path: VarTerm("foo")}
	imp3 := &Import{Path: RefTerm(VarTerm("bar"), StringTerm("baz"), StringTerm("qux")), Alias: Var("corge")}
	imp4 := &Import{Path: RefTerm(VarTerm("bar"), StringTerm("baz"), StringTerm("qux"))}
	assertImportToString(t, imp1, "import foo as bar")
	assertImportToString(t, imp2, "import foo")
	assertImportToString(t, imp3, "import bar.baz.qux as corge")
	assertImportToString(t, imp4, "import bar.baz.qux")
}

func TestExprEquals(t *testing.T) {

	// Scalars
	expr1 := &Expr{Terms: BooleanTerm(true)}
	expr2 := &Expr{Terms: BooleanTerm(true)}
	expr3 := &Expr{Terms: StringTerm("true")}
	assertExprEqual(t, expr1, expr2)
	assertExprNotEqual(t, expr1, expr3)

	// Vars, refs, and composites
	ref1 := RefTerm(VarTerm("foo"), StringTerm("bar"), VarTerm("i"))
	ref2 := RefTerm(VarTerm("foo"), StringTerm("bar"), VarTerm("i"))
	obj1 := ObjectTerm(Item(ref1, ArrayTerm(NumberTerm(1), NullTerm())))
	obj2 := ObjectTerm(Item(ref2, ArrayTerm(NumberTerm(1), NullTerm())))
	obj3 := ObjectTerm(Item(ref2, ArrayTerm(StringTerm("1"), NullTerm())))
	expr10 := &Expr{Terms: obj1}
	expr11 := &Expr{Terms: obj2}
	expr12 := &Expr{Terms: obj3}
	assertExprEqual(t, expr10, expr11)
	assertExprNotEqual(t, expr10, expr12)

	// Builtins and negation
	expr20 := &Expr{
		Negated: true,
		Terms:   []*Term{VarTerm("="), VarTerm("x"), ref1},
	}
	expr21 := &Expr{
		Negated: true,
		Terms:   []*Term{VarTerm("="), VarTerm("x"), ref1},
	}
	expr22 := &Expr{
		Negated: false,
		Terms:   []*Term{VarTerm("="), VarTerm("x"), ref1},
	}
	expr23 := &Expr{
		Negated: true,
		Terms:   []*Term{VarTerm("="), VarTerm("y"), ref1},
	}
	assertExprEqual(t, expr20, expr21)
	assertExprNotEqual(t, expr20, expr22)
	assertExprNotEqual(t, expr20, expr23)
}

func TestBodyIsGround(t *testing.T) {
	if MustParseBody(`a.b[0] = 1, a = [1,2,x]`).IsGround() {
		t.Errorf("Expected body to be non-ground")
	}
}

func TestExprOutputVars(t *testing.T) {

	tests := []struct {
		note     string
		expr     string
		safe     string
		expected string
	}{
		{"ref 1", "a[i].b[j]", "[a]", "[i, j]"},
		{"ref 2", "[1,2,a[i]]", "[a]", "[i]"},
		{"simple unify", `{"a": [{x: y}, b[z]]} = c[i]`, "[b, c]", "[y, z, i]"},
		{"built-in", "count([], x)", "[]", "[x]"},
	}

	for i, tc := range tests {

		expr := MustParseBody(tc.expr)[0]
		safe := VarSet{}
		for _, x := range MustParseTerm(tc.safe).Value.(Array) {
			safe.Add(x.Value.(Var))
		}

		result := expr.OutputVars(safe)

		expected := VarSet{}
		for _, x := range MustParseTerm(tc.expected).Value.(Array) {
			expected.Add(x.Value.(Var))
		}

		missing := expected.Diff(result)
		extra := result.Diff(expected)
		if len(missing) != 0 || len(extra) != 0 {
			t.Errorf("%s (%d): Missing output vars: %v, extra output vars: %v", tc.note, i, missing, extra)
		}
	}
}

func TextExprString(t *testing.T) {
	expr1 := &Expr{
		Terms: RefTerm(VarTerm("q"), StringTerm("r"), VarTerm("x")),
	}
	expr2 := &Expr{
		Negated: true,
		Terms:   RefTerm(VarTerm("q"), StringTerm("r"), VarTerm("x")),
	}
	expr3 := &Expr{
		Terms: []*Term{VarTerm("="), StringTerm("a"), NumberTerm(17.1)},
	}
	expr4 := &Expr{
		Terms: []*Term{
			VarTerm("!="),
			ObjectTerm(Item(VarTerm("foo"), ArrayTerm(
				NumberTerm(1), RefTerm(VarTerm("a"), StringTerm("b")),
			))),
			BooleanTerm(false),
		},
	}
	assertExprString(t, expr1, "q.r[x]")
	assertExprString(t, expr2, "not q.r[x]")
	assertExprString(t, expr3, "eq(\"a\", 17.1)")
	assertExprString(t, expr4, "ne({foo: [1, a.b]}, false)")
}

func TestExprBadJSON(t *testing.T) {

	assert := func(js string, exp error) {
		expr := Expr{}
		err := json.Unmarshal([]byte(js), &expr)
		if !reflect.DeepEqual(exp, err) {
			t.Errorf("Expected %v but got: %v", exp, err)
		}
	}

	js := `
	{
		"Negated": 100,
		"Terms": {
			"Value": "foo",
			"Type": "string"
		},
		"Index": 0
	}
	`

	exp := fmt.Errorf("ast: unable to unmarshal Negated field with type: float64 (expected true or false)")
	assert(js, exp)

	js = `
	{
		"Terms": [
			"foo"
		],
		"Index": 0
	}
	`
	exp = fmt.Errorf("ast: unable to unmarshal term")
	assert(js, exp)

	js = `
	{
		"Terms": "bad value",
		"Index": 0
	}
	`
	exp = fmt.Errorf(`ast: unable to unmarshal Terms field with type: string (expected {"Value": ..., "Type": ...} or [{"Value": ..., "Type": ...}, ...])`)
	assert(js, exp)

	js = `
	{
		"Terms": {"Value": "foo", "Type": "string"}
	}`
	exp = fmt.Errorf("ast: unable to unmarshal Index field with type: <nil> (expected integer)")
	assert(js, exp)
}

func TestRuleHeadEquals(t *testing.T) {
	assertRulesEqual(t, &Rule{}, &Rule{})

	// Same name/key/value
	assertRulesEqual(t, &Rule{Name: Var("p")}, &Rule{Name: Var("p")})
	assertRulesEqual(t, &Rule{Key: VarTerm("x")}, &Rule{Key: VarTerm("x")})
	assertRulesEqual(t, &Rule{Value: VarTerm("x")}, &Rule{Value: VarTerm("x")})

	// Different name/key/value
	assertRulesNotEqual(t, &Rule{Name: Var("p")}, &Rule{Name: Var("q")})
	assertRulesNotEqual(t, &Rule{Key: VarTerm("x")}, &Rule{Key: VarTerm("y")})
	assertRulesNotEqual(t, &Rule{Value: VarTerm("x")}, &Rule{Value: VarTerm("y")})
}

func TestRuleBodyEquals(t *testing.T) {

	true1 := &Expr{Terms: []*Term{BooleanTerm(true)}}
	true2 := &Expr{Terms: []*Term{BooleanTerm(true)}}
	false1 := &Expr{Terms: []*Term{BooleanTerm(false)}}

	ruleTrue1 := &Rule{Body: NewBody(true1)}
	ruleTrue12 := &Rule{Body: NewBody(true1, true2)}
	ruleTrue2 := &Rule{Body: NewBody(true2)}
	ruleTrue12_2 := &Rule{Body: NewBody(true1, true2)}
	ruleFalse1 := &Rule{Body: NewBody(false1)}
	ruleTrueFalse := &Rule{Body: NewBody(true1, false1)}
	ruleFalseTrue := &Rule{Body: NewBody(false1, true1)}

	// Same expressions
	assertRulesEqual(t, ruleTrue1, ruleTrue2)
	assertRulesEqual(t, ruleTrue12, ruleTrue12_2)

	// Different expressions/different order
	assertRulesNotEqual(t, ruleTrue1, ruleFalse1)
	assertRulesNotEqual(t, ruleTrueFalse, ruleFalseTrue)
}

func TestRuleString(t *testing.T) {

	rule1 := &Rule{
		Name: Var("p"),
		Body: NewBody(
			Equality.Expr(StringTerm("foo"), StringTerm("bar")),
		),
	}

	rule2 := &Rule{
		Name:  Var("p"),
		Key:   VarTerm("x"),
		Value: VarTerm("y"),
		Body: NewBody(
			Equality.Expr(StringTerm("foo"), VarTerm("x")),
			&Expr{
				Negated: true,
				Terms:   RefTerm(VarTerm("a"), StringTerm("b"), VarTerm("x")),
			},
			Equality.Expr(StringTerm("b"), VarTerm("y")),
		),
	}

	assertRuleString(t, rule1, "p :- eq(\"foo\", \"bar\")")
	assertRuleString(t, rule2, "p[x] = y :- eq(\"foo\", x), not a.b[x], eq(\"b\", y)")
}

func TestModuleString(t *testing.T) {

	input := `
	package a.b.c

	import data.foo.bar
	import xyz

	p :- not bar
	q :- xyz.abc = 2
	wildcard :- bar[_] = 1
	`

	mod := MustParseModule(input)

	roundtrip, err := ParseModule("", mod.String())
	if err != nil {
		t.Fatalf("Unexpected error while parsing roundtripped module: %v", err)
	}

	if !roundtrip.Equal(mod) {
		t.Fatalf("Expected roundtripped to equal original but:\n\nExpected:\n\n%v\n\nDoes not equal result:\n\n%v", mod, roundtrip)
	}
}

func assertExprEqual(t *testing.T, a, b *Expr) {
	if !a.Equal(b) {
		t.Errorf("Expressions are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertExprNotEqual(t *testing.T, a, b *Expr) {
	if a.Equal(b) {
		t.Errorf("Expressions are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertExprString(t *testing.T, expr *Expr, expected string) {
	result := expr.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}

func assertImportsEqual(t *testing.T, a, b *Import) {
	if !a.Equal(b) {
		t.Errorf("Imports are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertImportsNotEqual(t *testing.T, a, b *Import) {
	if a.Equal(b) {
		t.Errorf("Imports are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertImportToString(t *testing.T, imp *Import, expected string) {
	result := imp.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}

func assertPackagesEqual(t *testing.T, a, b *Package) {
	if !a.Equal(b) {
		t.Errorf("Packages are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertPackagesNotEqual(t *testing.T, a, b *Package) {
	if a.Equal(b) {
		t.Errorf("Packages are not equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertRulesEqual(t *testing.T, a, b *Rule) {
	if !a.Equal(b) {
		t.Errorf("Rules are not equal (expected equal): a=%v b=%v", a, b)
	}
}

func assertRulesNotEqual(t *testing.T, a, b *Rule) {
	if a.Equal(b) {
		t.Errorf("Rules are equal (expected not equal): a=%v b=%v", a, b)
	}
}

func assertRuleString(t *testing.T, rule *Rule, expected string) {
	result := rule.String()
	if result != expected {
		t.Errorf("Expected %v but got %v", expected, result)
	}
}
