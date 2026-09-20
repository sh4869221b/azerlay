package input

import (
	"math"
	"testing"
)

func TestNormalizeAxis(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		raw   int32
		info  AxisInfo
		want  float64
		valid bool
	}{
		{"signed min", -32768, AxisInfo{Minimum: -32768, Maximum: 32767}, -1, true},
		{"signed max", 32767, AxisInfo{Minimum: -32768, Maximum: 32767}, 1, true},
		{"offset min", 0, AxisInfo{Maximum: 255}, -1, true},
		{"offset max", 255, AxisInfo{Maximum: 255}, 1, true},
		{"center", 100, AxisInfo{Maximum: 200, Flat: 20}, 0, true},
		{"flat left", 80, AxisInfo{Maximum: 200, Flat: 20}, 0, true},
		{"flat right", 120, AxisInfo{Maximum: 200, Flat: 20}, 0, true},
		{"outside flat left", 40, AxisInfo{Maximum: 200, Flat: 20}, -.5, true},
		{"outside flat right", 160, AxisInfo{Maximum: 200, Flat: 20}, .5, true},
		{"clamp low", -50, AxisInfo{Maximum: 200}, -1, true},
		{"clamp high", 250, AxisInfo{Maximum: 200}, 1, true},
		{"extreme min", math.MinInt32, AxisInfo{Minimum: math.MinInt32, Maximum: math.MaxInt32}, -1, true},
		{"extreme max", math.MaxInt32, AxisInfo{Minimum: math.MinInt32, Maximum: math.MaxInt32}, 1, true},
		{"fuzz metadata", 160, AxisInfo{Maximum: 200, Flat: 20, Fuzz: 1000}, .5, true},
		{"equal", 0, AxisInfo{}, 0, false},
		{"reversed", 0, AxisInfo{Minimum: 1}, 0, false},
		{"negative flat", 0, AxisInfo{Maximum: 200, Flat: -1}, 0, false},
		{"excessive flat", 0, AxisInfo{Maximum: 200, Flat: 100}, 0, false},
		{"negative fuzz", 0, AxisInfo{Maximum: 200, Fuzz: -1}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, valid := NormalizeAxis(tc.raw, tc.info)
			if valid != tc.valid || math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-tc.want) > 1e-12 {
				t.Fatalf("NormalizeAxis = %v,%t; want %v,%t", got, valid, tc.want, tc.valid)
			}
		})
	}
}
