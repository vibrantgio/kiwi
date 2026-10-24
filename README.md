# kiwi

The Cassowary incremental constraint solver in Go, for
[Vibrant Gio](https://github.com/vibrantgio), a design system for native desktop
applications on macOS, Windows and Linux, written in pure Go on
[Gio](https://gioui.org). One package, no dependencies outside the standard
library.

Layout code normally computes positions. A constraint solver inverts that: you
*declare* the relationships — this midpoint is halfway between those corners,
this pane is at least ten pixels wide, this edge stays put unless something
forces it — and the solver finds the assignment that satisfies all of them. When
one value changes, Cassowary does not re-solve from scratch; it repairs the
existing solution with a dual-simplex pass over the tableau it already has.
That is what makes it usable in a pointer-drag loop, where the alternative is
recomputing an entire layout sixty times a second.

Cassowary is the algorithm behind Apple's Auto Layout and behind the constraint
layouts in Qt and Android. This module is a port of the C++
[nucleic/kiwi](https://github.com/nucleic/kiwi) implementation, and the
BSD-3-Clause LICENSE holds the Nucleic Development Team's 2013 copyright
alongside René Post's. It adds two things the C++ library
does not have: *stay* constraints, which are classic Cassowary but were dropped
from nucleic/kiwi's API, and a string DSL that parses a constraint out of a Go
expression using `go/parser`.

## Where it sits

Outside the tier table — a support library, not a layer of the stack. The
[organization page](https://github.com/vibrantgio) has the full tier table.

```sh
go get github.com/vibrantgio/kiwi
```

Two modules. The root module, `github.com/vibrantgio/kiwi`, has an
empty `go.sum` and no `require` block at all — the solver imports nothing but
`fmt`, `math`, `strings`, `strconv`, `sync/atomic`, `runtime` and `go/{ast,parser,token}`.
The nested `gio/` module (`github.com/vibrantgio/kiwi/gio`) holds one example
program and requires Gio and the kiwi version that `gio/go.mod` names.

## Packages

One library package at the module root, and one demo module.

| Symbol | |
| --- | --- |
| `NewSolver()` | The solver. Not goroutine-safe — one solver per goroutine. Independent solvers on different goroutines are safe and are tested under `-race`. |
| `Var(name, value...)` | A `*Variable`: a name and a `float64` `Value` the solver writes. `Vars(names...)` makes several; `NewVariable(name)` is the same thing without the optional initial value. |
| `Variable`, `Term`, `Expression` | The arithmetic layer. Each implements `Constrainer`, so each can build a `*Constraint` — `x.AddConstant(20).LessThanOrEqualsVariable(y)` is `x + 20 <= y`. See Status for what that costs. |
| `Solver.AddConstraint` / `RemoveConstraint` / `HasConstraint` | Constraints in and out of the system, incrementally. |
| `Solver.AddEditVariable` / `SuggestValue` / `RemoveEditVariable` | The interactive path: mark a variable editable, feed it new values, release it. |
| `Solver.AddStay` / `RemoveStay` / `UpdateStays` | "Leave this where it is unless you must move it." `RemoveEditVariable` calls `UpdateStays` for you, re-anchoring each stay to the value its variable now holds. |
| `Solver.UpdateVariables()` | Write the tableau back into every `Variable.Value`. Nothing is visible until you call it. |
| `Solver.Reset()` | Back to empty, reusing the allocations. |
| `OPTIONAL`, `WEAK`, `MEDIUM`, `STRONG`, `REQUIRED` | The strength scale: 0, 1, 1 000, 1 000 000 and 1 001 001 000. `Weak(w)`, `Medium(w)` and `Strong(w)` give a weighted point inside a band, clamping `w` to `[1, 999.9999999999999]`. |
| `ParseConstraint(expr, vars, opts...)` | A constraint from a string — `ParseConstraint("x == -0.5 * 20", []*Variable{x})`. `ParseExpr` is the same parse without the constraint. |
| `gio/example/quadrilateral` | The one demo: a quadrilateral whose four corners you drag while four midpoints follow. |

## Usage

The whole API in a few lines, from `TestSimple0` in `unit_test.go`:

```go
solver := kiwi.NewSolver()
x := kiwi.Var("x")
solver.AddConstraint(x.AddConstant(2).EqualsConstant(20)) // x + 2 == 20
solver.UpdateVariables()
// x.Value == 18
```

The interesting part is what happens when the system is under-determined and you
push on it. `TestEditVars` in `unit_test.go` builds three variables and four
required relations, pins `x1` to 40 only weakly, then makes the midpoint editable and
suggests a value for it:

```go
x1, x2, xm := kiwi.Var("x1"), kiwi.Var("x2"), kiwi.Var("xm")

solver.AddConstraint(x1.GreaterThanOrEqualsConstant(0))                    // x1 >= 0
solver.AddConstraint(x2.LessThanOrEqualsConstant(100))                     // x2 <= 100
solver.AddConstraint(x2.GreaterThanOrEqualsExpression(x1.AddConstant(20))) // x2 >= x1 + 20
solver.AddConstraint(xm.EqualsExpression(x1.AddVariable(x2).Divide(2)))    // xm == (x1 + x2) / 2
solver.AddConstraint(x1.EqualsConstant(40), kiwi.WithStrength(kiwi.WEAK))  // x1 == 40 | WEAK

solver.AddEditVariable(xm, kiwi.WithStrength(kiwi.STRONG))
solver.SuggestValue(xm, 60)
solver.UpdateVariables()
// x1 == 40, x2 == 80, xm == 60
```

`x1` held at its weak preference and `x2` moved, because moving `x2` was free
and moving `x1` was not. That choice is the entire value of the strength scale.

In a pointer loop the three edit calls line up with the three pointer kinds.
From the pointer switch in `Quadrilateral` in
`gio/example/quadrilateral/main.go`, with the `Point` helper's two-call
wrappers inlined:

```go
case pointer.Press:
	solver.AddEditVariable(p.X, kiwi.WithStrength(kiwi.STRONG))
	solver.SuggestValue(p.X, float64(pos.X))
case pointer.Drag:
	solver.SuggestValue(p.X, float64(pos.X))
case pointer.Release:
	solver.RemoveEditVariable(p.X) // also re-anchors the stays
}
solver.UpdateVariables()
```

Only the press and the release touch the tableau's shape; the drag is a
suggestion against a system that is already factored. The repository's own
benchmarks show it: building the quadrilateral's constraints from scratch
(`BenchmarkQuadBuild`) allocates, while one drag frame — two
`SuggestValue` plus `UpdateVariables` (`BenchmarkQuadDrag`) — makes **zero
allocations**, and so does resizing a 50-pane row layout
(`BenchmarkRowLayoutResize`). `go test -run XXX -bench . -benchmem` measures
them on your hardware.

## For coding assistants

Read the org guide before you write code against this module: the plan
root's [`AGENTS.md`](https://github.com/vibrantgio/organisation/blob/master/AGENTS.md).
The build and test commands are reached from that file.

## Status

Honest about what does not work yet.

- **There is no layout integration, only a demo.** The `gio/` module is a single
  `package main`. It contains no exported API, no `layout.Widget`, no
  reusable "constrained container" — nothing a caller could import. Wiring the
  solver to Gio's layout protocol is work that has not been done.
- **The `gio/` module has no `replace` directive**, so outside a Go workspace
  it compiles against the published kiwi version that `gio/go.mod` names,
  fetched from the module proxy, rather than the tree it sits in. Editing the
  solver and rebuilding the example that way does not exercise the edit.
- **`UpdateStays` throws its errors away.** It calls
  `RemoveConstraint` and `AddConstraint` for each stay and discards both return
  values, and the method itself returns nothing. Since `RemoveEditVariable`
  calls it, a stay that fails to re-anchor when the pointer is released is
  invisible to the caller. `AddConstraint` does the same on its unsatisfiable
  path, where `optimize` and `dualOptimize` are called only
  for their tableau-restoring side effect.
- **Errors come in two incompatible shapes and neither wraps.** Five sentinels
  are a `type Error string` and compare with `==`; eight more —
  `UnsatisfiableConstraint`, `DuplicateEditVariable`, `UnknownVariableName` and
  the rest — are structs that require a type assertion. None implements
  `Unwrap()`, and nothing is ever wrapped with `%w`, so `errors.Is` and
  `errors.As` do not traverse. `EvaluationError` is worse: it builds its message
  with `runtime.Caller(1)` from inside `ast.go`, so what a caller sees is
  kiwi's *own* file path — a module-cache path in any real build — and not
  anything in their program.
- **`Variable.Value` is a public mutable field, and `UpdateVariables` zeroes
  it.** Writing it directly desynchronizes it from the tableau, and
  `UpdateVariables` sets every solver-known variable that is not currently basic
  to `0.0`. There is no accessor that would have prevented
  either.
- **The fluent API is a combinatorial matrix, because Go has no operators.**
  `Constrainer` is twelve methods — three comparisons × four right-hand-side
  kinds — implemented by `Variable`, `Term` and `Expression`, each of which also
  has its own arithmetic builders. That is dozens of methods to
  express what `+ - * / == <= >=` express in C++, and the chains read backwards
  from the mathematics they encode. `ParseConstraint` is the escape hatch, and
  it trades the problem for a stringly-typed one: identifiers are matched
  against a caller-supplied `[]*Variable` by `Name`, so a typo is a runtime
  `UnknownVariableName` and a duplicate name silently shadows.
- **`float64` only.** No generics, no `float32` path. Every Gio caller converts
  on the way in and on the way out; the example does exactly that, twice per
  point per frame.
- **`Solver.String()` is not reproducible.** The tableau block is a sorted
  slice and is stable, but the Variables, Edit Variables, Stay Constraints and
  Constraints blocks are `range` over maps in `Solver.String`, so their line
  order changes between runs. It is usable for eyeballing a solver, not for a
  golden test or a diff. There is no `Dump(io.Writer)`.
- **There is no batch API.** Adding N constraints costs N full `optimize` +
  `dualOptimize` passes; `BenchmarkRowLayoutBuild` measures that for a 50-pane
  layout. An `AddConstraints([]*Constraint)` that
  optimized once at the end does not exist.
- **Coverage is partial in the root module, and `gio/` has no tests.** The
  untested code is concentrated in `ast.go`'s evaluation dispatch and in the
  `String()`/`TechString()` printers. The tests include a property test and a
  regression suite, and `-race` is clean.

On the other side: the library never panics,
never calls `os.Exit` or `log.Fatal`, and contains no `TODO`, `FIXME` or
`XXX` markers anywhere. Every failure is a returned error.
