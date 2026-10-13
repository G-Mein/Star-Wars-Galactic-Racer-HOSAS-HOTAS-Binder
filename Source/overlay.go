//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

const (
	overlayClassName     = "GalacticRacerHOSASOverlayClass"
	overlayTimerID       = 2
	overlayRefreshMs     = 33
	overlayTopmostTicks  = 30
	overlayAlpha         = 230
	overlayMargin        = 16
	overlayScreenMargin  = 16
	overlayHeaderHeight  = 30
	overlayLabelHeight   = 20
	overlayBoxSize       = 96
	overlayBoxGap        = 28
	overlayValueHeight   = 36
	overlayCloseSize     = 22
	overlayChangeEpsilon = 0.005

	WS_EX_LAYERED            = 0x00080000
	LWA_ALPHA                = 0x00000002
	MONITOR_DEFAULTTONEAREST = 0x00000002
	SRCCOPY                  = 0x00CC0020
	WM_MOUSEACTIVATE         = 0x0021
	WM_LBUTTONUP             = 0x0202
	MA_NOACTIVATE            = 3
)

const hwndTopmost = ^uintptr(0)

var (
	procCreateCompatibleDC         = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap     = gdi32.NewProc("CreateCompatibleBitmap")
	procBitBlt                     = gdi32.NewProc("BitBlt")
	procDeleteDC                   = gdi32.NewProc("DeleteDC")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procMonitorFromWindow          = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW            = user32.NewProc("GetMonitorInfoW")
)

var overlayWndProcCallback = syscall.NewCallback(overlayWndProc)

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

// overlaySnapshot holds exactly the values the overlay paints, so redraws only
// happen when something visible changed.
type overlaySnapshot struct {
	LeftX, LeftY   float64
	RightX, RightY float64
	Steer, Drive   float64
	Left, Right    float64
	Mode           string
}

func (s overlaySnapshot) differs(o overlaySnapshot) bool {
	if s.Mode != o.Mode {
		return true
	}
	a := [...]float64{s.LeftX, s.LeftY, s.RightX, s.RightY, s.Steer, s.Drive, s.Left, s.Right}
	b := [...]float64{o.LeftX, o.LeftY, o.RightX, o.RightY, o.Steer, o.Drive, o.Left, o.Right}
	for i := range a {
		if math.IsNaN(a[i]) != math.IsNaN(b[i]) || math.Abs(a[i]-b[i]) >= overlayChangeEpsilon {
			return true
		}
	}
	return false
}

func overlaySize() (int32, int32) {
	w := 2*overlayMargin + 3*overlayBoxSize + 2*overlayBoxGap
	h := overlayHeaderHeight + overlayLabelHeight + overlayBoxSize + overlayValueHeight + overlayMargin/2
	return int32(w), int32(h)
}

// overlayOrigin anchors the overlay to the top-right corner of a monitor work area.
func overlayOrigin(work rect, w, h int32) (int32, int32) {
	x := work.Right - w - overlayScreenMargin
	if x < work.Left {
		x = work.Left
	}
	y := work.Top + overlayScreenMargin
	if y+h > work.Bottom {
		y = work.Bottom - h
	}
	if y < work.Top {
		y = work.Top
	}
	return x, y
}

// liveBoxLayout positions the three live boxes (left, right, drive/steer)
// shared by the overlay window and the Steuerung live card.
type liveBoxLayout struct {
	left, top, size, gap, label, value int32
}

var overlayLiveLayout = liveBoxLayout{
	left: overlayMargin, top: overlayHeaderHeight, size: overlayBoxSize,
	gap: overlayBoxGap, label: overlayLabelHeight, value: overlayValueHeight,
}

func (l liveBoxLayout) box(index int) rect {
	left := l.left + int32(index)*(l.size+l.gap)
	top := l.top + l.label
	return rect{Left: left, Top: top, Right: left + l.size, Bottom: top + l.size}
}

func (l liveBoxLayout) width() int32 {
	return 3*l.size + 2*l.gap
}

func (l liveBoxLayout) height() int32 {
	return l.label + l.size + l.value
}

func overlayBoxRect(index int) rect {
	return overlayLiveLayout.box(index)
}

func overlayCloseRect(width int32) rect {
	top := int32((overlayHeaderHeight - overlayCloseSize) / 2)
	right := width - overlayMargin/2
	return rect{Left: right - overlayCloseSize, Top: top, Right: right, Bottom: top + overlayCloseSize}
}

// overlayDotRect maps a -1..1 x/y pair (y positive = forward/up) into a dot inside box.
func overlayDotRect(box rect, x, y float64) rect {
	dx, dy := stickDotOffset(int(box.Right-box.Left), x, y)
	cx := float64(box.Left+box.Right)/2.0 + dx
	cy := float64(box.Top+box.Bottom)/2.0 + dy
	left := int32(math.Round(cx - stickDotSize/2.0))
	top := int32(math.Round(cy - stickDotSize/2.0))
	return rect{Left: left, Top: top, Right: left + stickDotSize, Bottom: top + stickDotSize}
}

func pointInRect(r rect, x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *appState) overlaySnapshotNow() overlaySnapshot {
	a.monitorMu.RLock()
	m := a.monitor
	a.monitorMu.RUnlock()
	a.cfgMu.RLock()
	c := a.cfg
	a.cfgMu.RUnlock()
	mode := fmt.Sprintf("HOSAS T%d/%d S%d/%d/%d", normalizeScalePercent(c.ThrottleScale), normalizeExpoPercent(c.ThrottleExpo), normalizeSteerScalePercent(c.SteerScale), normalizeExpoPercent(c.SteerExpo), normalizeExpoPercent(c.SteerExpo2))
	if strings.EqualFold(normalizeControlMode(c.ControlMode), "HOTAS") {
		mode = "HOTAS"
	}
	return overlaySnapshot{
		LeftX: m.LeftX, LeftY: m.LeftY,
		RightX: m.RightX, RightY: m.RightY,
		Steer: m.Steer, Drive: m.Drive,
		Left: m.Left, Right: m.Right,
		Mode: mode,
	}
}

func (a *appState) ensureOverlayClass() bool {
	if a.overlayClassReady {
		return true
	}
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(IDC_ARROW))
	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		LpfnWndProc:   overlayWndProcCallback,
		HInstance:     hInstance,
		HCursor:       cursor,
		LpszClassName: utf16ptr(overlayClassName),
	}
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return false
	}
	a.overlayClassReady = true
	return true
}

func (a *appState) showOverlay() {
	if a.overlayHwnd != 0 {
		return
	}
	if !a.ensureOverlayClass() {
		a.postStatus("Overlay unavailable: window class could not be registered.")
		return
	}
	w, h := overlaySize()
	work := rect{Right: 1920, Bottom: 1080}
	if monitor, _, _ := procMonitorFromWindow.Call(a.hwnd, MONITOR_DEFAULTTONEAREST); monitor != 0 {
		info := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
		if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
			work = info.RcWork
		}
	}
	x, y := overlayOrigin(work, w, h)
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	// No owner window: owned windows are hidden while their owner is minimized.
	hwnd, _, e := procCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE|WS_EX_LAYERED,
		uintptr(unsafe.Pointer(utf16ptr(overlayClassName))),
		uintptr(unsafe.Pointer(utf16ptr("Galactic Racer Live Overlay"))),
		WS_POPUP,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		a.postStatus(fmt.Sprintf("Overlay unavailable: %v", e))
		return
	}
	a.overlayHwnd = hwnd
	a.overlayLast = a.overlaySnapshotNow()
	a.overlayTicks = 0
	a.overlayVisible.Store(true)
	if t, _, e := procSetTimer.Call(a.hwnd, overlayTimerID, overlayRefreshMs, 0); t == 0 {
		a.hideOverlay(false)
		a.postStatus(fmt.Sprintf("Overlay unavailable: refresh timer failed: %v", e))
		return
	}
	procSetLayeredWindowAttributes.Call(hwnd, 0, overlayAlpha, LWA_ALPHA)
	procShowWindow.Call(hwnd, SW_SHOWNOACTIVATE)
	procShowWindow.Call(a.hwnd, SW_MINIMIZE)
}

func (a *appState) hideOverlay(restoreMain bool) {
	if a.overlayHwnd == 0 {
		return
	}
	procKillTimer.Call(a.hwnd, overlayTimerID)
	a.overlayVisible.Store(false)
	hwnd := a.overlayHwnd
	a.overlayHwnd = 0
	procDestroyWindow.Call(hwnd)
	if restoreMain {
		procShowWindow.Call(a.hwnd, SW_RESTORE)
		procBringWindowToTop.Call(a.hwnd)
	}
}

func (a *appState) refreshOverlay() {
	if a.overlayHwnd == 0 {
		return
	}
	a.overlayTicks++
	if a.overlayTicks >= overlayTopmostTicks {
		// Games may raise their own topmost windows; periodically reassert ours.
		a.overlayTicks = 0
		procSetWindowPos.Call(a.overlayHwnd, hwndTopmost, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
	}
	snap := a.overlaySnapshotNow()
	if !snap.differs(a.overlayLast) {
		return
	}
	a.overlayLast = snap
	procRedrawWindow.Call(a.overlayHwnd, 0, 0, RDW_INVALIDATE|RDW_NOERASE|RDW_UPDATENOW)
}

func overlayWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_MOUSEACTIVATE:
		return MA_NOACTIVATE
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps paintStruct
		if hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps))); hdc != 0 {
			app.paintOverlay(hwnd, hdc)
			procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		}
		return 0
	case WM_LBUTTONUP:
		w, _ := overlaySize()
		if pointInRect(overlayCloseRect(w), signedLowWord(lParam), signedHighWord(lParam)) {
			app.hideOverlay(true)
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func (a *appState) paintOverlay(hwnd, hdc uintptr) {
	if !a.uiColorsReady {
		return
	}
	var bounds rect
	if ok, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds))); ok == 0 {
		return
	}
	width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
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

	snap := a.overlayLast
	procFillRect.Call(memDC, uintptr(unsafe.Pointer(&bounds)), a.cardBrush)
	header := rect{Right: width, Bottom: overlayHeaderHeight}
	procFillRect.Call(memDC, uintptr(unsafe.Pointer(&header)), a.sidebarBrush)

	oldPen, _, _ := procSelectObject.Call(memDC, a.borderPen)
	oldBrush, _, _ := procSelectObject.Call(memDC, procGetStockObjectNullBrush())
	procRoundRect.Call(memDC, 0, 0, uintptr(width), uintptr(height), 4, 4)
	procMoveToEx.Call(memDC, 0, overlayHeaderHeight-1, 0)
	procLineTo.Call(memDC, uintptr(width), overlayHeaderHeight-1)

	drawNativeText(memDC, "LIVE MONITOR", rect{Left: overlayMargin, Top: 0, Right: width / 2, Bottom: overlayHeaderHeight},
		uiText, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	closeBox := overlayCloseRect(width)
	drawNativeText(memDC, snap.Mode, rect{Left: width / 2, Top: 0, Right: closeBox.Left - 8, Bottom: overlayHeaderHeight},
		uiAccent, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	procSelectObject.Call(memDC, a.closeBrush)
	procRoundRect.Call(memDC, uintptr(closeBox.Left), uintptr(closeBox.Top), uintptr(closeBox.Right), uintptr(closeBox.Bottom), 6, 6)
	drawNativeText(memDC, "X", closeBox, uiText, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	a.drawLiveBoxes(memDC, overlayLiveLayout, snap)
	procSelectObject.Call(memDC, oldBrush)
	procSelectObject.Call(memDC, oldPen)

	procBitBlt.Call(hdc, 0, 0, uintptr(width), uintptr(height), memDC, 0, 0, SRCCOPY)
}

// drawLiveBoxes draws labels, crosshair boxes, dots and values; it changes the
// selected pen/brush, so callers must restore their own selection.
func (a *appState) drawLiveBoxes(hdc uintptr, layout liveBoxLayout, snap overlaySnapshot) {
	panels := []struct {
		title string
		x, y  float64
		value string
	}{
		{"LEFT STICK", snap.LeftX, snap.LeftY, fmt.Sprintf("Side %+0.2f\r\nFwd  %+0.2f", snap.LeftX, snap.LeftY)},
		{"RIGHT STICK", snap.RightX, snap.RightY, fmt.Sprintf("Side %+0.2f\r\nFwd  %+0.2f", snap.RightX, snap.RightY)},
		{"DRIVE / STEER", snap.Steer, snap.Drive, fmt.Sprintf("Drive %+0.2f\r\nSteer %+0.2f", snap.Drive, snap.Steer)},
	}
	pad := layout.gap / 2
	for i, p := range panels {
		box := layout.box(i)
		label := rect{Left: box.Left - pad, Top: layout.top, Right: box.Right + pad, Bottom: box.Top}
		drawNativeText(hdc, p.title, label, uiMuted, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)

		procSelectObject.Call(hdc, a.borderPen)
		procSelectObject.Call(hdc, a.fieldBrush)
		procRoundRect.Call(hdc, uintptr(box.Left), uintptr(box.Top), uintptr(box.Right), uintptr(box.Bottom), 6, 6)
		midX, midY := (box.Left+box.Right)/2, (box.Top+box.Bottom)/2
		procMoveToEx.Call(hdc, uintptr(midX), uintptr(box.Top+1), 0)
		procLineTo.Call(hdc, uintptr(midX), uintptr(box.Bottom-1))
		procMoveToEx.Call(hdc, uintptr(box.Left+1), uintptr(midY), 0)
		procLineTo.Call(hdc, uintptr(box.Right-1), uintptr(midY))

		dot := overlayDotRect(box, p.x, p.y)
		procSelectObject.Call(hdc, a.accentPen)
		procSelectObject.Call(hdc, a.accentBrush)
		procEllipse.Call(hdc, uintptr(dot.Left), uintptr(dot.Top), uintptr(dot.Right), uintptr(dot.Bottom))

		value := rect{Left: box.Left - pad, Top: box.Bottom + 2, Right: box.Right + pad, Bottom: box.Bottom + layout.value}
		drawNativeText(hdc, p.value, value, uiText, DT_CENTER|DT_NOPREFIX)
	}
}

func procGetStockObjectNullBrush() uintptr {
	const nullBrush = 5
	brush, _, _ := procGetStockObject.Call(nullBrush)
	return brush
}
