// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"fmt"
	"testing"
)

// quad replicates the constraint system of the quadrilateral demo in
// gio/example/quadrilateral: four draggable corners with weak stays,
// midpoints tied to the corners with required equalities, ordering
// constraints, and window bounds.
type quad struct {
	solver *Solver
	px, py [8]*Variable // 4 corners followed by 4 midpoints
	sx, sy *Variable    // window size
}

func newQuad() *quad {
	q := &quad{solver: NewSolver()}
	for i := 0; i < 8; i++ {
		q.px[i] = Var(fmt.Sprintf("p%d.x", i))
		q.py[i] = Var(fmt.Sprintf("p%d.y", i))
	}
	initial := [4][2]float64{{20, 20}, {20, 480}, {480, 480}, {480, 20}}
	weight := 1.0
	for i := 0; i < 4; i++ {
		q.px[i].Value, q.py[i].Value = initial[i][0], initial[i][1]
		weak := WithStrength(Weak(weight))
		q.solver.AddStay(q.px[i], weak)
		q.solver.AddStay(q.py[i], weak)
		weight *= 2
	}
	edges := [4][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}}
	for _, e := range edges {
		q.solver.AddConstraint(q.px[4+e[0]].EqualsExpression(q.px[e[0]].AddVariable(q.px[e[1]]).Divide(2)))
		q.solver.AddConstraint(q.py[4+e[0]].EqualsExpression(q.py[e[0]].AddVariable(q.py[e[1]]).Divide(2)))
	}
	q.solver.AddConstraint(q.px[0].AddConstant(20).LessThanOrEqualsVariable(q.px[2]))
	q.solver.AddConstraint(q.px[0].AddConstant(20).LessThanOrEqualsVariable(q.px[3]))
	q.solver.AddConstraint(q.px[1].AddConstant(20).LessThanOrEqualsVariable(q.px[2]))
	q.solver.AddConstraint(q.px[1].AddConstant(20).LessThanOrEqualsVariable(q.px[3]))
	q.solver.AddConstraint(q.py[0].AddConstant(20).LessThanOrEqualsVariable(q.py[1]))
	q.solver.AddConstraint(q.py[0].AddConstant(20).LessThanOrEqualsVariable(q.py[2]))
	q.solver.AddConstraint(q.py[3].AddConstant(20).LessThanOrEqualsVariable(q.py[1]))
	q.solver.AddConstraint(q.py[3].AddConstant(20).LessThanOrEqualsVariable(q.py[2]))
	q.sx, q.sy = Var("size.x", 499), Var("size.y", 499)
	q.solver.AddEditVariable(q.sx, WithStrength(STRONG))
	q.solver.AddEditVariable(q.sy, WithStrength(STRONG))
	q.solver.SuggestValue(q.sx, 499)
	q.solver.SuggestValue(q.sy, 499)
	for i := 0; i < 8; i++ {
		q.solver.AddConstraint(q.px[i].GreaterThanOrEqualsConstant(0))
		q.solver.AddConstraint(q.py[i].GreaterThanOrEqualsConstant(0))
		q.solver.AddConstraint(q.px[i].LessThanOrEqualsVariable(q.sx))
		q.solver.AddConstraint(q.py[i].LessThanOrEqualsVariable(q.sy))
	}
	q.solver.UpdateVariables()
	return q
}

// BenchmarkQuadBuild measures constructing the full quadrilateral system
// from scratch: the AddConstraint / createRow / optimize path.
func BenchmarkQuadBuild(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newQuad()
	}
}

// BenchmarkQuadDrag measures the interactive hot path: suggesting new
// values for an edited point every frame (SuggestValue / dualOptimize /
// UpdateVariables).
func BenchmarkQuadDrag(b *testing.B) {
	q := newQuad()
	q.solver.AddEditVariable(q.px[0], WithStrength(STRONG))
	q.solver.AddEditVariable(q.py[0], WithStrength(STRONG))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.solver.SuggestValue(q.px[0], float64(30+i%400))
		q.solver.SuggestValue(q.py[0], float64(30+(i*7)%400))
		q.solver.UpdateVariables()
	}
}

// BenchmarkQuadEditCycle measures the press/drag/release cycle from the
// demo: adding edit variables, suggesting a value, and removing the edit
// variables again (which also re-anchors the stays via UpdateStays).
func BenchmarkQuadEditCycle(b *testing.B) {
	q := newQuad()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := i % 4
		q.solver.AddEditVariable(q.px[p], WithStrength(STRONG))
		q.solver.AddEditVariable(q.py[p], WithStrength(STRONG))
		q.solver.SuggestValue(q.px[p], float64(50+i%300))
		q.solver.SuggestValue(q.py[p], float64(50+(i*3)%300))
		q.solver.UpdateVariables()
		q.solver.RemoveEditVariable(q.px[p])
		q.solver.RemoveEditVariable(q.py[p])
	}
}

// buildRowLayout constructs a horizontal box layout of n panes:
//
//	left[0] == 0
//	left[i] == right[i-1] + 5
//	right[i] == left[i] + width[i]
//	width[i] >= 10               (required)
//	width[i] == 100 | MEDIUM     (preferred width)
//	right[n-1] <= total
func buildRowLayout(n int) (*Solver, *Variable) {
	s := NewSolver()
	total := Var("total", float64(n)*105)
	var prev *Variable
	for i := 0; i < n; i++ {
		left := Var(fmt.Sprintf("left%d", i))
		width := Var(fmt.Sprintf("width%d", i))
		right := Var(fmt.Sprintf("right%d", i))
		if prev == nil {
			s.AddConstraint(left.EqualsConstant(0))
		} else {
			s.AddConstraint(left.EqualsExpression(prev.AddConstant(5)))
		}
		s.AddConstraint(right.EqualsExpression(left.AddVariable(width)))
		s.AddConstraint(width.GreaterThanOrEqualsConstant(10))
		s.AddConstraint(width.EqualsConstant(100), WithStrength(MEDIUM))
		prev = right
	}
	s.AddConstraint(prev.LessThanOrEqualsVariable(total))
	return s, total
}

// BenchmarkRowLayoutBuild measures building a 50 pane box layout.
func BenchmarkRowLayoutBuild(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buildRowLayout(50)
	}
}

// BenchmarkRowLayoutResize measures window resizing: repeatedly
// suggesting a new total width, forcing the pane widths to compress and
// expand again.
func BenchmarkRowLayoutResize(b *testing.B) {
	s, total := buildRowLayout(50)
	s.AddEditVariable(total, WithStrength(STRONG))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SuggestValue(total, float64(2000+i%3300))
		s.UpdateVariables()
	}
}

// BenchmarkAddRemoveConstraint measures repeatedly adding and removing a
// soft constraint on an existing system: the RemoveConstraint /
// getMarkerLeavingRow / substitute path.
func BenchmarkAddRemoveConstraint(b *testing.B) {
	s, total := buildRowLayout(20)
	c := total.EqualsConstant(1500)
	WithStrength(STRONG)(c)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddConstraint(c)
		s.RemoveConstraint(c)
	}
}
