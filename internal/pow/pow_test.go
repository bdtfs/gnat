package pow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

func TestLeadingZeroBits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sum  [32]byte
		want int
	}{
		{
			name: "all zero",
			sum:  [32]byte{},
			want: 256,
		},
		{
			name: "first byte 0xFF",
			sum:  [32]byte{0xFF},
			want: 0,
		},
		{
			name: "twelve leading zeros",
			sum:  [32]byte{0x00, 0x0F, 0xFF, 0xFF},
			want: 12,
		},
		{
			name: "one leading zero",
			sum:  [32]byte{0x7F},
			want: 1,
		},
		{
			name: "eight leading zeros",
			sum:  [32]byte{0x00, 0x80},
			want: 8,
		},
		{
			name: "sixteen leading zeros",
			sum:  [32]byte{0x00, 0x00, 0xFF},
			want: 16,
		},
		{
			name: "partial high bits in second byte",
			sum:  [32]byte{0x00, 0x01},
			want: 15,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := LeadingZeroBits(tt.sum); got != tt.want {
				t.Errorf("LeadingZeroBits(%v) = %d, want %d", tt.sum, got, tt.want)
			}
		})
	}
}

func TestSolveAndVerify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		prefix     string
		separator  string
		difficulty int
	}{
		{name: "difficulty 0", prefix: "gnat-d0", separator: ":", difficulty: 0},
		{name: "difficulty 4", prefix: "gnat-d4", separator: ":", difficulty: 4},
		{name: "difficulty 8", prefix: "gnat-d8", separator: ":", difficulty: 8},
		{name: "difficulty 12", prefix: "gnat-d12", separator: ":", difficulty: 12},
		{name: "empty separator defaults", prefix: "gnat-defsep", separator: "", difficulty: 8},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sol, err := Solve(context.Background(), Challenge{
				Prefix:     tt.prefix,
				Separator:  tt.separator,
				Difficulty: tt.difficulty,
			})
			if err != nil {
				t.Fatalf("Solve returned error: %v", err)
			}

			if !Verify(tt.prefix, tt.separator, sol.Nonce, tt.difficulty) {
				t.Errorf("Verify failed for nonce %q at difficulty %d", sol.Nonce, tt.difficulty)
			}

			sep := tt.separator
			if sep == "" {
				sep = ":"
			}
			sum := sha256.Sum256([]byte(tt.prefix + sep + sol.Nonce))
			if got := LeadingZeroBits(sum); got < tt.difficulty {
				t.Errorf("digest has %d leading zero bits, want >= %d", got, tt.difficulty)
			}

			if sol.Hash != hex.EncodeToString(sum[:]) {
				t.Errorf("Solution.Hash = %q, want %q", sol.Hash, hex.EncodeToString(sum[:]))
			}

			if sol.Iters == 0 {
				t.Errorf("Solution.Iters = 0, want >= 1")
			}
		})
	}
}

func TestSolveDifficulty12Acceptance(t *testing.T) {
	t.Parallel()

	const prefix = "gnat-accept"
	sol, err := Solve(context.Background(), Challenge{Prefix: prefix, Difficulty: 12})
	if err != nil {
		t.Fatalf("Solve returned error: %v", err)
	}

	if !Verify(prefix, ":", sol.Nonce, 12) {
		t.Fatalf("Verify(%q, %q, %q, 12) = false", prefix, ":", sol.Nonce)
	}

	sum := sha256.Sum256([]byte(prefix + ":" + sol.Nonce))
	if got := LeadingZeroBits(sum); got < 12 {
		t.Fatalf("digest leading zero bits = %d, want >= 12", got)
	}
}

func TestSolveMaxItersBounded(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	var err error
	go func() {
		_, err = Solve(context.Background(), Challenge{
			Prefix:     "gnat-hard",
			Difficulty: 256,
			MaxIters:   1,
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Solve hung with MaxIters=1 and high difficulty")
	}

	if err == nil {
		t.Fatal("expected non-nil error when MaxIters exceeded")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSolveTimeoutBounded(t *testing.T) {
	t.Parallel()

	_, err := Solve(context.Background(), Challenge{
		Prefix:     "gnat-timeout",
		Difficulty: 256,
		MaxIters:   1 << 62,
		Timeout:    50 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected non-nil error when Timeout elapses")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestSolveContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Solve(ctx, Challenge{
		Prefix:     "gnat-cancel",
		Difficulty: 256,
		MaxIters:   1 << 62,
		Timeout:    time.Hour,
	})
	if err == nil {
		t.Fatal("expected non-nil error when context cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestVerifyTrivialDifficulty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		difficulty int
		want       bool
	}{
		{name: "zero difficulty", difficulty: 0, want: true},
		{name: "negative difficulty", difficulty: -5, want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Verify("anything", ":", "999", tt.difficulty); got != tt.want {
				t.Errorf("Verify difficulty %d = %v, want %v", tt.difficulty, got, tt.want)
			}
		})
	}
}

func TestVerifySeparatorDefault(t *testing.T) {
	t.Parallel()

	const prefix = "gnat-sepdefault"
	sol, err := Solve(context.Background(), Challenge{Prefix: prefix, Difficulty: 8})
	if err != nil {
		t.Fatalf("Solve returned error: %v", err)
	}

	if !Verify(prefix, "", sol.Nonce, 8) {
		t.Error("Verify with empty separator should default to ':'")
	}
	if !Verify(prefix, ":", sol.Nonce, 8) {
		t.Error("Verify with explicit ':' should match")
	}
}

func TestSolveDifficultyZeroNonceZero(t *testing.T) {
	t.Parallel()

	sol, err := Solve(context.Background(), Challenge{Prefix: "x", Difficulty: 0})
	if err != nil {
		t.Fatalf("Solve returned error: %v", err)
	}
	if sol.Nonce != "0" {
		t.Errorf("Solution.Nonce = %q, want %q", sol.Nonce, "0")
	}
}
