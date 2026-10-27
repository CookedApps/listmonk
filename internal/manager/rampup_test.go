package manager

import (
	"testing"
	"time"
)

func TestRecommendedRampDays(t *testing.T) {
	cases := []struct {
		total, start int
		growth       float64
		want         int
	}{
		{100, 2000, 2, 1},
		{2000, 2000, 2, 1},
		{2001, 2000, 2, 2},
		{14000, 2000, 2, 3},
		{100000, 2000, 2, 6},
		{10000, 2000, 1, 5},
		{10000, 0, 2, 1},
	}
	for _, c := range cases {
		if got := RecommendedRampDays(c.total, c.start, c.growth); got != c.want {
			t.Errorf("RecommendedRampDays(%d, %d, %v) = %d, want %d", c.total, c.start, c.growth, got, c.want)
		}
	}
}

func TestRampCapsSumToTotalAndStayUnderStart(t *testing.T) {
	for _, total := range []int{1, 7, 2000, 2001, 14000, 100000, 1234567} {
		days := RecommendedRampDays(total, 2000, 2)
		caps := RampCaps(total, days, 2)

		sum := 0
		for _, c := range caps {
			sum += c
		}
		if sum != total {
			t.Errorf("caps for %d sum to %d: %v", total, sum, caps)
		}
		if caps[0] > 2000 {
			t.Errorf("day one cap %d exceeds start for total %d: %v", caps[0], total, caps)
		}
		for i := 1; i < len(caps); i++ {
			if caps[i] < caps[i-1] {
				t.Errorf("caps not growing for total %d: %v", total, caps)
			}
		}
	}
}

func TestRampCapsExample(t *testing.T) {
	got := RampCaps(100000, 6, 2)
	want := []int{1587, 3175, 6349, 12698, 25397, 50794}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RampCaps(100000, 6, 2) = %v, want %v", got, want)
		}
	}
}

func TestRampUpWindows(t *testing.T) {
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	r := rampUp{caps: []int{10, 20, 40}, start: start}

	if d := r.day(start.Add(-time.Hour)); d != 0 {
		t.Errorf("day before start = %d, want 0", d)
	}
	if d := r.day(start.Add(25 * time.Hour)); d != 1 {
		t.Errorf("day after 25h = %d, want 1", d)
	}
	if a := r.allowance(1, 12); a != 18 {
		t.Errorf("allowance(1, 12) = %d, want 18", a)
	}
	if a := r.allowance(5, 70); a != 0 {
		t.Errorf("allowance beyond last day = %d, want 0", a)
	}
	if !r.isLastDay(2) || r.isLastDay(1) {
		t.Error("isLastDay wrong")
	}
	if next := r.nextWindow(1); !next.Equal(start.Add(48 * time.Hour)) {
		t.Errorf("nextWindow(1) = %v", next)
	}
}
