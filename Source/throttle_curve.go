package main

import "math"

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// normalizeScalePercent (max throttle) maps unset/invalid values to 100 (default) and clamps to 10..200.
// Above 100 the stick sum is amplified, so full thrust stays reachable while one stick pulls back.
func normalizeScalePercent(pct int) int {
	if pct <= 0 {
		return 100
	}
	if pct > 200 {
		return 200
	}
	if pct < 10 {
		return 10
	}
	return pct
}

// normalizeSteerScalePercent keeps max steering 50..100 (one stick full); invalid -> 100, below 50 -> 50.
func normalizeSteerScalePercent(pct int) int {
	if pct <= 0 || pct > 100 {
		return 100
	}
	if pct < 50 {
		return 50
	}
	return pct
}

// normalizeExpoPercent keeps -100..100; out-of-range values fall back to linear (0).
// Shared by throttle curve and steer curve. Positive = softer center, negative = more aggressive center.
func normalizeExpoPercent(pct int) int {
	if pct < -100 || pct > 100 {
		return 0
	}
	return pct
}

// clampExpoPercent clamps user input from the UI to -100..100 (unlike config normalization, no fallback to 0).
func clampExpoPercent(pct int) int {
	if pct < -100 {
		return -100
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// curveShape maps |t| (0..1) through the expo curve and keeps the sign; 0 -> 0 and 1 -> 1 for every expo.
// Positive expo: (1-e)t + e*t^3 (soft center). Negative expo: mirrored curve 1-g(1-t) (aggressive center).
func curveShape(t float64, expo int) float64 {
	sign := 1.0
	if t < 0 {
		sign, t = -1, -t
	}
	t = math.Min(t, 1)
	e := float64(normalizeExpoPercent(expo)) / 100.0
	g := func(v, a float64) float64 { return (1-a)*v + a*v*v*v }
	if e >= 0 {
		return sign * g(t, e)
	}
	return sign * (1 - g(1-t, -e))
}

// applyCurve clamps x to -1..1, applies the expo curve and scales by max percent.
// Expo keeps full deflection at 1 and only reshapes the center; scale caps the end value.
func applyCurve(x float64, scale, expo int) float64 {
	x = curveShape(clamp(x, -1, 1), expo)
	return x * float64(normalizeScalePercent(scale)) / 100.0
}

// calculateSteering mixes L-R (-2..2) in two segments of t=|L-R|/2: segment 1 (0..0.5, one stick)
// runs 0 -> max steering with curve expo1; segment 2 (0.5..1, counter-stick) runs max steering -> 100%
// with its own curve expo2, so e.g. a soft start and an aggressive counter-steer can be combined.
func calculateSteering(left, right float64, scale, expo1, expo2 int) float64 {
	d := clamp(left-right, -2, 2)
	sign := 1.0
	if d < 0 {
		sign, d = -1, -d
	}
	t := d / 2
	s := float64(normalizeSteerScalePercent(scale)) / 100.0
	if t <= 0.5 {
		return sign * s * curveShape(2*t, expo1)
	}
	return sign * math.Min(s+(1-s)*curveShape(2*t-1, expo2), 1)
}

// calculateDrive sums both sticks and applies throttle curve + max throttle, symmetric for forward and reverse.
// At scale 100/expo 0: one stick full + other centered = 100%, both at 50% = 100%, full forward + full back = 0.
// Scale <=100 caps the output; scale >100 amplifies the sum (e.g. 200: left full + right half back = 100%).
func calculateDrive(left, right float64, scale, expo int) float64 {
	s := normalizeScalePercent(scale)
	if s <= 100 {
		return applyCurve(left+right, s, expo)
	}
	return applyCurve((left+right)*float64(s)/100.0, 100, expo)
}
