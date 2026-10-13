//go:build windows

package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestLeverInvertAndNegativePolarity(t *testing.T) {
	for _, test := range []struct {
		name             string
		raw              float64
		invert, negative bool
		want             float64
	}{
		{"released normal", -1, false, false, 0},
		{"pressed normal", 1, false, false, 1},
		{"released inverted", -1, true, false, 1},
		{"pressed inverted", 1, true, false, 0},
		{"released negative", -1, false, true, 0},
		{"pressed negative", 1, false, true, -1},
		{"released inverted negative", -1, true, true, -1},
		{"pressed inverted negative", 1, true, true, 0},
		{"half travel", 0, false, false, .5},
		{"half travel negative", 0, false, true, -.5},
		{"half travel inverted negative", 0, true, true, -.5},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := normalizeAuxiliaryValue(test.raw, auxiliaryAxisConfig{
				AxisType: "Lever", Invert: test.invert, OutputNegative: test.negative,
			}, 0)
			if math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
}

func TestNegativeLeverDeadzoneAndCenteredIsolation(t *testing.T) {
	lever := auxiliaryAxisConfig{AxisType: "Lever", OutputNegative: true}
	for _, tc := range []struct {
		input float64
		want  float64
	}{
		{-1, 0}, {-.8, 0}, {0, -.375}, {1, -1},
	} {
		got := normalizeAuxiliaryValue(tc.input, lever, 20)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("lever at %v got %v want %v", tc.input, got, tc.want)
		}
	}
	center := auxiliaryAxisConfig{AxisType: "Centered", OutputNegative: true}
	if got := normalizeAuxiliaryValue(.6, center, 0); got != .6 {
		t.Errorf("centered positive changed: %v", got)
	}
	center.Invert = true
	if got := normalizeAuxiliaryValue(.6, center, 0); got != -.6 {
		t.Errorf("centered inversion changed: %v", got)
	}
}

func TestLeverOutputNegativeConfigRoundTrip(t *testing.T) {
	var previous auxiliaryAxisConfig
	if err := json.Unmarshal([]byte(`{"axis_type":"Lever","invert":true}`), &previous); err != nil {
		t.Fatal(err)
	}
	if previous.OutputNegative {
		t.Fatal("legacy configurations should default output_negative to false")
	}
	previous.OutputNegative = true
	b, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded auxiliaryAxisConfig
	if err := json.Unmarshal(b, &reloaded); err != nil {
		t.Fatal(err)
	}
	if !reloaded.OutputNegative || !reloaded.Invert {
		t.Fatalf("lost setting: %+v", reloaded)
	}
}
