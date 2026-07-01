package cbreaker

import (
	"fmt"
)

// or returns predicate by joining the passed predicates with logical 'or'.
func or(fns ...hpredicate) hpredicate {
	return func(c *CircuitBreaker) bool {
		for _, fn := range fns {
			if fn(c) {
				return true
			}
		}

		return false
	}
}

// and returns predicate by joining the passed predicates with logical 'and'.
func and(fns ...hpredicate) hpredicate {
	return func(c *CircuitBreaker) bool {
		for _, fn := range fns {
			if !fn(c) {
				return false
			}
		}

		return true
	}
}

// not creates negation of the passed predicate.
func not(p hpredicate) hpredicate {
	return func(c *CircuitBreaker) bool {
		return !p(c)
	}
}

// eq returns predicate that tests for equality of the value of the mapper and the constant.
func eq(m any, value any) (hpredicate, error) {
	switch mapper := m.(type) {
	case toType[int]:
		return genericEQ(mapper, value)
	case toType[int64]:
		return int64EQ(mapper, value)
	case toType[float64]:
		return genericEQ(mapper, value)
	}

	return nil, fmt.Errorf("eq: unsupported argument: %T", m)
}

// neq returns predicate that tests for inequality of the value of the mapper and the constant.
func neq(m any, value any) (hpredicate, error) {
	p, err := eq(m, value)
	if err != nil {
		return nil, err
	}

	return not(p), nil
}

// lt returns predicate that tests that value of the mapper function is less than the constant.
func lt(m any, value any) (hpredicate, error) {
	switch mapper := m.(type) {
	case toType[int]:
		return genericLT(mapper, value)
	case toType[int64]:
		return int64LT(mapper, value)
	case toType[float64]:
		return genericLT(mapper, value)
	}

	return nil, fmt.Errorf("lt: unsupported argument: %T", m)
}

// le returns predicate that tests that value of the mapper function is less or equal than the constant.
func le(m any, value any) (hpredicate, error) {
	l, err := lt(m, value)
	if err != nil {
		return nil, err
	}

	e, err := eq(m, value)
	if err != nil {
		return nil, err
	}

	return func(c *CircuitBreaker) bool {
		return l(c) || e(c)
	}, nil
}

// gt returns predicate that tests that value of the mapper function is greater than the constant.
func gt(m any, value any) (hpredicate, error) {
	switch mapper := m.(type) {
	case toType[int]:
		return genericGT(mapper, value)
	case toType[int64]:
		return int64GT(mapper, value)
	case toType[float64]:
		return genericGT(mapper, value)
	}

	return nil, fmt.Errorf("gt: unsupported argument: %T", m)
}

// ge returns predicate that tests that value of the mapper function is less or equal than the constant.
func ge(m any, value any) (hpredicate, error) {
	g, err := gt(m, value)
	if err != nil {
		return nil, err
	}

	e, err := eq(m, value)
	if err != nil {
		return nil, err
	}

	return func(c *CircuitBreaker) bool {
		return g(c) || e(c)
	}, nil
}

func genericEQ[T int | float64](m toType[T], val any) (hpredicate, error) {
	value, ok := val.(T)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}

	return func(c *CircuitBreaker) bool {
		return m(c) == value
	}, nil
}

func genericLT[T int | float64](m toType[T], val any) (hpredicate, error) {
	value, ok := val.(T)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}

	return func(c *CircuitBreaker) bool {
		return m(c) < value
	}, nil
}

func genericGT[T int | float64](m toType[T], val any) (hpredicate, error) {
	value, ok := val.(T)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}

	return func(c *CircuitBreaker) bool {
		return m(c) > value
	}, nil
}

func int64EQ(m toType[int64], val any) (hpredicate, error) {
	// Use `int` instead of `int64` because `vulcand/predicate` only use `int`.
	// Note: using `int64` should be the default type of any integer, but changing this will break `vulcand/predicate`.
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}

	return func(c *CircuitBreaker) bool {
		return m(c) == int64(value)
	}, nil
}

func int64LT(m toType[int64], val any) (hpredicate, error) {
	// Use `int` instead of `int64` because `vulcand/predicate` only use `int`.
	// Note: using `int64` should be the default type of any integer, but changing this will break `vulcand/predicate`.
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}

	return func(c *CircuitBreaker) bool {
		return m(c) < int64(value)
	}, nil
}

func int64GT(m toType[int64], val any) (hpredicate, error) {
	// Use `int` instead of `int64` because `vulcand/predicate` only use `int`.
	// Note: using `int64` should be the default type of any integer, but changing this will break `vulcand/predicate`.
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}

	return func(c *CircuitBreaker) bool {
		return m(c) > int64(value)
	}, nil
}
