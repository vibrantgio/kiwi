// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"fmt"
	"math"
	"strings"
)

// cell is one symbol:coefficient pair of a row. The symbol's sort key
// (kind in the top bits, id below) is stored inline so that searches
// and merges never have to dereference the symbol.
type cell struct {
	key   uint64
	coeff float64
	sym   *symbol
}

/*
row represents a linear expression: constant + sum of coeff * symbol.

The cells are held in a flat slice sorted ascending by symbol key (the
same design as the sorted vector map used by the C++ kiwi
implementation). Compared to a hash map this keeps scans and merges on
contiguous memory, avoids per-operation hashing, and makes every
"first symbol such that ..." selection deterministic. Because the key
orders by kind first (EXTERNAL < SLACK < ERROR < DUMMY), external
symbols always form a prefix of the cells and dummies a suffix.
*/
type row struct {
	constant float64
	cells    []cell
	scratch  []cell // reusable merge buffer, see insertRowWithCoefficient
}

type rowOption func(*row)

func withConstant(constant float64) rowOption {
	return func(r *row) {
		r.constant = constant
	}
}

func newRow(options ...rowOption) *row {
	r := &row{}
	for _, option := range options {
		option(r)
	}
	return r
}

func (r *row) copy() *row {
	return &row{constant: r.constant, cells: append([]cell(nil), r.cells...)}
}

/*
add adds a constant value to the row constant.

# Returns

The new value of the constant
*/
func (r *row) add(value float64) float64 {
	r.constant += value
	return r.constant
}

/*
search returns the position of the cell with the given key, or the
position where such a cell would be inserted when it is not present.
*/
func (r *row) search(key uint64) int {
	lo, hi := 0, len(r.cells)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if r.cells[mid].key < key {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

/*
has reports whether the given symbol is present in the row.
*/
func (r *row) has(sym *symbol) bool {
	i := r.search(sym.key)
	return i < len(r.cells) && r.cells[i].key == sym.key
}

/*
insertSymbolWithCoefficient inserts a symbol into the row with a given coefficient.

If the symbol already exists in the row, the coefficient will be
added to the existing coefficient. If the resulting coefficient
is zero, the symbol will be removed from the row
*/
func (r *row) insertSymbolWithCoefficient(sym *symbol, coeff float64) {
	i := r.search(sym.key)
	if i < len(r.cells) && r.cells[i].key == sym.key {
		coeff += r.cells[i].coeff
		if NearZero(coeff) {
			r.cells = append(r.cells[:i], r.cells[i+1:]...)
		} else {
			r.cells[i].coeff = coeff
		}
	} else if !NearZero(coeff) {
		r.cells = append(r.cells, cell{})
		copy(r.cells[i+1:], r.cells[i:])
		r.cells[i] = cell{sym.key, coeff, sym}
	}
}

/*
insertSymbol inserts a symbol into the row with coefficient 1.0.

If the symbol already exists in the row, the coefficient will be
added to the existing coefficient. If the resulting coefficient
is zero, the symbol will be removed from the row
*/
func (r *row) insertSymbol(sym *symbol) {
	r.insertSymbolWithCoefficient(sym, 1.0)
}

/*
insertRowWithCoefficient inserts a row into this row with a given coefficient.

The constant and the cells of the other row will be multiplied by
the coefficient and added to this row. Any cell with a resulting
coefficient of zero will be removed from the row.

Both cell slices are sorted, so this is a single two pointer merge
into the row's scratch buffer, which is then swapped with the cells.
The other row must not be the row itself.
*/
func (r *row) insertRowWithCoefficient(other *row, coefficient float64) {
	r.constant += other.constant * coefficient
	if len(other.cells) == 0 {
		return
	}
	a, b := r.cells, other.cells
	dst := r.scratch[:0]
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i].key < b[j].key:
			dst = append(dst, a[i])
			i++
		case a[i].key > b[j].key:
			if coeff := b[j].coeff * coefficient; !NearZero(coeff) {
				dst = append(dst, cell{b[j].key, coeff, b[j].sym})
			}
			j++
		default:
			if coeff := a[i].coeff + b[j].coeff*coefficient; !NearZero(coeff) {
				dst = append(dst, cell{a[i].key, coeff, a[i].sym})
			}
			i++
			j++
		}
	}
	dst = append(dst, a[i:]...)
	for ; j < len(b); j++ {
		if coeff := b[j].coeff * coefficient; !NearZero(coeff) {
			dst = append(dst, cell{b[j].key, coeff, b[j].sym})
		}
	}
	r.cells, r.scratch = dst, r.cells
}

/*
removeSymbol removes the given symbol from the row.
*/
func (r *row) removeSymbol(sym *symbol) {
	i := r.search(sym.key)
	if i < len(r.cells) && r.cells[i].key == sym.key {
		r.cells = append(r.cells[:i], r.cells[i+1:]...)
	}
}

/*
reverseSign reverse the sign of the constant and all cells in the row.
*/
func (r *row) reverseSign() {
	r.constant = -r.constant
	cells := r.cells
	for i := range cells {
		cells[i].coeff = -cells[i].coeff
	}
}

/*
chooseSubject chooses the subject for solving for the row

This method will choose the best subject for using as the solve
target for the row. An invalid symbol will be returned if there
is no valid target.
The symbols are chosen according to the following precedence:
 1. The first symbol representing an external variable.
 2. A negative slack or error tag variable.

If a subject cannot be found, an invalid symbol will be returned.
*/
func (r *row) chooseSubject(tag tag) *symbol {
	// External symbols sort first, so any external is at cell 0.
	if len(r.cells) > 0 && r.cells[0].sym.is(EXTERNAL) {
		return r.cells[0].sym
	}

	if tag.marker.is(SLACK) || tag.marker.is(ERROR) {
		if r.coefficientFor(tag.marker) < 0.0 {
			return tag.marker
		}
	}

	if tag.other != nil && (tag.other.is(SLACK) || tag.other.is(ERROR)) {
		if r.coefficientFor(tag.other) < 0.0 {
			return tag.other
		}
	}

	return invalidSymbol
}

/*
allDummies tests whether a row is composed of all dummy variables.
*/
func (r *row) allDummies() bool {
	// Dummy symbols sort last, so all cells are dummies exactly when
	// the first one is.
	return len(r.cells) == 0 || r.cells[0].sym.is(DUMMY)
}

/*
solveFor solves the row for the given symbol.

This method assumes the row is of the form a * x + b * y + c = 0
and (assuming solve for x) will modify the row to represent the
right hand side of x = -b/a * y - c / a. The target symbol will
be removed from the row, and the constant and other cells will
be multiplied by the negative inverse of the target coefficient.
The given symbol *must* exist in the row.
*/
func (r *row) solveFor(sym *symbol) {
	i := r.search(sym.key)
	coeff := -1.0 / r.cells[i].coeff
	r.cells = append(r.cells[:i], r.cells[i+1:]...)
	r.constant *= coeff
	cells := r.cells
	for k := range cells {
		cells[k].coeff *= coeff
	}
}

/*
solveForPair solves the row for the given symbols.

This method assumes the row is of the form x = b * y + c and will
solve the row such that y = x / b - c / b. The rhs symbol will be
removed from the row, the lhs added, and the result divided by the
negative inverse of the rhs coefficient.
The lhs symbol *must not* exist in the row, and the rhs symbol
must* exist in the row.
*/
func (r *row) solveForPair(lhs, rhs *symbol) {
	r.insertSymbolWithCoefficient(lhs, -1.0)
	r.solveFor(rhs)
}

/*
coefficientFor gets the coefficient for the given symbol.

If the symbol does not exist in the row, zero will be returned.
*/
func (r *row) coefficientFor(sym *symbol) float64 {
	if i := r.search(sym.key); i < len(r.cells) && r.cells[i].key == sym.key {
		return r.cells[i].coeff
	}
	return 0.0
}

/*
substitute substitutes a symbol with the data from another row.

Given a row of the form a * x + b and a substitution of the
form x = 3 * y + c the row will be updated to reflect the
expression 3 * a * y + a * c + b.

If the symbol does not exist in the row, this is a no-op.
*/
func (r *row) substitute(sym *symbol, other *row) {
	i := r.search(sym.key)
	if i < len(r.cells) && r.cells[i].key == sym.key {
		coeff := r.cells[i].coeff
		r.cells = append(r.cells[:i], r.cells[i+1:]...)
		r.insertRowWithCoefficient(other, coeff)
	}
}

/*
anyPivotableSymbol gets the first Slack or Error symbol in the row.

If no such symbol is present, and Invalid symbol will be returned.
*/
func (r *row) anyPivotableSymbol() *symbol {
	for i := range r.cells {
		if sym := r.cells[i].sym; sym.is(SLACK) || sym.is(ERROR) {
			return sym
		}
	}
	return invalidSymbol
}

// dummyKeys is the lower bound of the keys of all DUMMY symbols; cells
// with a key at or above it are dummies (they sort last in a row).
const dummyKeys = uint64(DUMMY) << 60

/*
getEnteringSymbol computes the entering variable for a pivot operation.

This method will return first symbol in the objective function which
is non-dummy and has a coefficient less than zero. If no symbol meets
the criteria, it means the objective function is at a minimum, and an
invalid symbol is returned.
*/
func (r *row) getEnteringSymbol() *symbol {
	for i := range r.cells {
		c := &r.cells[i]
		if c.key >= dummyKeys {
			break
		}
		if c.coeff < 0.0 {
			return c.sym
		}
	}
	return invalidSymbol
}

/*
getDualEnteringSymbol computes the entering symbol for the dual optimize operation.

This method will return the symbol in the row which has a positive
coefficient and yields the minimum ratio for its respective symbol
in the objective function. The provided row *must* be infeasible.
If no symbol is found which meats the criteria, an invalid symbol
is returned.
*/
func (r *row) getDualEnteringSymbol(other *row) *symbol {
	objective := r
	ratio := math.MaxFloat64
	entering := invalidSymbol
	for i := range other.cells {
		c := &other.cells[i]
		if c.key >= dummyKeys {
			break
		}
		if c.coeff > 0.0 {
			if ra := objective.coefficientFor(c.sym) / c.coeff; ra < ratio {
				ratio = ra
				entering = c.sym
			}
		}
	}
	return entering
}

func (r *row) String() string {
	c := []string{fmt.Sprint(r.constant)}
	for _, e := range r.cells {
		c = append(c, fmt.Sprint(e.coeff, " * ", e.sym))
	}
	return strings.Join(c, " + ")
}
