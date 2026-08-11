# AGENTS.md — kiwi

The Cassowary incremental constraint solver in Go: a `Solver` with
`AddConstraint` and `RemoveConstraint`, `AddEditVariable`, `SuggestValue`
and `UpdateVariables` for interactive re-solving, stays, `Variable`, `Term`
and `Expression` arithmetic, the
`OPTIONAL`-`WEAK`-`MEDIUM`-`STRONG`-`REQUIRED` strength ladder, and
`ParseConstraint`, which builds a constraint from a Go expression handed
over as a string. The `gio` module holds a single example: a quadrilateral
whose corners you drag, laid out by the solver.

**Layer.** Outside ADR-001's tier table: a support library, which the rule
binds in one direction only — every tier may import it, and it may import
nothing in the table itself. It depends on nothing but the standard
library, so a change to it can break nothing but itself. Its root module
imports nothing else in the organization. Both directions are measured
rather than typed — `scripts/check-layers.sh --edges` reports the graph and
`scripts/sync-agents.sh` renders these sentences from it — so correcting
them here changes nothing.

**Read the canonical guide before you write code against this module.** It is
the organization's one agent guide — the module inventory with current tags,
the application skeleton, the MVU loop and rx semantics, typography, and the
pitfalls that are not guessable. It lives exactly once, in `vibrantgio/.github`,
and this file links it rather than copying it:

    https://raw.githubusercontent.com/vibrantgio/.github/master/llms.txt

**Modules.** `github.com/vibrantgio/kiwi` at the repository root, and one
nested module: `gio/` (`github.com/vibrantgio/kiwi/gio`). Nested-module
tags carry the directory as a prefix — `gio/v0.0.6`, not `v0.0.6`.

**Build and test.** From the repository root, and again inside each nested
module directory — `./...` does not cross a module boundary:

    go build ./... && go test ./...
