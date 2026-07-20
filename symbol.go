// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"fmt"
	"sync/atomic"
)

type symbol struct {
	kind
	id  uint64
	key uint64 // kind in the top bits, id below: the row cell sort key
}

// _sid is shared by all solvers, so it must be updated atomically to
// allow independent solvers to be used from different goroutines.
var _sid uint64

func newSymbol(k kind) *symbol {
	id := atomic.AddUint64(&_sid, 1)
	return &symbol{k, id, uint64(k)<<60 | id}
}

// invalidSymbol is the shared sentinel returned by lookups that find no
// suitable symbol. It is only ever inspected for its kind and must
// never be inserted into a row or used as a map key.
var invalidSymbol = &symbol{INVALID, 0, 0}

func (s symbol) String() string {
	return fmt.Sprintf("%v%d", s.kind, s.id)
}
