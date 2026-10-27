package manager

import (
	"math"
	"time"
)

const rampWindow = 24 * time.Hour

// RecommendedRampDays returns how many days are needed to send total mails when day one
// sends at most start mails and every following day sends growth times the previous day.
func RecommendedRampDays(total, start int, growth float64) int {
	if total <= start || start < 1 {
		return 1
	}
	if growth <= 1 {
		return int(math.Ceil(float64(total) / float64(start)))
	}

	ratio := float64(total) / float64(start)
	return int(math.Ceil(math.Log(ratio*(growth-1)+1)/math.Log(growth) - 1e-9))
}

// RampCaps splits total into one cap per day, each day growth times the previous one.
func RampCaps(total, days int, growth float64) []int {
	days = max(days, 1)
	growth = math.Max(growth, 1)

	weightSum := 0.0
	for i := 0; i < days; i++ {
		weightSum += math.Pow(growth, float64(i))
	}

	caps := make([]int, days)
	remaining := total
	for i := 0; i < days-1; i++ {
		caps[i] = int(math.Round(float64(total) * math.Pow(growth, float64(i)) / weightSum))
		remaining -= caps[i]
	}
	caps[days-1] = max(remaining, 0)

	return caps
}

type rampUp struct {
	caps  []int
	start time.Time
}

func (r rampUp) day(now time.Time) int {
	return max(int(now.Sub(r.start)/rampWindow), 0)
}

func (r rampUp) isLastDay(day int) bool {
	return day >= len(r.caps)-1
}

func (r rampUp) allowance(day, sent int) int {
	allowed := 0
	for i := 0; i <= day && i < len(r.caps); i++ {
		allowed += r.caps[i]
	}

	return allowed - sent
}

func (r rampUp) nextWindow(day int) time.Time {
	return r.start.Add(time.Duration(day+1) * rampWindow)
}
