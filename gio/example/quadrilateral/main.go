package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/kiwi"
)

var (
	grey900   = color.NRGBA{R: 0x21, G: 0x21, B: 0x21, A: 0xFF}
	grey600   = color.NRGBA{R: 0x75, G: 0x75, B: 0x75, A: 0xFF}
	orange50  = color.NRGBA{R: 0xFF, G: 0xF3, B: 0xE0, A: 0xFF}
	orange200 = color.NRGBA{R: 0xFF, G: 0xCC, B: 0x80, A: 0xFF}
	orange700 = color.NRGBA{R: 0xF5, G: 0x7C, B: 0x00, A: 0xFF}
)

func main() {
	go Quadrilateral()
	app.Main()
}

func Quadrilateral() {
	window := new(app.Window)
	window.Option(
		app.Title("Kiwi - Quadrilateral"),
		app.Size(500, 500))

	// Note that even though we’re drawing a quadrilateral, we have 8 points.
	// We’re tracking the position of the midpoints independent of the corners of our quadrilateral.
	// However, we don’t need to define the position of the midpoints. The position of the midpoints
	// will be set by defining constraints.
	points := []Point{
		// corners
		Pt("C0", 20, 20),
		Pt("C1", 20, 480),
		Pt("C2", 480, 480),
		Pt("C3", 480, 20),
		// midpoints
		Pt("M0", 0, 0),
		Pt("M1", 0, 0),
		Pt("M2", 0, 0),
		Pt("M3", 0, 0),
	}
	corners := points[:4]
	midpoints := points[4:]
	hovered := -1
	selected := -1

	solver := kiwi.NewSolver()

	// Next, we set up some stays. A stay is a constraint that says that a particular variable
	// shouldn’t be modified unless it needs to be - that it should “stay” as is unless there is a
	// reason not to. In this case, we’re going to set a stay for each of the four corners - that
	// is, don’t move the corners unless you have to. These stays are defined as WEAK stays – so
	// they’ll have a very low priority in the constraint system. As a tie breaking mechanism,
	// we’re also going to set each stay to have a different weight - so the top left corner
	// (point 1) will be moved in preference to the bottom left corner (point 2), and so on:
	weight, multiplier := 1.0, 2.0
	for _, c := range corners {
		weak := kiwi.WithStrength(kiwi.Weak(weight))
		solver.AddStay(c.X, weak)
		solver.AddStay(c.Y, weak)
		weight *= multiplier
	}

	// Now we can set up the constraints to define where the midpoints fall. By definition, each
	// midpoint must fall exactly halfway between two points that form a line, and that’s exactly
	// what we describe - an expression that computes the position of the midpoint. This expression
	// is used to construct a Constraint, describing that the value of the midpoint must equal the
	// value of the expression. The Constraint is then added to the solver system:
	edges := []struct{ a, b int }{{0, 1}, {1, 2}, {2, 3}, {3, 0}}
	for _, edge := range edges {
		cx := midpoints[edge.a].X.EqualsExpression(points[edge.a].X.AddVariable(points[edge.b].X).Divide(2)) // m0.x == (p0.x + p1.x) / 2
		cy := midpoints[edge.a].Y.EqualsExpression(points[edge.a].Y.AddVariable(points[edge.b].Y).Divide(2)) // m0.y == (p0.y + p1.y) / 2
		solver.AddConstraint(cx)
		solver.AddConstraint(cy)
	}
	// When we added these constraints, we didn’t provide any arguments - that means that they will
	// be added as REQUIRED constraints.

	// Next, lets add some constraints to ensure that the left side of the quadrilateral stays on
	// the left, and the top stays on top:
	solver.AddConstraint(points[0].X.AddConstant(20).LessThanOrEqualsVariable(points[2].X)) // points[0].x + 20 <= points[2].x
	solver.AddConstraint(points[0].X.AddConstant(20).LessThanOrEqualsVariable(points[3].X)) // points[0].x + 20 <= points[3].x
	solver.AddConstraint(points[1].X.AddConstant(20).LessThanOrEqualsVariable(points[2].X)) // points[1].x + 20 <= points[2].x
	solver.AddConstraint(points[1].X.AddConstant(20).LessThanOrEqualsVariable(points[3].X)) // points[1].x + 20 <= points[3].x
	solver.AddConstraint(points[0].Y.AddConstant(20).LessThanOrEqualsVariable(points[1].Y)) // points[0].y + 20 <= points[1].y
	solver.AddConstraint(points[0].Y.AddConstant(20).LessThanOrEqualsVariable(points[2].Y)) // points[0].y + 20 <= points[2].y
	solver.AddConstraint(points[3].Y.AddConstant(20).LessThanOrEqualsVariable(points[1].Y)) // points[3].y + 20 <= points[1].y
	solver.AddConstraint(points[3].Y.AddConstant(20).LessThanOrEqualsVariable(points[2].Y)) // points[3].y + 20 <= points[2].y
	// Each of these constraints is posed as a Constraint. For example, the first expression
	// describes a point 20 pixels to the right of the x coordinate of the top left point.
	// This Constraint is then added as a constraint on the x coordinate of the bottom right
	// (point 2) and top right (point 3) corners - the x coordinate of these points must be
	// at least 20 pixels greater than the x coordinate of the top left corner (point 0).

	// Setup size variables to prevent the points from moving outside the window to the right
	// and bottom.
	size := Pt("size", 499, 499)
	size.Edit(solver)
	size.Suggest(solver, size.Pt(1.0), 1.0)

	// Lastly, we set the overall constraints – the constraints that limit how large our 2D
	// canvas is. We’ll constraint the canvas to be 500x500 pixels, and require that all
	// points fall on that canvas:
	for i := range points {
		solver.AddConstraint(points[i].X.GreaterThanOrEqualsConstant(0))   // point.x >= 0
		solver.AddConstraint(points[i].Y.GreaterThanOrEqualsConstant(0))   // point.y >= 0
		solver.AddConstraint(points[i].X.LessThanOrEqualsVariable(size.X)) // point.x <= 499
		solver.AddConstraint(points[i].Y.LessThanOrEqualsVariable(size.Y)) // point.y <= 499
	}

	solver.UpdateVariables()

	ops := new(op.Ops)
	backdrop := new(int)
	for {
		switch frame := window.Event().(type) {
		case app.DestroyEvent:
			os.Exit(0)
		case app.FrameEvent:
			scale := frame.Metric.PxPerDp

			frameSize := f32.Pt(float32(frame.Size.X), float32(frame.Size.Y))
			if size.Pt(scale) != frameSize {
				size.Suggest(solver, frameSize, scale)
			}

			// Handle the pointer events of the previous frame before
			// drawing, so the frame reflects the latest positions.
			for {
				ev, ok := frame.Source.Event(pointer.Filter{
					Target: backdrop,
					Kinds:  pointer.Move | pointer.Press | pointer.Drag | pointer.Release,
				})
				if !ok {
					break
				}
				if p, ok := ev.(pointer.Event); ok {
					switch p.Kind {
					case pointer.Press:
						previous := selected
						selected = -1
						for i := range points {
							if points[i].Hit(p.Position, scale) {
								selected = i
							}
						}
						if previous != selected {
							if previous != -1 {
								points[previous].Unedit(solver)
							}
							if selected != -1 {
								points[selected].Edit(solver)
								points[selected].Suggest(solver, p.Position, scale)
							}
						}
					case pointer.Release:
						if selected != -1 {
							points[selected].Unedit(solver)
							selected = -1
						}
					case pointer.Drag:
						if selected != -1 {
							points[selected].Suggest(solver, p.Position, scale)
						}
					case pointer.Move:
						hovered = -1
						for i := range points {
							if points[i].Hit(p.Position, scale) {
								hovered = i
							}
						}
					}
				}
			}

			solver.UpdateVariables()

			ops.Reset()

			// backdrop
			stack := clip.Rect(image.Rectangle{Max: frame.Size}).Push(ops)
			event.Op(ops, backdrop)
			paint.Fill(ops, grey900)
			stack.Pop()

			// edges
			canvas := clip.Path{}
			canvas.Begin(ops)
			canvas.MoveTo(corners[0].Pt(scale))
			for i := range corners {
				canvas.LineTo(corners[(i+1)%4].Pt(scale))
			}
			canvas.MoveTo(midpoints[0].Pt(scale))
			for i := range midpoints {
				canvas.LineTo(midpoints[(i+1)%4].Pt(scale))
			}
			path := clip.Stroke{Width: 2 * scale, Path: canvas.End()}.Op()
			paint.FillShape(ops, grey600, path)

			// points
			for i := range points {
				switch i {
				case selected:
					points[i].Fill(ops, scale, orange50)
				case hovered:
					points[i].Fill(ops, scale, orange200)
				default:
					points[i].Fill(ops, scale, orange700)
				}
			}

			frame.Frame(ops)
		}
	}
}

const PointSize = 5

type Point struct{ X, Y *kiwi.Variable }

func Pt(name string, x, y float64) Point {
	return Point{X: kiwi.Var(name+".X", x), Y: kiwi.Var(name+".Y", y)}
}

func (p Point) Pt(scale float32) f32.Point {
	return f32.Pt(float32(p.X.Value), float32(p.Y.Value)).Mul(scale)
}

func (p Point) Hit(hit f32.Point, scale float32) bool {
	hit = hit.Div(scale)
	dx, dy := float64(hit.X)-p.X.Value, float64(hit.Y)-p.Y.Value
	return math.Sqrt(dx*dx+dy*dy) <= PointSize
}

func (p Point) Fill(ops *op.Ops, scale float32, fill color.NRGBA) {
	x, y, d := int(p.X.Value*float64(scale)), int(p.Y.Value*float64(scale)), int(PointSize*scale)
	paint.FillShape(ops, fill, clip.Rect(image.Rect(x-d, y-d, x+d, y+d)).Op())
}

func (p Point) Edit(solver *kiwi.Solver, options ...kiwi.ConstraintOption) {
	if err := solver.AddEditVariable(p.X, options...); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if err := solver.AddEditVariable(p.Y, options...); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func (p Point) Suggest(solver *kiwi.Solver, pt f32.Point, scale float32) {
	pt = pt.Div(scale)
	if err := solver.SuggestValue(p.X, float64(pt.X)); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if err := solver.SuggestValue(p.Y, float64(pt.Y)); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func (p Point) Unedit(solver *kiwi.Solver) {
	solver.RemoveEditVariable(p.X)
	solver.RemoveEditVariable(p.Y)
}
