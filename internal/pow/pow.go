package pow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/bits"
	"strconv"
	"time"
)

const (
	defaultSeparator = ":"
	defaultMaxIters  = uint64(5_000_000)
	defaultTimeout   = 5 * time.Second
	pollInterval     = 4096
)

var ErrNotFound = errors.New("pow: no solution within bounds")

type Challenge struct {
	Prefix     string
	Separator  string
	Difficulty int
	MaxIters   uint64
	Timeout    time.Duration
}

type Solution struct {
	Nonce   string
	Hash    string
	Iters   uint64
	Elapsed time.Duration
}

func Solve(ctx context.Context, c Challenge) (Solution, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	sep := c.Separator
	if sep == "" {
		sep = defaultSeparator
	}

	maxIters := c.MaxIters
	if maxIters == 0 {
		maxIters = defaultMaxIters
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	start := time.Now()
	deadline := start.Add(timeout)

	base := c.Prefix + sep

	if c.Difficulty <= 0 {
		nonce := "0"
		sum := sha256.Sum256([]byte(base + nonce))
		return Solution{
			Nonce:   nonce,
			Hash:    hex.EncodeToString(sum[:]),
			Iters:   1,
			Elapsed: time.Since(start),
		}, nil
	}

	for n := uint64(0); ; n++ {
		if err := budgetExceeded(ctx, n, maxIters, deadline); err != nil {
			return Solution{}, err
		}

		nonce := strconv.FormatUint(n, 10)
		sum := sha256.Sum256([]byte(base + nonce))
		if LeadingZeroBits(sum) >= c.Difficulty {
			return Solution{
				Nonce:   nonce,
				Hash:    hex.EncodeToString(sum[:]),
				Iters:   n + 1,
				Elapsed: time.Since(start),
			}, nil
		}
	}
}

func budgetExceeded(ctx context.Context, n, maxIters uint64, deadline time.Time) error {
	if n >= maxIters {
		return ErrNotFound
	}

	if n%pollInterval != 0 {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}

	return nil
}

func Verify(prefix, separator, nonce string, difficulty int) bool {
	if difficulty <= 0 {
		return true
	}

	sep := separator
	if sep == "" {
		sep = defaultSeparator
	}

	sum := sha256.Sum256([]byte(prefix + sep + nonce))
	return LeadingZeroBits(sum) >= difficulty
}

func LeadingZeroBits(sum [32]byte) int {
	count := 0
	for _, b := range sum {
		if b == 0 {
			count += 8
			continue
		}
		count += bits.LeadingZeros8(b)
		break
	}
	return count
}
