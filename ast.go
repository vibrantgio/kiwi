// SPDX-License-Identifier: BSD-3-Clause

package kiwi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

type AST struct{ ast.Expr }

func ParseExpr(a ...interface{}) (*AST, error) {
	expr, err := parser.ParseExpr(fmt.Sprint(a...))
	if err != nil {
		return nil, err
	}
	return &AST{expr}, nil
}

func ParseConstraint(x string, vars []*Variable, options ...ConstraintOption) (*Constraint, error) {
	expr, err := ParseExpr(x)
	if err != nil {
		return nil, err
	}
	cnst, err := expr.NewConstraint(vars, options...)
	if err != nil {
		return nil, err
	}
	return cnst, nil
}

func (a AST) String() string {
	var f strings.Builder
	var walk func(expr ast.Expr)
	walk = func(expr ast.Expr) {
		switch e := expr.(type) {
		case *ast.BinaryExpr:
			walk(e.X)
			fmt.Fprintf(&f, " %v ", e.Op)
			walk(e.Y)
		case *ast.Ident:
			if e.Obj == nil {
				fmt.Fprintf(&f, "%v", e.Name)
			} else {
				fmt.Fprintf(&f, "%v %v", e.Name, e.Obj)
			}
		case *ast.BasicLit:
			fmt.Fprintf(&f, "%v(%v)", e.Kind, e.Value)
		case *ast.ParenExpr:
			fmt.Fprint(&f, "(")
			walk(e.X)
			fmt.Fprint(&f, ")")
		default:
			fmt.Fprintf(&f, "%#v", e)
		}
	}
	walk(a.Expr)
	return f.String()
}

func (a AST) TechString() string {
	const PAD = 8
	var f strings.Builder
	var walk func(expr ast.Expr, level int)
	walk = func(expr ast.Expr, level int) {
		padding := fmt.Sprintf("%*v", level, "")
		paddingnext := fmt.Sprintf("%*v", level+PAD, "")
		switch e := expr.(type) {
		case *ast.BinaryExpr:
			fmt.Fprintln(&f, padding+"(*ast.BinaryExpr:")
			walk(e.X, level+PAD)
			fmt.Fprintf(&f, paddingnext+"%v\n", e.Op)
			walk(e.Y, level+PAD)
			fmt.Fprintln(&f, padding+")")
		case *ast.Ident:
			fmt.Fprint(&f, padding+"(*ast.Ident: ")
			if e.Obj == nil {
				fmt.Fprintf(&f, "%v", e.Name)
			} else {
				fmt.Fprintf(&f, "%v %v", e.Name, e.Obj)
			}
			fmt.Fprintln(&f, ")")
		case *ast.BasicLit:
			fmt.Fprint(&f, padding+"(*ast.BasicLit: ")
			fmt.Fprintf(&f, "%v %v", e.Kind, e.Value)
			fmt.Fprintln(&f, ")")
		case *ast.ParenExpr:
			fmt.Fprintln(&f, padding+"(*ast.ParenExpr:")
			walk(e.X, level+PAD)
			fmt.Fprintln(&f, padding+")")
		default:
			fmt.Fprintf(&f, "%#v\n", e)
		}
	}
	walk(a.Expr, 0)
	return f.String()
}

func (a AST) NewConstraint(vars []*Variable, options ...ConstraintOption) (*Constraint, error) {
	varmap := make(map[string]*Variable)
	for _, v := range vars {
		varmap[v.Name] = v
	}
	var evaluate func(expr ast.Expr) (evaluation, error)
	evaluate = func(expr ast.Expr) (evaluation, error) {
		switch e := expr.(type) {
		case *ast.BinaryExpr:
			lhs, err := evaluate(e.X)
			if err != nil {
				return nil, err
			}
			rhs, err := evaluate(e.Y)
			if err != nil {
				return nil, err
			}
			switch e.Op {
			case token.ADD:
				return lhs.add(rhs)
			case token.SUB:
				return lhs.sub(rhs)
			case token.MUL:
				return lhs.mul(rhs)
			case token.QUO:
				return lhs.div(rhs)
			case token.EQL:
				return lhs.eql(rhs)
			case token.LEQ:
				return lhs.leq(rhs)
			case token.GEQ:
				return lhs.geq(rhs)
			default:
				return nil, EvaluationError("operator ", e.Op, " not supported")
			}
		case *ast.Ident:
			v, present := varmap[e.Name]
			if !present {
				return nil, UnknownVariableName{e.Name}
			}
			return vareval{v}, nil
		case *ast.BasicLit:
			switch e.Kind {
			case token.FLOAT, token.INT:
				fv, err := strconv.ParseFloat(e.Value, 64)
				if err != nil {
					return nil, err
				}
				return liteval{fv}, nil
			}
			return nil, SyntaxError
		case *ast.ParenExpr:
			ex, err := evaluate(e.X)
			if err != nil {
				return nil, err
			}
			return ex, nil
		case *ast.UnaryExpr:
			x, err := evaluate(e.X)
			if err != nil {
				return nil, err
			}
			switch e.Op {
			case token.ADD:
				return x, nil
			case token.SUB:
				switch v := x.(type) {
				case liteval:
					return liteval{-v.Value}, nil
				case vareval:
					return termeval{v.Variable.Negate()}, nil
				case termeval:
					return termeval{v.Term.Negate()}, nil
				case expreval:
					return expreval{v.Expression.Negate()}, nil
				}
				return nil, EvaluationError("cannot negate operand")
			}
			return nil, EvaluationError("unary operator ", e.Op, " not supported")
		}
		return nil, EvaluationError("unsupported expression")
	}
	evl, err := evaluate(a.Expr)
	if err != nil {
		return nil, err
	}
	var cns *Constraint
	switch e := evl.(type) {
	case constreval:
		cns = e.Constraint
	case expreval:
		cns = e.Expression.EqualsConstant(0)
	case termeval:
		cns = e.Term.EqualsConstant(0)
	case vareval:
		cns = e.Variable.EqualsConstant(0)
	}
	if cns == nil {
		return nil, EvaluationError("NewConstraint")
	}
	for _, opt := range options {
		opt(cns)
	}
	return cns, nil
}

type evaluation interface {
	add(evaluation) (evaluation, error)
	sub(evaluation) (evaluation, error)
	mul(evaluation) (evaluation, error)
	div(evaluation) (evaluation, error)
	eql(evaluation) (evaluation, error)
	leq(evaluation) (evaluation, error)
	geq(evaluation) (evaluation, error)
}

type liteval struct {
	Value float64
}

func (lhs liteval) add(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		lhs.Value += rhs.Value
		return lhs, nil
	case vareval:
		return expreval{rhs.Variable.AddConstant(lhs.Value)}, nil
	case termeval:
		return expreval{rhs.Term.AddConstant(lhs.Value)}, nil
	case expreval:
		return expreval{rhs.Expression.AddConstant(lhs.Value)}, nil
	}
	return nil, EvaluationError("add")
}

func (lhs liteval) sub(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		lhs.Value -= rhs.Value
		return lhs, nil
	case vareval:
		return expreval{Expression{[]Term{{rhs.Variable, -1.0}}, lhs.Value}}, nil
	case termeval:
		return expreval{Expression{[]Term{rhs.Term.Negate()}, lhs.Value}}, nil
	case expreval:
		rhsxneg := rhs.Expression.Negate()
		return expreval{Expression{rhsxneg.Terms, rhsxneg.Constant + lhs.Value}}, nil
	}
	return nil, EvaluationError("sub")
}

func (lhs liteval) mul(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		lhs.Value *= rhs.Value
		return lhs, nil
	case vareval:
		return termeval{rhs.Variable.Multiply(lhs.Value)}, nil
	case termeval:
		return termeval{rhs.Term.Multiply(lhs.Value)}, nil
	case expreval:
		return expreval{rhs.Expression.Multiply(lhs.Value)}, nil
	}
	return nil, EvaluationError("mul")
}

func (lhs liteval) div(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		lhs.Value /= rhs.Value
		return lhs, nil
	case vareval:
		return nil, EvaluationError("cannot divide by var")
	case termeval:
		return nil, EvaluationError("cannot divide by term")
	case expreval:
		if len(rhs.Expression.Terms) == 0 {
			lhs.Value /= rhs.Expression.Constant
			return lhs, nil
		}
		return nil, EvaluationError("cannot divide by expression")
	}
	return nil, EvaluationError("div")
}

func (lhs liteval) eql(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{Expression{nil, lhs.Value}.EqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{rhs.Variable.EqualsConstant(lhs.Value)}, nil
	case termeval:
		return constreval{rhs.Term.EqualsConstant(lhs.Value)}, nil
	case expreval:
		return constreval{rhs.Expression.EqualsConstant(lhs.Value)}, nil
	}
	return nil, EvaluationError("eql")
}

func (lhs liteval) leq(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{Expression{nil, rhs.Value}.GreaterThanOrEqualsConstant(lhs.Value)}, nil
	case vareval:
		return constreval{rhs.Variable.GreaterThanOrEqualsConstant(lhs.Value)}, nil
	case termeval:
		return constreval{rhs.Term.GreaterThanOrEqualsConstant(lhs.Value)}, nil
	case expreval:
		return constreval{rhs.Expression.GreaterThanOrEqualsConstant(lhs.Value)}, nil
	}
	return nil, EvaluationError("leq")
}

func (lhs liteval) geq(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{Expression{nil, rhs.Value}.LessThanOrEqualsConstant(lhs.Value)}, nil
	case vareval:
		return constreval{rhs.Variable.LessThanOrEqualsConstant(lhs.Value)}, nil
	case termeval:
		return constreval{rhs.Term.LessThanOrEqualsConstant(lhs.Value)}, nil
	case expreval:
		return constreval{rhs.Expression.LessThanOrEqualsConstant(lhs.Value)}, nil
	}
	return nil, EvaluationError("geq")
}

type vareval struct {
	Variable *Variable
}

func (lhs vareval) add(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return expreval{lhs.Variable.AddConstant(rhs.Value)}, nil
	case vareval:
		return expreval{lhs.Variable.AddVariable(rhs.Variable)}, nil
	case termeval:
		return expreval{lhs.Variable.AddTerm(rhs.Term)}, nil
	case expreval:
		return expreval{lhs.Variable.AddExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("add")
}

func (lhs vareval) sub(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return expreval{lhs.Variable.AddConstant(-rhs.Value)}, nil
	case vareval:
		return expreval{lhs.Variable.AddTerm(rhs.Variable.Negate())}, nil
	case termeval:
		return expreval{lhs.Variable.AddTerm(rhs.Term.Negate())}, nil
	case expreval:
		return expreval{lhs.Variable.AddExpression(rhs.Expression.Negate())}, nil
	}
	return nil, EvaluationError("sub")
}

func (lhs vareval) mul(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return termeval{lhs.Variable.Multiply(rhs.Value)}, nil
	case vareval:
		return nil, EvaluationError("cannot multiply var by var")
	case termeval:
		return nil, EvaluationError("cannot multiply var by term")
	case expreval:
		if len(rhs.Expression.Terms) == 0 {
			return termeval{lhs.Variable.Multiply(rhs.Expression.Constant)}, nil
		}
		return nil, EvaluationError("cannot multiply var by expression")
	}
	return nil, EvaluationError("mul")
}

func (lhs vareval) div(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return termeval{lhs.Variable.Divide(rhs.Value)}, nil
	case vareval:
		return nil, EvaluationError("cannot divide var by var")
	case termeval:
		return nil, EvaluationError("cannot divide var by term")
	case expreval:
		if len(rhs.Expression.Terms) == 0 {
			return termeval{lhs.Variable.Divide(rhs.Expression.Constant)}, nil
		}
		return nil, EvaluationError("cannot divide var by expression")
	}
	return nil, EvaluationError("div")
}

func (lhs vareval) eql(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Variable.EqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Variable.EqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Variable.EqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Variable.EqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("eql")
}

func (lhs vareval) leq(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Variable.LessThanOrEqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Variable.LessThanOrEqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Variable.LessThanOrEqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Variable.LessThanOrEqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("leq")
}

func (lhs vareval) geq(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Variable.GreaterThanOrEqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Variable.GreaterThanOrEqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Variable.GreaterThanOrEqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Variable.GreaterThanOrEqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("geq")
}

type termeval struct {
	Term Term
}

func (lhs termeval) add(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return expreval{lhs.Term.AddConstant(rhs.Value)}, nil
	case vareval:
		return expreval{lhs.Term.AddVariable(rhs.Variable)}, nil
	case termeval:
		return expreval{lhs.Term.AddTerm(rhs.Term)}, nil
	case expreval:
		return expreval{lhs.Term.AddExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("add")
}

func (lhs termeval) sub(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return expreval{lhs.Term.AddConstant(-rhs.Value)}, nil
	case vareval:
		return expreval{lhs.Term.AddTerm(rhs.Variable.Negate())}, nil
	case termeval:
		return expreval{lhs.Term.AddTerm(rhs.Term.Negate())}, nil
	case expreval:
		return expreval{lhs.Term.AddExpression(rhs.Expression.Negate())}, nil
	}
	return nil, EvaluationError("sub")
}

func (lhs termeval) mul(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return termeval{lhs.Term.Multiply(rhs.Value)}, nil
	case vareval:
		return nil, EvaluationError("cannot multiply term by var")
	case termeval:
		return nil, EvaluationError("cannot multiply term by term")
	case expreval:
		if len(rhs.Expression.Terms) == 0 {
			return termeval{lhs.Term.Multiply(rhs.Expression.Constant)}, nil
		}
		return nil, EvaluationError("cannot multiply term by expression")
	}
	return nil, EvaluationError("mul")
}

func (lhs termeval) div(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return termeval{lhs.Term.Divide(rhs.Value)}, nil
	case vareval:
		return nil, EvaluationError("cannot divide term by var")
	case termeval:
		return nil, EvaluationError("cannot divide term by term")
	case expreval:
		if len(rhs.Expression.Terms) == 0 {
			return termeval{lhs.Term.Divide(rhs.Expression.Constant)}, nil
		}
		return nil, EvaluationError("cannot divide term by expression")
	}
	return nil, EvaluationError("div")
}

func (lhs termeval) eql(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Term.EqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Term.EqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Term.EqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Term.EqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("eql")
}

func (lhs termeval) leq(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Term.LessThanOrEqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Term.LessThanOrEqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Term.LessThanOrEqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Term.LessThanOrEqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("leq")
}

func (lhs termeval) geq(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Term.GreaterThanOrEqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Term.GreaterThanOrEqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Term.GreaterThanOrEqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Term.GreaterThanOrEqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("geq")
}

type expreval struct {
	Expression Expression
}

func (lhs expreval) add(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return expreval{lhs.Expression.AddConstant(rhs.Value)}, nil
	case vareval:
		return expreval{lhs.Expression.AddVariable(rhs.Variable)}, nil
	case termeval:
		return expreval{lhs.Expression.AddTerm(rhs.Term)}, nil
	case expreval:
		return expreval{lhs.Expression.AddExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("add")
}

func (lhs expreval) sub(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return expreval{lhs.Expression.AddConstant(-rhs.Value)}, nil
	case vareval:
		return expreval{lhs.Expression.AddTerm(rhs.Variable.Negate())}, nil
	case termeval:
		return expreval{lhs.Expression.AddTerm(rhs.Term.Negate())}, nil
	case expreval:
		return expreval{lhs.Expression.AddExpression(rhs.Expression.Negate())}, nil
	}
	return nil, EvaluationError("sub")
}

func (lhs expreval) mul(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return expreval{lhs.Expression.Multiply(rhs.Value)}, nil
	case vareval:
		if len(lhs.Expression.Terms) == 0 {
			return liteval{lhs.Expression.Constant}.mul(rhs)
		}
		return nil, EvaluationError("cannot multiply expression by var")
	case termeval:
		if len(lhs.Expression.Terms) == 0 {
			return liteval{lhs.Expression.Constant}.mul(rhs)
		}
		return nil, EvaluationError("cannot multiply expresion by term")
	case expreval:
		if len(lhs.Expression.Terms) == 0 {
			return liteval{lhs.Expression.Constant}.mul(rhs)
		}
		return nil, EvaluationError("cannot multiply term by expression")
	}
	return nil, EvaluationError("mul")
}

func (lhs expreval) div(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return expreval{lhs.Expression.Divide(rhs.Value)}, nil
	case vareval:
		return nil, EvaluationError("cannot divide expression by var")
	case termeval:
		return nil, EvaluationError("cannot divide expression by term")
	case expreval:
		if len(rhs.Expression.Terms) == 0 {
			return expreval{lhs.Expression.Divide(rhs.Expression.Constant)}, nil
		}
		return nil, EvaluationError("cannot divide expression by expression")
	}
	return nil, EvaluationError("div")
}

func (lhs expreval) eql(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Expression.EqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Expression.EqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Expression.EqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Expression.EqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("eql")
}

func (lhs expreval) leq(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Expression.LessThanOrEqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Expression.LessThanOrEqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Expression.LessThanOrEqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Expression.LessThanOrEqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("leq")
}

func (lhs expreval) geq(other evaluation) (evaluation, error) {
	switch rhs := other.(type) {
	case liteval:
		return constreval{lhs.Expression.GreaterThanOrEqualsConstant(rhs.Value)}, nil
	case vareval:
		return constreval{lhs.Expression.GreaterThanOrEqualsVariable(rhs.Variable)}, nil
	case termeval:
		return constreval{lhs.Expression.GreaterThanOrEqualsTerm(rhs.Term)}, nil
	case expreval:
		return constreval{lhs.Expression.GreaterThanOrEqualsExpression(rhs.Expression)}, nil
	}
	return nil, EvaluationError("geq")
}

type constreval struct {
	Constraint *Constraint
}

func (constreval) add(evaluation) (evaluation, error) {
	return nil, EvaluationError("cannot add to a constraint")
}

func (constreval) sub(evaluation) (evaluation, error) {
	return nil, EvaluationError("cannot subtract from a constraint")
}

func (constreval) mul(evaluation) (evaluation, error) {
	return nil, EvaluationError("cannot multiply a constraint")
}

func (constreval) div(evaluation) (evaluation, error) {
	return nil, EvaluationError("cannot divide a constraint")
}

func (constreval) eql(evaluation) (evaluation, error) {
	return nil, EvaluationError("cannot create a linear equation from a constraint")
}

func (constreval) leq(evaluation) (evaluation, error) {
	return nil, EvaluationError("cannot create a linear inequality from a constraint")
}

func (constreval) geq(evaluation) (evaluation, error) {
	return nil, EvaluationError("cannot create a linear inequality from a constraint")
}
