// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"fmt"
	"runtime"
)

type Error string

func (e Error) Error() string { return string(e) }

const InternalSolverError = Error("Internal Solver Error")
const UnboundedObjective = Error("Objective is Unbounded")
const BadRequiredStrength = Error("Bad Required Strength")
const FailedToFindLeavingRow = Error("Failed to find Leaving Row")

const SyntaxError = Error("Syntax Error")

func EvaluationError(a ...interface{}) Error {
	_, filename, line, _ := runtime.Caller(1)
	return Error(fmt.Sprintf("Evaluation Error @ %s:%d (%s)", filename, line, fmt.Sprint(a...)))
}

type DuplicateConstraint struct{ *Constraint }

func (e DuplicateConstraint) Error() string {
	return fmt.Sprintf("Duplicate Constraint: %v", e.Constraint)
}

type UnsatisfiableConstraint struct{ *Constraint }

func (e UnsatisfiableConstraint) Error() string {
	return fmt.Sprintf("Unsatisfiable Constraint: %v", e.Constraint)
}

type UnknownConstraint struct{ *Constraint }

func (e UnknownConstraint) Error() string {
	return fmt.Sprintf("Unknown Constraint: %v", e.Constraint)
}

type UnknownEditVariable struct{ *Variable }

func (e UnknownEditVariable) Error() string {
	return fmt.Sprintf("Unknown Edit Variable: %v", e.Variable)
}

type DuplicateEditVariable struct{ *Variable }

func (e DuplicateEditVariable) Error() string {
	return fmt.Sprintf("Duplicate Edit Variable: %v", e.Variable)
}

type UnknownStayVariable struct{ *Variable }

func (e UnknownStayVariable) Error() string {
	return fmt.Sprintf("Unknown Stay Variable: %v", e.Variable)
}

type DuplicateStayVariable struct{ *Variable }

func (e DuplicateStayVariable) Error() string {
	return fmt.Sprintf("Duplicate Stay Variable: %v", e.Variable)
}

type UnknownVariableName struct{ Name string }

func (e UnknownVariableName) Error() string {
	return fmt.Sprintf("Unknown Variable Name: %q", e.Name)
}
