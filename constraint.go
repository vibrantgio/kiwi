// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"fmt"
	"math"
)

type Constraint struct {
	Expression Expression
	Operator   Operator
	Strength   Strength
}

type ConstraintOption func(*Constraint)

// WithStrength is a constraint option to set the strength of the constraint
func WithStrength(strength Strength) ConstraintOption {
	return func(c *Constraint) {
		optional, strength, required := float64(OPTIONAL), float64(strength), float64(REQUIRED)
		c.Strength = Strength(math.Max(optional, math.Min(strength, required)))
	}
}

func NewConstraint(expr Expression, op Operator, options ...ConstraintOption) *Constraint {
	// Reduce on a copy of the terms; reducing in place would corrupt
	// the expression still held by the caller.
	expr.Terms = append([]Term(nil), expr.Terms...)
	// reduce: c + pv + qv + rw -> c + (p+q)v + rw
	vars := make(map[*Variable]int)
	collapsed := 0
	for i, t := range expr.Terms {
		if j, present := vars[t.Variable]; present {
			expr.Terms[j].Coefficient += t.Coefficient
			collapsed++
		} else {
			vars[t.Variable] = i - collapsed
			if collapsed > 0 {
				expr.Terms[i-collapsed] = t
			}
		}
	}
	expr.Terms = expr.Terms[:len(expr.Terms)-collapsed]
	cns := &Constraint{expr, op, REQUIRED}
	cns.ApplyOptions(options...)
	return cns
}

func (c *Constraint) ApplyOptions(options ...ConstraintOption) {
	for _, opt := range options {
		opt(c)
	}
}

func (c *Constraint) String() string {
	return fmt.Sprint(c.Expression, c.Operator, " 0 | Strength = ", c.Strength)
}
