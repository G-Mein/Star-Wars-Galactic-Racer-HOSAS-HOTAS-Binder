//go:build windows

package main

import (
	"math"
	"testing"
)

func TestOverlayOriginAnchorsTopRightOfWorkArea(t *testing.T) {
	w, h := overlaySize()
	x, y := overlayOrigin(rect{Left: 1920, Top: 0, Right: 3840, Bottom: 1040}, w, h)
	if x != 3840-w-overlayScreenMargin || y != overlayScreenMargin {
		t.Fatalf("origin = %d,%d", x, y)
	}
}

func TestOverlayOriginClampsToSmallWorkArea(t *testing.T) {
	w, h := overlaySize()
	x, y := overlayOrigin(rect{Left: 100, Top: 50, Right: 100 + w/2, Bottom: 50 + h/2}, w, h)
	if x != 100 || y != 50 {
		t.Fatalf("origin = %d,%d, want 100,50", x, y)
	}
}

func TestOverlayLayoutFitsWindow(t *testing.T) {
	w, h := overlaySize()
	last := overlayBoxRect(2)
	if last.Right+overlayMargin != w {
		t.Fatalf("last box right %d + margin != width %d", last.Right, w)
	}
	if last.Bottom+overlayValueHeight > h {
		t.Fatalf("values exceed height: %d > %d", last.Bottom+overlayValueHeight, h)
	}
	closeBox := overlayCloseRect(w)
	if closeBox.Right > w || closeBox.Top < 0 || closeBox.Bottom > overlayHeaderHeight {
		t.Fatalf("close box outside header: %+v", closeBox)
	}
}

func TestOverlayDotRectMapsAxes(t *testing.T) {
	box := rect{Left: 0, Top: 0, Right: 96, Bottom: 96}
	center := overlayDotRect(box, 0, 0)
	if center.Left != 42 || center.Top != 42 {
		t.Fatalf("center = %+v", center)
	}
	fwdRight := overlayDotRect(box, 1, 1)
	if fwdRight.Right != box.Right || fwdRight.Top != box.Top {
		t.Fatalf("forward/right = %+v", fwdRight)
	}
	backLeft := overlayDotRect(box, -5, -5)
	if backLeft.Left != box.Left || backLeft.Bottom != box.Bottom {
		t.Fatalf("clamped back/left = %+v", backLeft)
	}
}

func TestOverlaySnapshotDiffers(t *testing.T) {
	base := overlaySnapshot{LeftY: 0.5, Mode: "HOSAS  MAX 100%"}
	if base.differs(overlaySnapshot{LeftY: 0.503, Mode: base.Mode}) {
		t.Fatal("sub-epsilon change must not redraw")
	}
	if !base.differs(overlaySnapshot{LeftY: 0.51, Mode: base.Mode}) {
		t.Fatal("visible change must redraw")
	}
	if !base.differs(overlaySnapshot{LeftY: 0.5, Mode: "HOTAS"}) {
		t.Fatal("mode change must redraw")
	}
}

func TestPointInRectExcludesRightBottomEdge(t *testing.T) {
	r := rect{Left: 10, Top: 10, Right: 20, Bottom: 20}
	if !pointInRect(r, 10, 10) || pointInRect(r, 20, 15) || pointInRect(r, 15, 20) {
		t.Fatal("pointInRect edge semantics wrong")
	}
}

func TestOverlayOriginOnNegativeMonitor(t *testing.T) {
	w, h := overlaySize()
	x, y := overlayOrigin(rect{Left: -1920, Top: -200, Right: 0, Bottom: 880}, w, h)
	if x != -w-overlayScreenMargin || y != -200+overlayScreenMargin {
		t.Fatalf("origin = %d,%d", x, y)
	}
}

func TestOverlayDotRectNaNCenters(t *testing.T) {
	box := rect{Left: 0, Top: 0, Right: 96, Bottom: 96}
	if got := overlayDotRect(box, math.NaN(), math.NaN()); got != overlayDotRect(box, 0, 0) {
		t.Fatalf("NaN dot = %+v", got)
	}
}

func TestOverlaySnapshotDiffersEveryAxis(t *testing.T) {
	base := overlaySnapshot{LeftX: 0.1, LeftY: 0.2, RightX: 0.3, RightY: 0.4, Steer: 0.5, Drive: 0.6, Mode: "HOSAS"}
	fields := []*float64{&base.LeftX, &base.LeftY, &base.RightX, &base.RightY, &base.Steer, &base.Drive}
	for i := range fields {
		changed := base
		moved := []*float64{&changed.LeftX, &changed.LeftY, &changed.RightX, &changed.RightY, &changed.Steer, &changed.Drive}
		*moved[i] -= 0.006
		if !base.differs(changed) {
			t.Fatalf("axis %d change not detected", i)
		}
	}
}

func TestPointInRectOutsideLeftTop(t *testing.T) {
	r := rect{Left: -20, Top: -20, Right: -10, Bottom: -10}
	if !pointInRect(r, -15, -15) || pointInRect(r, -21, -15) || pointInRect(r, -15, -21) {
		t.Fatal("pointInRect left/top semantics wrong")
	}
}

func TestOverlaySnapshotDiffersNaNTransition(t *testing.T) {
	nan := overlaySnapshot{LeftX: math.NaN()}
	if !nan.differs(overlaySnapshot{LeftX: 1}) || !(overlaySnapshot{}).differs(nan) {
		t.Fatal("NaN transition must redraw")
	}
	if nan.differs(overlaySnapshot{LeftX: math.NaN()}) {
		t.Fatal("NaN to NaN must not redraw")
	}
}
