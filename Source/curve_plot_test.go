//go:build windows

package main

import (
	"math"
	"testing"
)

func TestCurvePlotPointMapsAndClamps(t *testing.T) {
	plot := rect{Left: 10, Top: 20, Right: 110, Bottom: 120}
	cases := []struct {
		fx, fy float64
		x, y   int32
	}{
		{0, 0, 10, 120},
		{1, 1, 110, 20},
		{0.5, 0.25, 60, 95},
		{-1, 2, 10, 20},
	}
	for _, tc := range cases {
		x, y := curvePlotPoint(plot, tc.fx, tc.fy)
		if x != tc.x || y != tc.y {
			t.Fatalf("curvePlotPoint(%v,%v)=(%d,%d), want (%d,%d)", tc.fx, tc.fy, x, y, tc.x, tc.y)
		}
	}
}

func TestThrustCurveSampleUsesMagnitude(t *testing.T) {
	x, y := thrustCurveSample(-0.5, 80, 0)
	if math.Abs(x-0.5) > 1e-9 || math.Abs(y-0.4) > 1e-9 {
		t.Fatalf("got (%v,%v), want (0.5,0.4)", x, y)
	}
	x, y = thrustCurveSample(1.6, 100, 0)
	if x != 1 || y != 1 {
		t.Fatalf("clamp: got (%v,%v), want (1,1)", x, y)
	}
}

func TestSteerCurveSampleCoversCounterStickRange(t *testing.T) {
	cases := []struct {
		in, x, y float64
	}{
		{0, 0, 0},
		{1, 0.5, 0.7},
		{-1.5, 0.75, 0.85},
		{2, 1, 1},
		{3, 1, 1},
	}
	for _, tc := range cases {
		x, y := steerCurveSample(tc.in, 70, 0, 0)
		if math.Abs(x-tc.x) > 1e-9 || math.Abs(y-tc.y) > 1e-9 {
			t.Fatalf("steerCurveSample(%v)=(%v,%v), want (%v,%v)", tc.in, x, y, tc.x, tc.y)
		}
	}
}

func TestCurvePlotAreaRejectsSmallCards(t *testing.T) {
	if _, _, _, ok := curvePlotArea(rect{Right: 600, Bottom: 100}); ok {
		t.Fatal("expected too-small card to be rejected")
	}
	area, thrust, steer, ok := curvePlotArea(rect{Right: 1110, Bottom: 280})
	if !ok || thrust.Top != area.Top+curvePlotLabelH || thrust.Right >= steer.Left {
		t.Fatalf("unexpected layout area=%v thrust=%v steer=%v ok=%v", area, thrust, steer, ok)
	}
}

func TestOverlaySnapshotDiffersOnThrustInputs(t *testing.T) {
	base := overlaySnapshot{Left: 0.2, Right: 0.1}
	moved := base
	moved.Left = 0.5
	if !moved.differs(base) {
		t.Fatal("expected Left change to trigger redraw")
	}
}

func TestLiveCardShouldBeActive(t *testing.T) {
	cases := []struct {
		name      string
		minimized bool
		section   int
		profile   string
		want      bool
	}{
		{"setup hosas", false, idSectionSetup, "HOSAS", true},
		{"setup hosas lowercase", false, idSectionSetup, "hosas", true},
		{"minimized", true, idSectionSetup, "HOSAS", false},
		{"other section", false, idSectionBindings, "HOSAS", false},
		{"hotas profile", false, idSectionSetup, "HOTAS", false},
	}
	for _, tc := range cases {
		if got := liveCardShouldBeActive(tc.minimized, tc.section, tc.profile); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestLiveCardBoxLayoutCentersAndClamps(t *testing.T) {
	layout := liveCardBoxLayout(284)
	if layout.left != 12 {
		t.Fatalf("left = %d, want 12", layout.left)
	}
	want := [][2]int32{{12, 88}, {104, 180}, {196, 272}}
	for i, w := range want {
		b := layout.box(i)
		if b.Left != w[0] || b.Right != w[1] {
			t.Fatalf("box %d = %+v, want %d..%d", i, b, w[0], w[1])
		}
	}
	if narrow := liveCardBoxLayout(100); narrow.left != 0 {
		t.Fatalf("narrow left = %d, want 0", narrow.left)
	}
}

func TestCurvePlotPointHandlesNaN(t *testing.T) {
	plot := rect{Left: 10, Top: 20, Right: 110, Bottom: 120}
	x, y := curvePlotPoint(plot, math.NaN(), math.NaN())
	if x < plot.Left || x > plot.Right || y < plot.Top || y > plot.Bottom {
		t.Fatalf("curvePlotPoint(NaN, NaN) = (%d, %d), outside %+v", x, y, plot)
	}
}
