// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"sync"
	"testing"
)

// TestRemoveSoftEqualityCleansObjective verifies that removing a soft
// equality constraint removes the objective function contributions of
// BOTH of its error symbols (errplus AND errminus). The reference kiwi
// implementation removes both; keeping one leaks a dead cell into the
// objective for every removed soft equality (e.g. once per edit
// variable press/release cycle), so the objective grows without bound
// in long interactive sessions.
func TestRemoveSoftEqualityCleansObjective(t *testing.T) {
	// Case 1: plain soft equality, both error symbols parametric.
	s := NewSolver()
	x := Var("x")
	c := x.EqualsConstant(10)
	if err := s.AddConstraint(c, WithStrength(MEDIUM)); err != nil {
		t.Fatal(err)
	}
	tag := s.cns[c]
	if err := s.RemoveConstraint(c); err != nil {
		t.Fatal(err)
	}
	if s.objective.has(tag.marker) {
		t.Errorf("objective still references removed errplus symbol %v", tag.marker)
	}
	if s.objective.has(tag.other) {
		t.Errorf("objective still references removed errminus symbol %v", tag.other)
	}
	if len(s.objective.cells) != 0 {
		t.Errorf("objective not empty after removing the only soft constraint: %v", s.objective)
	}

	// Case 2: the errminus symbol is basic when the constraint is
	// removed (a stronger equality pushes x below the soft target).
	s = NewSolver()
	x = Var("x")
	c = x.EqualsConstant(10)
	if err := s.AddConstraint(c, WithStrength(MEDIUM)); err != nil {
		t.Fatal(err)
	}
	if err := s.AddConstraint(x.EqualsConstant(0), WithStrength(STRONG)); err != nil {
		t.Fatal(err)
	}
	tag = s.cns[c]
	if err := s.RemoveConstraint(c); err != nil {
		t.Fatal(err)
	}
	if s.objective.has(tag.marker) {
		t.Errorf("objective still references removed errplus symbol %v", tag.marker)
	}
	if s.objective.has(tag.other) {
		t.Errorf("objective still references removed errminus symbol %v", tag.other)
	}
}

// TestRemoveEditVariableDoesNotLeakObjectiveCells drives the
// press/drag/release cycle of the quadrilateral demo and checks that
// the objective size stays bounded instead of growing with every
// cycle.
func TestRemoveEditVariableDoesNotLeakObjectiveCells(t *testing.T) {
	s := NewSolver()
	x, y := Var("x"), Var("y")
	if err := s.AddConstraint(x.AddVariable(y).EqualsConstant(100)); err != nil {
		t.Fatal(err)
	}
	baseline := -1
	for i := 0; i < 50; i++ {
		if err := s.AddEditVariable(x, WithStrength(STRONG)); err != nil {
			t.Fatal(err)
		}
		if err := s.SuggestValue(x, float64(i)); err != nil {
			t.Fatal(err)
		}
		s.UpdateVariables()
		if err := s.RemoveEditVariable(x); err != nil {
			t.Fatal(err)
		}
		if baseline < 0 {
			baseline = len(s.objective.cells)
		}
	}
	if got := len(s.objective.cells); got > baseline {
		t.Errorf("objective grew from %d to %d cells over 50 edit cycles", baseline, got)
	}
}

// TestNewConstraintDoesNotMutateCallerExpression verifies that reducing
// duplicate variable terms does not corrupt the expression passed in by
// the caller (the terms slice must be copied, not collapsed in place).
func TestNewConstraintDoesNotMutateCallerExpression(t *testing.T) {
	x, y := Var("x", 1), Var("y", 1)
	e := Expression{Terms: []Term{{x, 1}, {x, 2}, {y, 3}}, Constant: 0}
	before := e.GetValue() // 1*1 + 2*1 + 3*1 = 6
	NewConstraint(e, EQ)
	if after := e.GetValue(); after != before {
		t.Errorf("NewConstraint corrupted caller expression: value %v -> %v (terms %v)", before, after, e.Terms)
	}
}

// TestParseNegativeLiteral verifies the expression parser understands
// unary minus and plus.
func TestParseNegativeLiteral(t *testing.T) {
	x := Var("x")
	vars := []*Variable{x}

	for expr, want := range map[string]float64{
		"x == -10":         -10,
		"x == -(3 + 4)":    -7,
		"-x == 10":         -10,
		"x - -5 == 0":      -5,
		"x == +8":          8,
		"2 * -3 + x == 0":  6,
		"x == -0.5 * 20":   -10,
		"-(x + 10) == -10": 0,
	} {
		s := NewSolver()
		cns, err := ParseConstraint(expr, vars)
		if err != nil {
			t.Errorf("ParseConstraint(%q): %v", expr, err)
			continue
		}
		if err := s.AddConstraint(cns); err != nil {
			t.Errorf("AddConstraint(%q): %v", expr, err)
			continue
		}
		s.UpdateVariables()
		if !NearZero(x.Value - want) {
			t.Errorf("%q: x = %v, want %v", expr, x.Value, want)
		}
	}
}

// TestConcurrentSolvers verifies that independent solvers can be used
// from different goroutines (run with -race). Symbol id generation is
// the only shared state between solvers.
func TestConcurrentSolvers(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := NewSolver()
			x, y := Var("x"), Var("y")
			if err := s.AddConstraint(x.AddVariable(y).EqualsConstant(20)); err != nil {
				t.Error(err)
			}
			if err := s.AddConstraint(x.GreaterThanOrEqualsConstant(5)); err != nil {
				t.Error(err)
			}
			s.UpdateVariables()
		}()
	}
	wg.Wait()
}

// TestStays exercises the stay constraint API: a weak stay anchors a
// variable, editing moves it, and removing the edit variable re-anchors
// the stay at the edited position instead of snapping back.
func TestStays(t *testing.T) {
	s := NewSolver()
	x := Var("x", 100)

	if s.HasStay(x) {
		t.Error("HasStay before AddStay")
	}
	if err := s.AddStay(x, WithStrength(WEAK)); err != nil {
		t.Fatal(err)
	}
	if !s.HasStay(x) {
		t.Error("HasStay after AddStay")
	}
	if err := s.AddStay(x); err == nil {
		t.Error("duplicate AddStay did not fail")
	} else if _, ok := err.(DuplicateStayVariable); !ok {
		t.Errorf("duplicate AddStay: got %T, want DuplicateStayVariable", err)
	}

	s.UpdateVariables()
	if !NearZero(x.Value - 100) {
		t.Errorf("x = %v, want 100 (anchored by stay)", x.Value)
	}

	// Drag x to 200: the edit constraint overpowers the weak stay.
	if err := s.AddEditVariable(x, WithStrength(STRONG)); err != nil {
		t.Fatal(err)
	}
	if err := s.SuggestValue(x, 200); err != nil {
		t.Fatal(err)
	}
	s.UpdateVariables()
	if !NearZero(x.Value - 200) {
		t.Errorf("x = %v, want 200 (edited)", x.Value)
	}

	// Release: RemoveEditVariable commits the stay at the new position.
	if err := s.RemoveEditVariable(x); err != nil {
		t.Fatal(err)
	}
	s.UpdateVariables()
	if !NearZero(x.Value - 200) {
		t.Errorf("x = %v, want 200 (stay re-anchored)", x.Value)
	}

	if err := s.RemoveStay(x); err != nil {
		t.Fatal(err)
	}
	if s.HasStay(x) {
		t.Error("HasStay after RemoveStay")
	}
	if err := s.RemoveStay(x); err == nil {
		t.Error("removing unknown stay did not fail")
	} else if _, ok := err.(UnknownStayVariable); !ok {
		t.Errorf("RemoveStay: got %T, want UnknownStayVariable", err)
	}
}
