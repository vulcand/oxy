package cbreaker

import (
	"fmt"

	"github.com/vulcand/predicate"
)

type hpredicate func(*CircuitBreaker) bool

// parseExpression parses expression in the go language into predicates.
func parseExpression(in string) (hpredicate, error) {
	p, err := predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: and,
			OR:  or,
			EQ:  eq,
			NEQ: neq,
			LT:  lt,
			LE:  le,
			GT:  gt,
			GE:  ge,
		},
		Functions: map[string]any{
			"LatencyAtQuantileMS": latencyAtQuantile,
			"NetworkErrorRatio":   networkErrorRatio,
			"ResponseCodeRatio":   responseCodeRatio,
		},
	})
	if err != nil {
		return nil, err
	}

	out, err := p.Parse(in)
	if err != nil {
		return nil, err
	}

	pr, ok := out.(hpredicate)
	if !ok {
		return nil, fmt.Errorf("expected predicate, got %T", out)
	}

	return pr, nil
}
