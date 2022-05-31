package ratelimit

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// TokenBucketSet represents a set of TokenBucket covering different time periods.
type TokenBucketSet struct {
	buckets   map[time.Duration]*tokenBucket
	maxPeriod time.Duration
}

// NewTokenBucketSet creates a `TokenBucketSet` from the specified `rates`.
func NewTokenBucketSet(rates *RateSet) *TokenBucketSet {
	tbs := new(TokenBucketSet)
	// In the majority of cases we will have only one bucket.
	tbs.buckets = make(map[time.Duration]*tokenBucket, len(rates.m))
	for _, rate := range rates.m {
		newBucket := newTokenBucket(rate)
		tbs.buckets[rate.period] = newBucket
		tbs.maxPeriod = maxDuration(tbs.maxPeriod, rate.period)
	}
	return tbs
}

// Update brings the buckets in the set in accordance with the provided `rates`.
func (tbs *TokenBucketSet) Update(rates *RateSet) {
	// Update existing buckets and delete those that have no corresponding spec.
	for _, bucket := range tbs.buckets {
		if rate, ok := rates.m[bucket.period]; ok {
			_ = bucket.update(rate)
		} else {
			delete(tbs.buckets, bucket.period)
		}
	}
	// Add missing buckets.
	for _, rate := range rates.m {
		if _, ok := tbs.buckets[rate.period]; !ok {
			newBucket := newTokenBucket(rate)
			tbs.buckets[rate.period] = newBucket
		}
	}
	// Identify the maximum period in the set
	tbs.maxPeriod = 0
	for _, bucket := range tbs.buckets {
		tbs.maxPeriod = maxDuration(tbs.maxPeriod, bucket.period)
	}
}

// Consume consume tokens.
func (tbs *TokenBucketSet) Consume(tokens int64) (time.Duration, error) {
	var maxDelay time.Duration = UndefinedDelay
	var firstErr error
	for _, tokenBucket := range tbs.buckets {
		// We keep calling `Consume` even after a error is returned for one of
		// buckets because that allows us to simplify the rollback procedure,
		// that is to just call `Rollback` for all buckets.
		delay, err := tokenBucket.consume(tokens)
		if firstErr == nil {
			if err != nil {
				firstErr = err
			} else {
				maxDelay = maxDuration(maxDelay, delay)
			}
		}
	}
	// If we could not make ALL buckets consume tokens for whatever reason,
	// then rollback consumption for all of them.
	if firstErr != nil || maxDelay > 0 {
		for _, tokenBucket := range tbs.buckets {
			tokenBucket.rollback()
		}
	}
	return maxDelay, firstErr
}

// GetMaxPeriod returns the max period.
func (tbs *TokenBucketSet) GetMaxPeriod() time.Duration {
	return tbs.maxPeriod
}

// debugState returns string that reflects the current state of all buckets in
// this set. It is intended to be used for debugging and testing only.
func (tbs *TokenBucketSet) debugState() string {
	periods := make([]int64, 0, len(tbs.buckets))
	for period := range tbs.buckets {
		periods = append(periods, int64(period))
	}
	sort.Slice(periods, func(i, j int) bool { return periods[i] < periods[j] })
	bucketRepr := make([]string, 0, len(tbs.buckets))
	for _, period := range periods {
		bucket := tbs.buckets[time.Duration(period)]
		bucketRepr = append(bucketRepr, fmt.Sprintf("{%v: %v}", bucket.period, bucket.availableTokens))
	}
	return strings.Join(bucketRepr, ", ")
}

func maxDuration(x, y time.Duration) time.Duration {
	if x > y {
		return x
	}
	return y
}
