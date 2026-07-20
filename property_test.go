// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// violationCost returns the weighted violation of a constraint for the
// current variable values: zero when satisfied, otherwise the distance
// from satisfaction times the constraint strength.
func violationCost(c *Constraint) float64 {
	v := c.Expression.GetValue()
	violation := 0.0
	switch c.Operator {
	case LE:
		if v > 0 {
			violation = v
		}
	case GE:
		if v < 0 {
			violation = -v
		}
	case EQ:
		violation = math.Abs(v)
	}
	return float64(c.Strength) * violation
}

// TestIncrementalMatchesBatch adds and removes random constraints
// incrementally and checks that the resulting solution is exactly as
// good (same total weighted violation) as solving the surviving
// constraint set from scratch, and that required constraints are
// satisfied. This guards the RemoveConstraint bookkeeping: a solver
// state corrupted by a removal shows up as a worse-than-optimal
// solution later on.
func TestIncrementalMatchesBatch(t *testing.T) {
	rng := rand.New(rand.NewSource(20260720))
	strengths := []Strength{WEAK, MEDIUM, STRONG, REQUIRED}

	for trial := 0; trial < 300; trial++ {
		vars := []*Variable{Var("a"), Var("b"), Var("c"), Var("d")}

		randomConstraint := func() *Constraint {
			n := 1 + rng.Intn(3)
			perm := rng.Perm(len(vars))[:n]
			terms := make([]Term, 0, n)
			for _, vi := range perm {
				coeff := 0
				for coeff == 0 {
					coeff = rng.Intn(7) - 3
				}
				terms = append(terms, Term{vars[vi], float64(coeff)})
			}
			expr := Expression{terms, float64(rng.Intn(41) - 20)}
			op := Operator(rng.Intn(3))
			return NewConstraint(expr, op, WithStrength(strengths[rng.Intn(len(strengths))]))
		}

		s := NewSolver()
		var live []*Constraint
		for i := 0; i < 12; i++ {
			c := randomConstraint()
			if err := s.AddConstraint(c); err == nil {
				live = append(live, c)
			}
		}
		for i := 0; i < 4 && len(live) > 0; i++ {
			j := rng.Intn(len(live))
			if err := s.RemoveConstraint(live[j]); err != nil {
				t.Fatalf("trial %d: RemoveConstraint: %v", trial, err)
			}
			live = append(live[:j], live[j+1:]...)
		}

		cost := func(who string) float64 {
			total := 0.0
			for _, c := range live {
				if c.Strength == REQUIRED {
					if v := violationCost(c); v > 1e-6*float64(REQUIRED) {
						t.Fatalf("trial %d: %s violates required constraint %v (violation cost %g)", trial, who, c, v)
					}
				} else {
					total += violationCost(c)
				}
			}
			return total
		}

		s.UpdateVariables()
		incremental := cost("incremental solver")

		batch := NewSolver()
		for _, c := range live {
			if err := batch.AddConstraint(c); err != nil {
				t.Fatalf("trial %d: batch AddConstraint: %v", trial, err)
			}
		}
		batch.UpdateVariables()
		fresh := cost("batch solver")

		// Tolerance: costs are weighted by strengths up to 1e6, so an
		// absolute slack of 1e-3 in weighted cost corresponds to a
		// physically irrelevant violation (1e-3 units on a WEAK
		// constraint, 1e-9 units on a STRONG one). Rounding drift of
		// this order is inherent to the pivoting arithmetic.
		if diff := math.Abs(incremental - fresh); diff > 1e-3+1e-6*math.Max(incremental, fresh) {
			var desc string
			for _, c := range live {
				desc += fmt.Sprintln(" ", c)
			}
			t.Fatalf("trial %d: incremental cost %g != batch cost %g\nconstraints:\n%s", trial, incremental, fresh, desc)
		}
	}
}
