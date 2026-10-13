//go:build windows

package main

import (
	"math"
	"runtime"
	"strings"
	"unsafe"
)

const (
	curvePlotLabelH = 18
	curveDotSize    = 9
)

// curvePlotArea returns the full plot area (incl. label row) and both plots for the
// Driving Response card; ok=false when the card is too small to draw them.
func curvePlotArea(card rect) (area, thrust, steer rect, ok bool) {
	area = rect{Left: card.Left + 570, Top: card.Top + 36 - curvePlotLabelH, Right: card.Right - 20, Bottom: card.Bottom - 12}
	if area.Right-area.Left < 220 || area.Bottom-area.Top-curvePlotLabelH < 24 {
		return rect{}, rect{}, rect{}, false
	}
	top := area.Top + curvePlotLabelH
	mid := area.Left + (area.Right-area.Left)/2
	thrust = rect{Left: area.Left, Top: top, Right: mid - 10, Bottom: area.Bottom}
	steer = rect{Left: mid + 10, Top: top, Right: area.Right, Bottom: area.Bottom}
	return area, thrust, steer, true
}

// curvePlotPoint maps fractions 0..1 (x right, y up) into plot pixels.
func curvePlotPoint(plot rect, fx, fy float64) (int32, int32) {
	if math.IsNaN(fx) {
		fx = 0
	}
	if math.IsNaN(fy) {
		fy = 0
	}
	fx, fy = clamp(fx, 0, 1), clamp(fy, 0, 1)
	x := plot.Left + int32(math.Round(float64(plot.Right-plot.Left)*fx))
	y := plot.Bottom - int32(math.Round(float64(plot.Bottom-plot.Top)*fy))
	return x, y
}

// thrustCurveSample: x = |L+R| (0..1), y = drive output magnitude (incl. amplification above 100).
func thrustCurveSample(input float64, scale, expo int) (float64, float64) {
	x := clamp(math.Abs(input), 0, 1)
	return x, calculateDrive(x, 0, scale, expo)
}

// steerCurveSample: |L-R| spans 0..2 (counter-stick range), x is plotted as fraction of 2.
func steerCurveSample(input float64, scale, expo1, expo2 int) (float64, float64) {
	d := clamp(math.Abs(input), 0, 2)
	return d / 2, calculateSteering(d, 0, scale, expo1, expo2)
}

func (a *appState) drawThrottleCurve(hdc uintptr, card rect) {
	area, thrust, steer, ok := curvePlotArea(card)
	if !ok {
		return
	}
	width, height := area.Right-area.Left, area.Bottom-area.Top
	memDC, _, _ := procCreateCompatibleDC.Call(hdc)
	if memDC == 0 {
		return
	}
	defer procDeleteDC.Call(memDC)
	bitmap, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return
	}
	defer procDeleteObject.Call(bitmap)
	oldBitmap, _, _ := procSelectObject.Call(memDC, bitmap)
	defer procSelectObject.Call(memDC, oldBitmap)
	if a.font != 0 {
		oldFont, _, _ := procSelectObject.Call(memDC, a.font)
		defer procSelectObject.Call(memDC, oldFont)
	}
	oldPen, _, _ := procSelectObject.Call(memDC, a.borderPen)
	defer procSelectObject.Call(memDC, oldPen)
	oldBrush, _, _ := procSelectObject.Call(memDC, a.cardBrush)
	defer procSelectObject.Call(memDC, oldBrush)

	local := rect{Right: width, Bottom: height}
	procFillRect.Call(memDC, uintptr(unsafe.Pointer(&local)), a.cardBrush)
	shift := func(r rect) rect {
		return rect{Left: r.Left - area.Left, Top: r.Top - area.Top, Right: r.Right - area.Left, Bottom: r.Bottom - area.Top}
	}
	thrust, steer = shift(thrust), shift(steer)

	tScale, tExpo := a.throttleScaleValue(), a.throttleExpoValue()
	sScale, sExpo, sExpo2 := a.steerScaleValue(), a.steerExpoValue(), a.steerExpo2Value()
	snap := a.liveCardLast

	a.drawPlotFrame(memDC, thrust)
	a.drawResponseLine(memDC, thrust, a.accentPen, func(t float64) (float64, float64) { return thrustCurveSample(t, tScale, tExpo) })
	x, y := thrustCurveSample(snap.Left+snap.Right, tScale, tExpo)
	a.drawCurveDot(memDC, thrust, x, y, a.accentPen, a.accentBrush)
	drawNativeText(memDC, "THRUST  |  (L+R) CURVE x MAX", rect{Left: thrust.Left, Top: 0, Right: thrust.Right, Bottom: curvePlotLabelH}, uiMuted, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)

	a.drawPlotFrame(memDC, steer)
	a.drawResponseLine(memDC, steer, a.cyanPen, func(t float64) (float64, float64) { return steerCurveSample(2*t, sScale, sExpo, sExpo2) })
	x, y = steerCurveSample(snap.Left-snap.Right, sScale, sExpo, sExpo2)
	a.drawCurveDot(memDC, steer, x, y, a.cyanPen, a.cyanBrush)
	drawNativeText(memDC, "STEERING  |  (L-R) CURVE x MAX", rect{Left: steer.Left, Top: 0, Right: steer.Right, Bottom: curvePlotLabelH}, uiMuted, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)

	procBitBlt.Call(hdc, uintptr(area.Left), uintptr(area.Top), uintptr(width), uintptr(height), memDC, 0, 0, SRCCOPY)
}

// drawPlotFrame draws the axes plus the linear reference diagonal.
func (a *appState) drawPlotFrame(hdc uintptr, plot rect) {
	oldPen, _, _ := procSelectObject.Call(hdc, a.borderPen)
	procMoveToEx.Call(hdc, uintptr(plot.Left), uintptr(plot.Bottom), 0)
	procLineTo.Call(hdc, uintptr(plot.Right), uintptr(plot.Bottom))
	procMoveToEx.Call(hdc, uintptr(plot.Left), uintptr(plot.Top), 0)
	procLineTo.Call(hdc, uintptr(plot.Left), uintptr(plot.Bottom))
	procSelectObject.Call(hdc, a.mutedPen)
	procMoveToEx.Call(hdc, uintptr(plot.Left), uintptr(plot.Bottom), 0)
	procLineTo.Call(hdc, uintptr(plot.Right), uintptr(plot.Top))
	procSelectObject.Call(hdc, oldPen)
}

// drawResponseLine plots sample(t) for t in 0..1; sample returns plot fractions.
func (a *appState) drawResponseLine(hdc uintptr, plot rect, pen uintptr, sample func(float64) (float64, float64)) {
	if pen == 0 {
		return
	}
	const steps = 64
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	procMoveToEx.Call(hdc, uintptr(plot.Left), uintptr(plot.Bottom), 0)
	for i := 1; i <= steps; i++ {
		fx, fy := sample(float64(i) / steps)
		px, py := curvePlotPoint(plot, fx, fy)
		procLineTo.Call(hdc, uintptr(px), uintptr(py))
	}
	procSelectObject.Call(hdc, oldPen)
}

func (a *appState) drawCurveDot(hdc uintptr, plot rect, fx, fy float64, pen, brush uintptr) {
	cx, cy := curvePlotPoint(plot, fx, fy)
	half := int32(curveDotSize / 2)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	procEllipse.Call(hdc, uintptr(cx-half), uintptr(cy-half), uintptr(cx-half+curveDotSize), uintptr(cy-half+curveDotSize))
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
}

// invalidateCurvePlots repaints only the curve plots of visible Driving Response cards.
func (a *appState) invalidateCurvePlots(profile string) {
	for _, card := range a.cards {
		if !curvePlotVisible(card, a.topNavigationID, profile) {
			continue
		}
		scrolled := rect{
			Left: card.bounds.Left - int32(a.scrollX), Top: card.bounds.Top - int32(a.scrollY),
			Right: card.bounds.Right - int32(a.scrollX), Bottom: card.bounds.Bottom - int32(a.scrollY),
		}
		dirty, _, _, ok := curvePlotArea(scrolled)
		if !ok {
			continue
		}
		procRedrawWindow.Call(a.hwnd, uintptr(unsafe.Pointer(&dirty)), 0, RDW_INVALIDATE|RDW_NOERASE|RDW_NOCHILDREN)
		runtime.KeepAlive(&dirty)
	}
}

func curvePlotVisible(card uiCard, section int, profile string) bool {
	return card.curve && card.section == section && strings.EqualFold(profile, "HOSAS") &&
		(card.profile == "" || strings.EqualFold(card.profile, profile))
}
