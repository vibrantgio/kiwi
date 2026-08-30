// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"math"
	"strings"
	"testing"
)

func TestParser(t *testing.T) {
	// Parse expressions checking for syntax errors
	expr, err := ParseExpr("x + y <= 10")
	assert.Equal(t, nil, err, "err")
	assert.EqualString(t, "x + y <= INT(10)", expr.String(), "ast.String()")

	x, y := Var("x"), Var("y")

	expr, err = ParseExpr(x, "+", y, "<= 10")
	assert.Equal(t, nil, err, "err")
	assert.EqualString(t, "x + y <= INT(10)", expr.String(), "ast.String()")

	expr, err = ParseExpr("xm == 2 + 5 + 0.5 * (xr + xl) / (2 * 2)")
	assert.Equal(t, nil, err, "err")
	assert.EqualString(t, "xm == INT(2) + INT(5) + FLOAT(0.5) * (xr + xl) / (INT(2) * INT(2))", expr.String(), "ast.String()")

	expr, err = ParseExpr("xm == (2 * xr + 5 * xl + (xr + xl) + 20 * xr) / (2 * 2)")
	assert.Equal(t, nil, err, "err")
	assert.EqualString(t, "xm == (INT(2) * xr + INT(5) * xl + (xr + xl) + INT(20) * xr) / (INT(2) * INT(2))", expr.String(), "ast.String()")

	// Generate a new constraint from the ast while checking for evaluation errors
	xm, xr, xl := Var("xm"), Var("xr"), Var("xl")
	vars := []*Variable{xm, xr, xl}

	expr, err = ParseExpr("xm == 2 + 5 + 0.5 * (xr + xl) / (2 * 2)")
	assert.Equal(t, nil, err, "err")
	cns, err := expr.NewConstraint(vars, WithStrength(Strong(123)))
	assert.Equal(t, nil, err, "err")
	assert.EqualString(t, "0.125 * xr + 0.125 * xl + -1 * xm + 7 == 0 | Strength = Strong(123)", cns.String(), "cns.String()")

	expr, err = ParseExpr("xm == (2 * xr + 5 * xl + (xr + xl) + 20 * xr) / (2 * 2)")
	assert.Equal(t, nil, err, "err")
	cns, err = expr.NewConstraint(vars, WithStrength(Strong(123)))
	assert.Equal(t, nil, err, "err")
	assert.EqualString(t, "5.75 * xr + 1.5 * xl + -1 * xm + 0 == 0 | Strength = Strong(123)", cns.String(), "cns.String()")
}

func TestSimple0(t *testing.T) {
	solver := NewSolver()
	x := Var("x")

	solver.AddConstraint(x.AddConstant(2).EqualsConstant(20)) // x + 2 = 20

	solver.UpdateVariables()
	assert.EqualFloat64(t, 18, x.Value, "x.Value")
}

func TestSimple1(t *testing.T) {
	solver := NewSolver()
	x, y := Var("x"), Var("y")

	solver.AddConstraint(x.EqualsConstant(20))                                 // x = 20
	solver.AddConstraint(x.AddConstant(2).EqualsExpression(y.AddConstant(10))) // x + 2 = y + 10

	solver.UpdateVariables()
	assert.EqualFloat64(t, 12, y.Value, "y.Value")
	assert.EqualFloat64(t, 20, x.Value, "x.Value")
}

func TestSimple2(t *testing.T) {
	solver := NewSolver()
	x, y := Var("x"), Var("y")

	solver.AddConstraint(x.EqualsVariable(y)) // x == y

	solver.UpdateVariables()
	assert.EqualFloat64(t, 0, x.Value, "x.Value")
	assert.EqualFloat64(t, 0, y.Value, "y.Value")
}

func TestSimple3(t *testing.T) {
	solver := NewSolver()
	x, y := Var("x"), Var("y")

	solver.AddConstraint(x.EqualsConstant(27))                     // x = 27
	solver.AddConstraint(x.Multiply(10).EqualsTerm(y.Multiply(5))) // 10 x = 5 y

	solver.UpdateVariables()
	assert.EqualFloat64(t, 27, x.Value, "x.Value")
	assert.EqualFloat64(t, 54, y.Value, "y.Value")
}

func TestCasso0(t *testing.T) {
	x := Var("x")
	solver := NewSolver()

	err := solver.AddConstraint(x.LessThanOrEqualsConstant(10)) // x <= 10
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(x.EqualsConstant(5)) // x = 5
	assert.Equal(t, nil, err, "err")

	solver.UpdateVariables()
	assert.EqualFloat64(t, 5, x.Value, "x.Value")
}

func TestCasso1(t *testing.T) {
	x, y := Var("x"), Var("y")
	solver := NewSolver()

	err := solver.AddConstraint(x.LessThanOrEqualsVariable(y)) // x <= y
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(y.EqualsExpression(x.AddConstant(3))) // y = x + 3
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(x.EqualsConstant(10), WithStrength(WEAK)) // x = 10 | WEAK
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(y.EqualsConstant(10), WithStrength(WEAK)) // y = 10 | WEAK
	assert.Equal(t, nil, err, "err")

	solver.UpdateVariables()
	if NearZero(x.Value - 10.0) {
		assert.EqualFloat64(t, 10, x.Value, "x.Value")
		assert.EqualFloat64(t, 13, y.Value, "y.Value")
	} else {
		assert.EqualFloat64(t, 7, x.Value, "x.Value")
		assert.EqualFloat64(t, 10, y.Value, "y.Value")
	}
}

func TestAddDelete1(t *testing.T) {
	x := Var("x")
	solver := NewSolver()

	solver.AddConstraint(x.LessThanOrEqualsConstant(100), WithStrength(WEAK)) // x <= 100 | WEAK

	solver.UpdateVariables()
	assert.EqualFloat64(t, 100, x.Value, "x.Value")

	c10 := x.LessThanOrEqualsConstant(10) // x <= 10
	c20 := x.LessThanOrEqualsConstant(20) // x <= 20
	solver.AddConstraint(c10)
	solver.AddConstraint(c20)

	solver.UpdateVariables()
	assert.EqualFloat64(t, 10, x.Value, "x.Value")

	solver.RemoveConstraint(c10)

	solver.UpdateVariables()
	assert.EqualFloat64(t, 20, x.Value, "x.Value")

	solver.RemoveConstraint(c20)

	solver.UpdateVariables()
	assert.EqualFloat64(t, 100, x.Value, "x.Value")

	c10again := x.LessThanOrEqualsConstant(10.0) // x <= 10

	solver.AddConstraint(c10again)
	solver.AddConstraint(c10)

	solver.UpdateVariables()
	assert.EqualFloat64(t, 10, x.Value, "x.Value")

	solver.RemoveConstraint(c10)

	solver.UpdateVariables()
	assert.EqualFloat64(t, 10, x.Value, "x.Value")

	solver.RemoveConstraint(c10again)

	solver.UpdateVariables()
	assert.EqualFloat64(t, 100, x.Value, "x.Value")
}

func TestInconsistent1(t *testing.T) {
	x := Var("x")
	solver := NewSolver()

	err := solver.AddConstraint(x.EqualsConstant(10)) // x == 10
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(x.EqualsConstant(5)) // x == 5
	assert.NotEqual(t, nil, err, "err")
	_, ok := err.(UnsatisfiableConstraint)
	assert.Equal(t, true, ok, "_, ok := err.(UnsatisfiableConstraint); ok")
}

func TestInconsistent2(t *testing.T) {
	x := Var("x")
	solver := NewSolver()

	err := solver.AddConstraint(x.GreaterThanOrEqualsConstant(10)) // x >= 10
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(x.LessThanOrEqualsConstant(5)) // x <= 5
	assert.NotEqual(t, nil, err, "err")
	_, ok := err.(UnsatisfiableConstraint)
	assert.Equal(t, true, ok, "_, ok := err.(UnsatisfiableConstraint); ok")
}

func TestInconsistent3(t *testing.T) {
	w, x, y, z := Var("w"), Var("x"), Var("y"), Var("z")
	solver := NewSolver()

	err := solver.AddConstraint(w.GreaterThanOrEqualsConstant(10))
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(x.GreaterThanOrEqualsVariable(w))
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(y.GreaterThanOrEqualsVariable(x))
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(z.GreaterThanOrEqualsVariable(y))
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(z.GreaterThanOrEqualsConstant(8))
	assert.Equal(t, nil, err, "err")
	err = solver.AddConstraint(z.LessThanOrEqualsConstant(4))
	assert.NotEqual(t, nil, err, "err")
	_, ok := err.(UnsatisfiableConstraint)
	assert.Equal(t, true, ok, "_, ok := err.(UnsatisfiableConstraint); ok")
}

func TestStrength(t *testing.T) {
	STRENGTH := func(strong, medium, weak float64) float64 {
		var s float64
		s += math.Max(0, math.Min(1000, strong)) * 1000000
		s += math.Max(0, math.Min(1000, medium)) * 1000
		s += math.Max(0, math.Min(1000, weak))
		return s
	}

	assert.EqualFloat64(t, 1001001000, STRENGTH(1000, 1000, 1000), "REQUIRED")
	assert.EqualFloat64(t, 1000000, STRENGTH(1, 0, 0), "STRONG")
	assert.EqualFloat64(t, 1000, STRENGTH(0, 1, 0), "MEDIUM")
	assert.EqualFloat64(t, 1, STRENGTH(0, 0, 1), "WEAK")
	assert.EqualFloat64(t, 0, STRENGTH(0, 0, 0), "OPTIONAL")

	assert.EqualFloat64(t, 1001001000, float64(REQUIRED), "REQUIRED")
	assert.EqualFloat64(t, 1000000, float64(STRONG), "STRONG")
	assert.EqualFloat64(t, 1000, float64(MEDIUM), "MEDIUM")
	assert.EqualFloat64(t, 1, float64(WEAK), "WEAK")
	assert.EqualFloat64(t, 0, float64(OPTIONAL), "OPTIONAL")

	assert.EqualFloat64(t, 1000000, float64(Strong(1)), "Strong(1)")
	assert.EqualFloat64(t, 321000000, float64(Strong(321)), "Strong(321)")
	assert.EqualFloat64(t, 1000, float64(Medium(1)), "Medium(1)")
	assert.EqualFloat64(t, 321000, float64(Medium(321)), "Medium(321)")
	assert.EqualFloat64(t, 1, float64(Weak(1)), "Weak(1)")
	assert.EqualFloat64(t, 321, float64(Weak(321)), "Weak(321)")

	assert.Equal(t, true, REQUIRED > Strong(1000), "REQUIRED > Strong(1000)")
	assert.Equal(t, true, Strong(0) > Medium(1000), "Strong(0) > Medium(1000)")
	assert.Equal(t, true, Medium(0) > Weak(1000), "Medium(0) > Weak(1000)")
	assert.Equal(t, true, Weak(0) > OPTIONAL, "Weak(0) > OPTIONAL")
}

func TestEditVars(t *testing.T) {
	x1, x2, xm := Var("x1"), Var("x2"), Var("xm")

	cns := []*Constraint{
		x1.GreaterThanOrEqualsConstant(0),                    // x1 >= 0
		x2.LessThanOrEqualsConstant(100),                     // x2 <= 100
		x2.GreaterThanOrEqualsExpression(x1.AddConstant(20)), // x2 >= 20
		xm.EqualsExpression(x1.AddVariable(x2).Divide(2)),    // xm == (x1 + x2) / 2
	}

	solver := NewSolver()
	for _, c := range cns {
		err := solver.AddConstraint(c)
		assert.Equal(t, nil, err, "err")
	}

	err := solver.AddConstraint(x1.EqualsConstant(40), WithStrength(WEAK)) // x1 == 40 | WEAK
	assert.Equal(t, nil, err, "err")
	err = solver.AddEditVariable(xm, WithStrength(STRONG))
	assert.Equal(t, nil, err, "err")
	err = solver.SuggestValue(xm, 60) // xm == 60 | STRONG
	assert.Equal(t, nil, err, "err")

	solver.UpdateVariables()
	assert.EqualFloat64(t, 40, x1.Value, "x1.Value")
	assert.EqualFloat64(t, 80, x2.Value, "x2.Value")
	assert.EqualFloat64(t, 60, xm.Value, "xm.Value")
}

// Test initializing a solver.
func TestSolverCreation(t *testing.T) {
	assert.NotEqual(t, nil, NewSolver(), "NewSolver()")
}

// Test adding/removing edit variables.
func TestManagingEditVariable(t *testing.T) {
	s := NewSolver()
	foo, bar := Var("foo"), Var("bar")

	assert.Equal(t, false, s.HasEditVariable(foo), "s.HasEditVariable(foo)")

	s.AddEditVariable(foo, WithStrength(WEAK))
	assert.Equal(t, true, s.HasEditVariable(foo), "s.HasEditVariable(foo)")

	err := s.AddEditVariable(foo, WithStrength(MEDIUM))
	assert.NotEqual(t, nil, err, "err")
	_, ok := err.(DuplicateEditVariable)
	assert.Equal(t, true, ok, "_, ok := err.(DuplicateEditVariable); ok")

	err = s.RemoveEditVariable(bar)
	assert.NotEqual(t, nil, err, "err")
	_, ok = err.(UnknownEditVariable)
	assert.Equal(t, true, ok, "_, ok = err.(UnknownEditVariable); ok")

	err = s.RemoveEditVariable(foo)
	assert.Equal(t, nil, err, "err")

	assert.Equal(t, false, s.HasEditVariable(foo), "s.HasEditVariable(foo)")

	err = s.AddEditVariable(foo, WithStrength(REQUIRED))
	assert.Equal(t, BadRequiredStrength, err, "err")

	err = s.AddEditVariable(bar, WithStrength(STRONG))
	assert.Equal(t, nil, err, "err")
	assert.Equal(t, true, s.HasEditVariable(bar), "s.HasEditVariable(bar)")

	err = s.SuggestValue(foo, 10)
	_, ok = err.(UnknownEditVariable)
	assert.Equal(t, true, ok, "_, ok := err.(UnknownEditVariable); ok")

	s.Reset()

	assert.Equal(t, false, s.HasEditVariable(bar), "s.HasEditVariable(bar)")
}

// Test suggesting values in different situations.
func TestSuggestingValuesForEditVariables(t *testing.T) {
	// Suggest value for an edit variable entering a weak equality
	s := NewSolver()
	v1 := Var("foo")

	err := s.AddEditVariable(v1, WithStrength(MEDIUM))
	assert.Equal(t, nil, err, "err")
	err = s.AddConstraint(v1.EqualsConstant(1), WithStrength(WEAK)) // v1 == 1
	assert.Equal(t, nil, err, "err")
	err = s.SuggestValue(v1, 2)
	assert.Equal(t, nil, err, "err")
	s.UpdateVariables()

	assert.EqualFloat64(t, 2, v1.Value, "v1.Value")

	// Suggest a value for an edit variable entering multiple solver rows
	v1, v2 := Var("foo"), Var("bar")
	s = NewSolver()

	s.AddEditVariable(v2, WithStrength(WEAK))                              // v2 == <var> | WEAK
	s.AddConstraint(v1.AddVariable(v2).EqualsConstant(0))                  // v1 + v2 == 0
	s.AddConstraint(v2.LessThanOrEqualsConstant(-1))                       // v2 <= -1
	s.AddConstraint(v2.GreaterThanOrEqualsConstant(0), WithStrength(WEAK)) // v2 >= 0 | WEAK
	s.SuggestValue(v2, 0.0)                                                // v2 = 0

	s.UpdateVariables()
	assert.EqualFloat64(t, -1, v2.Value, "v2.Value")
}

// Test adding/removing constraints.
func TestManagingConstraints(t *testing.T) {
	s := NewSolver()
	v := Var("foo")
	c1 := v.GreaterThanOrEqualsConstant(1) // v >= 1
	c2 := v.LessThanOrEqualsConstant(0)    // v <= 0

	assert.Equal(t, false, s.HasConstraint(c1), "s.HasConstraint(c1)")
	s.AddConstraint(c1)
	assert.Equal(t, true, s.HasConstraint(c1), "s.hasConstraint(c1)")

	err := s.AddConstraint(c1)
	_, ok := err.(DuplicateConstraint)
	assert.Equal(t, true, ok, "_, ok = err.(DuplicateConstraint); ok")
	err = s.RemoveConstraint(c2)
	_, ok = err.(UnknownConstraint)
	assert.Equal(t, true, ok, "_, ok = err.(UnknownConstraint); ok")

	err = s.AddConstraint(c2)
	_, ok = err.(UnsatisfiableConstraint)
	assert.Equal(t, true, ok, "_, ok = err.(UnsatisfiableConstraint); ok")

	s.RemoveConstraint(c1)
	assert.Equal(t, false, s.HasConstraint(c1), "s.HasConstraint(c1)")

	s.AddConstraint(c2)
	assert.Equal(t, true, s.HasConstraint(c2), "s.HasConstraint(c2)")
	s.Reset()
	assert.Equal(t, false, s.HasConstraint(c2), "s.HasConstraint(c2)")
}

// Test solving an under constrained system.
func TestSolvingUnderConstrainedSystem(t *testing.T) {
	s := NewSolver()
	v := Var("foo")

	c := v.Multiply(2).AddConstant(1).GreaterThanOrEqualsConstant(0) // 2 * v + 1 >= 0
	s.AddEditVariable(v, WithStrength(WEAK))
	s.AddConstraint(c)
	err := s.SuggestValue(v, 10)
	assert.Equal(t, nil, err, "err")

	s.UpdateVariables()
	assert.EqualFloat64(t, 21, c.Expression.GetValue(), "c.Expression.GetValue()")
	assert.EqualFloat64(t, 20, c.Expression.Terms[0].GetValue(), "c.Expression.Terms[0].GetValue()")
	assert.EqualFloat64(t, 10, c.Expression.Terms[0].Variable.Value, "c.Expression.Terms[0].Variable.Value")
}

// Test solving a system with unstatisfiable non-required constraint.
func TestSolvingWithStrength(t *testing.T) {
	foo, bar := Var("foo"), Var("bar")

	s := NewSolver()

	s.AddConstraint(foo.AddVariable(bar).EqualsConstant(0))                 // foo + bar == 0
	s.AddConstraint(foo.EqualsConstant(10))                                 // foo == 10
	s.AddConstraint(bar.GreaterThanOrEqualsConstant(0), WithStrength(WEAK)) // bar >= 0 | WEAK

	s.UpdateVariables()
	assert.EqualFloat64(t, 10, foo.Value, "foo.Value")
	assert.EqualFloat64(t, -10, bar.Value, "bar.Value")

	s.Reset()

	s.AddConstraint(foo.AddVariable(bar).EqualsConstant(0))                    // v1 + v2 == 0
	s.AddConstraint(foo.GreaterThanOrEqualsConstant(10), WithStrength(MEDIUM)) // v1 >= 10 | MEDIUM
	s.AddConstraint(bar.EqualsConstant(2), WithStrength(STRONG))               // v2 == 2 | STRONG

	s.UpdateVariables()
	assert.EqualFloat64(t, -2, foo.Value, "foo.Value")
	assert.EqualFloat64(t, 2, bar.Value, "bar.Value")
}

/*
	# Typical output solver.dump in the following function.
	# the order is not stable.

	# Objective
	# ---------
	# -2 + 2 * e2 + 1 * s8 + -2 * s10

	# Tableau
	# -------
	# v1 | 1 + 1 * s10
	# e3 | -1 + 1 * e2 + -1 * s10
	# v4 | -1 + -1 * d5 + -1 * s10
	# s6 | -2 + -1 * s10
	# e9 | -1 + 1 * s8 + -1 * s10

	# Infeasible
	# ----------
	# e3
	# e9

	# Variables
	# ---------
	# bar = v1
	# foo = v4

	# Edit Variables
	# --------------
	# bar

	# Constraints
	# -----------
	# 1 * bar + -0 >= 0  | strength = 1
	# 1 * bar + 1 <= 0  | strength = 1.001e+09
	# 1 * foo + 1 * bar + 0 == 0  | strength = 1.001e+09
	# 1 * bar + 0 == 0  | strength = 1
*/

// Test dumping the solver internal to stdout.
func TestDumpingSolver(t *testing.T) {
	v1, v2 := Var("foo"), Var("bar")

	s := NewSolver()

	s.AddEditVariable(v2, WithStrength(WEAK))
	s.AddConstraint(v1.AddVariable(v2).EqualsConstant(0))                  // v1 + v2 == 0
	s.AddConstraint(v2.LessThanOrEqualsConstant(-1))                       // v2 <= -1
	s.AddConstraint(v2.GreaterThanOrEqualsConstant(0), WithStrength(WEAK)) // v2 >= 0 | WEAK

	s.UpdateVariables()
	err := s.AddConstraint(v2.GreaterThanOrEqualsConstant(1)) // (v2 >= 1)
	assert.NotEqual(t, nil, err, "err")

	state := s.String()
	headers := []string{"Objective", "Tableau", "Infeasible", "Variables", "Edit Variables", "Constraints"}
	for _, h := range headers {
		if !strings.Contains(state, h) {
			t.Fatalf("state does not contain %q", h)
		}
	}
}

// Test that we properly handle infeasible constraints.
func TestHandlingInfeasibleConstraints(t *testing.T) {
	// We use the example of the cassowary paper to generate an infeasible
	// situation after updating an edit variable which causes the solver to use
	// the dual optimization.

	xm, xl, xr := Var("xm"), Var("xl"), Var("xr")

	s := NewSolver()

	s.AddEditVariable(xm, WithStrength(STRONG))
	s.AddEditVariable(xl, WithStrength(WEAK))
	s.AddEditVariable(xr, WithStrength(WEAK))

	s.AddConstraint(xm.Multiply(2).EqualsExpression(xl.AddVariable(xr))) // 2 * xm == xl + xr
	s.AddConstraint(xl.AddConstant(20).LessThanOrEqualsVariable(xr))     // xl + 20 <= xr
	s.AddConstraint(xl.GreaterThanOrEqualsConstant(-10))                 // xl >= -10
	s.AddConstraint(xr.LessThanOrEqualsConstant(100))                    // xr <= 100

	s.SuggestValue(xm, 40)
	s.SuggestValue(xr, 50)
	s.SuggestValue(xl, 30)

	// First update causing a normal update.
	s.SuggestValue(xm, 60)

	// Create an infeasible condition triggering a dual optimization
	s.SuggestValue(xm, 90)

	s.UpdateVariables()
	assert.Equal(t, true, (xl.Value+xr.Value == 2*xm.Value), "(xl.Value+xr.Value == 2*xm.Value)")
	assert.EqualFloat64(t, 80, xl.Value, "xl.Value")
	assert.EqualFloat64(t, 90, xm.Value, "xm.Value")
	assert.EqualFloat64(t, 100, xr.Value, "xr.Value")
}

var assert = struct {
	Equal        func(t *testing.T, exp, got interface{}, msg string, info ...interface{})
	NotEqual     func(t *testing.T, exp, got interface{}, msg string, info ...interface{})
	EqualFloat64 func(t *testing.T, exp, got float64, msg string, info ...interface{})
	EqualString  func(t *testing.T, exp, got string, msg string, info ...interface{})
}{
	Equal: func(t *testing.T, exp, got interface{}, msg string, info ...interface{}) {
		t.Helper()
		if exp != got {
			t.Errorf(msg+" expected %#v got %#v", append(append(info, exp), got)...)
		}
	},
	NotEqual: func(t *testing.T, exp, got interface{}, msg string, info ...interface{}) {
		t.Helper()
		if exp == got {
			t.Errorf(msg+" expected not to be %#v", append(info, exp)...)
		}
	},
	EqualFloat64: func(t *testing.T, exp, got float64, msg string, info ...interface{}) {
		t.Helper()
		if !NearZero(got - exp) {
			t.Errorf(msg+" expected %g got %g", append(append(info, exp), got)...)
		}
	},
	EqualString: func(t *testing.T, exp, got string, msg string, info ...interface{}) {
		t.Helper()
		if exp != got {
			t.Errorf(msg+" expected %#q got %#q", append(append(info, exp), got)...)
		}
	},
}
