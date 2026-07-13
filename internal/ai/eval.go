package ai

import (
	"fmt"
	"strconv"
	"strings"
)

// EvalExpr evaluates a small integer arithmetic expression (+ - * / ( ),
// unary minus, no floats) -- used to sanity-check a word problem's `check`
// field against its stored numeric answer. Integer division must be exact;
// division by zero or a non-exact division is an error.
func EvalExpr(expr string) (int, error) {
	p := &exprParser{tokens: tokenize(expr)}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos != len(p.tokens) {
		return 0, fmt.Errorf("unexpected token %q", p.tokens[p.pos])
	}
	return v, nil
}

func tokenize(expr string) []string {
	var tokens []string
	var num strings.Builder
	flush := func() {
		if num.Len() > 0 {
			tokens = append(tokens, num.String())
			num.Reset()
		}
	}
	for _, r := range expr {
		switch {
		case r >= '0' && r <= '9':
			num.WriteRune(r)
		case r == ' ' || r == '\t':
			flush()
		case strings.ContainsRune("+-*/()", r):
			flush()
			tokens = append(tokens, string(r))
		default:
			flush()
			tokens = append(tokens, string(r)) // invalid char, surfaces as a parse error
		}
	}
	flush()
	return tokens
}

type exprParser struct {
	tokens []string
	pos    int
}

func (p *exprParser) peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos]
}

func (p *exprParser) next() string {
	t := p.peek()
	p.pos++
	return t
}

// parseExpr := term (('+' | '-') term)*
func (p *exprParser) parseExpr() (int, error) {
	v, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.peek() == "+" || p.peek() == "-" {
		op := p.next()
		rhs, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == "+" {
			v += rhs
		} else {
			v -= rhs
		}
	}
	return v, nil
}

// parseTerm := factor (('*' | '/') factor)*
func (p *exprParser) parseTerm() (int, error) {
	v, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for p.peek() == "*" || p.peek() == "/" {
		op := p.next()
		rhs, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == "*" {
			v *= rhs
			continue
		}
		if rhs == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		if v%rhs != 0 {
			return 0, fmt.Errorf("inexact integer division: %d / %d", v, rhs)
		}
		v /= rhs
	}
	return v, nil
}

// parseFactor := ['-'] ( number | '(' expr ')' )
func (p *exprParser) parseFactor() (int, error) {
	if p.peek() == "-" {
		p.next()
		v, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -v, nil
	}
	if p.peek() == "(" {
		p.next()
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ")" {
			return 0, fmt.Errorf("expected ')'")
		}
		p.next()
		return v, nil
	}
	tok := p.next()
	n, err := strconv.Atoi(tok)
	if err != nil {
		return 0, fmt.Errorf("expected number, got %q", tok)
	}
	return n, nil
}
