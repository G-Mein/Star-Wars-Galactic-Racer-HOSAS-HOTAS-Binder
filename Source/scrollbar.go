//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	scrollBarClassName  = "GalacticRacerHOSASScrollBarClass"
	scrollBarMinThumb   = 28
	scrollBarTrackInset = 3
	scrollBarLineStep   = 60
	WM_MOUSEMOVE        = 0x0200
	WM_CAPTURECHANGED   = 0x0215
	scrollBarThumbColor = 0x00695A46 // RGB(70, 90, 105) as COLORREF (BGR)
	scrollBarTrackColor = 0x00180E0E
	scrollBarNullPen    = 8
)

var (
	procSetCapture        = user32.NewProc("SetCapture")
	procReleaseCapture    = user32.NewProc("ReleaseCapture")
	procSetWindowRgn      = user32.NewProc("SetWindowRgn")
	procCreateRectRgn     = gdi32.NewProc("CreateRectRgn")
	procIntersectClipRect = gdi32.NewProc("IntersectClipRect")
	procSaveDC            = gdi32.NewProc("SaveDC")
	procRestoreDC         = gdi32.NewProc("RestoreDC")
)

var scrollBarWndProcCallback = syscall.NewCallback(scrollBarWndProc)

// customScrollBar is a self-drawn scrollbar child. Native SCROLLBAR controls ignore
// the dark theme, so the bar paints its own track/thumb and drives app scrolling.
type customScrollBar struct {
	hwnd       uintptr
	horizontal bool
	min        int
	max        int
	page       int
	pos        int
	dragging   bool
	dragOffset int
}

func scrollBarTrackLength(track rect, horizontal bool) int {
	if horizontal {
		return int(track.Right - track.Left)
	}
	return int(track.Bottom - track.Top)
}

// scrollThumbBounds returns the thumb rectangle inside track for the given range.
func scrollThumbBounds(track rect, horizontal bool, minPos, maxPos, page, pos int) rect {
	length := scrollBarTrackLength(track, horizontal)
	if length <= 0 {
		return track
	}
	if page < 1 {
		page = 1
	}
	span := maxPos - minPos
	if span < 0 {
		span = 0
	}
	thumb := length * page / (span + page)
	if thumb < scrollBarMinThumb {
		thumb = scrollBarMinThumb
	}
	if thumb > length {
		thumb = length
	}
	offset := 0
	if span > 0 {
		if pos < minPos {
			pos = minPos
		}
		if pos > maxPos {
			pos = maxPos
		}
		offset = (length - thumb) * (pos - minPos) / span
	}
	if horizontal {
		return rect{Left: track.Left + int32(offset), Top: track.Top, Right: track.Left + int32(offset+thumb), Bottom: track.Bottom}
	}
	return rect{Left: track.Left, Top: track.Top + int32(offset), Right: track.Right, Bottom: track.Top + int32(offset+thumb)}
}

// scrollPosFromThumbOffset maps a thumb start offset inside the track back to a scroll position.
func scrollPosFromThumbOffset(offset, length, thumb, minPos, maxPos int) int {
	travel := length - thumb
	if travel <= 0 || maxPos <= minPos {
		return minPos
	}
	if offset < 0 {
		offset = 0
	}
	if offset > travel {
		offset = travel
	}
	return minPos + (offset*(maxPos-minPos)+travel/2)/travel
}

func clampScrollPos(pos, minPos, maxPos int) int {
	if maxPos < minPos {
		maxPos = minPos
	}
	if pos < minPos {
		return minPos
	}
	if pos > maxPos {
		return maxPos
	}
	return pos
}

func (a *appState) ensureScrollBarClass() bool {
	if a.scrollBarClassReady {
		return true
	}
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(IDC_ARROW))
	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		LpfnWndProc:   scrollBarWndProcCallback,
		HInstance:     hInstance,
		HCursor:       cursor,
		LpszClassName: utf16ptr(scrollBarClassName),
	}
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return false
	}
	a.scrollBarClassReady = true
	return true
}

func (a *appState) createCustomScrollBar(horizontal bool) *customScrollBar {
	if !a.ensureScrollBarClass() {
		return nil
	}
	hwnd := a.createFixedControl(scrollBarClassName, "", WS_CHILD, 0, 0, scrollBarWidth, scrollBarWidth, 0)
	if hwnd == 0 {
		return nil
	}
	bar := &customScrollBar{hwnd: hwnd, horizontal: horizontal}
	if a.scrollBars == nil {
		a.scrollBars = make(map[uintptr]*customScrollBar)
	}
	a.scrollBars[hwnd] = bar
	return bar
}

func (a *appState) scrollBarTrack(bar *customScrollBar) rect {
	var rc rect
	procGetClientRect.Call(bar.hwnd, uintptr(unsafe.Pointer(&rc)))
	return rect{Left: rc.Left + scrollBarTrackInset, Top: rc.Top + scrollBarTrackInset, Right: rc.Right - scrollBarTrackInset, Bottom: rc.Bottom - scrollBarTrackInset}
}

func (a *appState) setScrollBarState(bar *customScrollBar, minPos, maxPos, page, pos int) {
	if bar == nil {
		return
	}
	bar.min, bar.max, bar.page, bar.pos = minPos, maxPos, page, pos
	procRedrawWindow.Call(bar.hwnd, 0, 0, RDW_INVALIDATE|RDW_NOERASE)
}

// scrollTo moves the content to pos on the given axis and keeps the bar in sync.
func (a *appState) scrollTo(horizontal bool, pos int) {
	bar := a.verticalBar
	current := &a.scrollY
	if horizontal {
		bar = a.horizontalBar
		current = &a.scrollX
	}
	minPos, maxPos, _ := a.scrollRange(horizontal)
	pos = clampScrollPos(pos, minPos, maxPos)
	if pos == *current {
		return
	}
	*current = pos
	if bar != nil {
		bar.pos = pos
		procRedrawWindow.Call(bar.hwnd, 0, 0, RDW_INVALIDATE|RDW_NOERASE|RDW_UPDATENOW)
	}
	a.repositionScrolledChildren()
}

func scrollBarWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	bar := app.scrollBars[hwnd]
	if bar == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
		return r
	}
	switch message {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps paintStruct
		if hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps))); hdc != 0 {
			app.paintScrollBar(bar, hdc)
			procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		}
		return 0
	case WM_LBUTTONDOWN:
		app.closeCombo(false)
		app.scrollBarMouseDown(bar, signedLowWord(lParam), signedHighWord(lParam))
		return 0
	case WM_MOUSEMOVE:
		if bar.dragging {
			app.scrollBarDrag(bar, signedLowWord(lParam), signedHighWord(lParam))
		}
		return 0
	case WM_LBUTTONUP:
		if bar.dragging {
			bar.dragging = false
			procReleaseCapture.Call()
			procRedrawWindow.Call(hwnd, 0, 0, RDW_INVALIDATE|RDW_NOERASE)
		}
		return 0
	case WM_CAPTURECHANGED:
		if bar.dragging {
			bar.dragging = false
			procRedrawWindow.Call(hwnd, 0, 0, RDW_INVALIDATE|RDW_NOERASE)
		}
		return 0
	case WM_MOUSEWHEEL:
		procSendMessageW.Call(app.hwnd, WM_MOUSEWHEEL, wParam, lParam)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func (a *appState) scrollBarMouseDown(bar *customScrollBar, x, y int32) {
	track := a.scrollBarTrack(bar)
	thumb := scrollThumbBounds(track, bar.horizontal, bar.min, bar.max, bar.page, bar.pos)
	coordinate, thumbStart, thumbEnd := y, thumb.Top, thumb.Bottom
	if bar.horizontal {
		coordinate, thumbStart, thumbEnd = x, thumb.Left, thumb.Right
	}
	if coordinate >= thumbStart && coordinate < thumbEnd {
		bar.dragging = true
		bar.dragOffset = int(coordinate - thumbStart)
		procSetCapture.Call(bar.hwnd)
		procRedrawWindow.Call(bar.hwnd, 0, 0, RDW_INVALIDATE|RDW_NOERASE)
		return
	}
	page := bar.page - 40
	if page < 40 {
		page = 40
	}
	if coordinate < thumbStart {
		a.scrollTo(bar.horizontal, bar.pos-page)
	} else {
		a.scrollTo(bar.horizontal, bar.pos+page)
	}
}

func (a *appState) scrollBarDrag(bar *customScrollBar, x, y int32) {
	track := a.scrollBarTrack(bar)
	thumb := scrollThumbBounds(track, bar.horizontal, bar.min, bar.max, bar.page, bar.pos)
	length := scrollBarTrackLength(track, bar.horizontal)
	thumbLength := scrollBarTrackLength(thumb, bar.horizontal)
	offset := int(y-track.Top) - bar.dragOffset
	if bar.horizontal {
		offset = int(x-track.Left) - bar.dragOffset
	}
	a.scrollTo(bar.horizontal, scrollPosFromThumbOffset(offset, length, thumbLength, bar.min, bar.max))
}

func (a *appState) paintScrollBar(bar *customScrollBar, hdc uintptr) {
	var rc rect
	procGetClientRect.Call(bar.hwnd, uintptr(unsafe.Pointer(&rc)))
	background := a.bgBrush
	if background == 0 {
		return
	}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), background)
	track := a.scrollBarTrack(bar)
	trackBrush, _, _ := procCreateSolidBrush.Call(scrollBarTrackColor)
	thumbColor := uintptr(scrollBarThumbColor)
	if bar.dragging {
		thumbColor = uiAccent
	}
	thumbBrush, _, _ := procCreateSolidBrush.Call(thumbColor)
	nullPen, _, _ := procGetStockObject.Call(scrollBarNullPen)
	oldPen, _, _ := procSelectObject.Call(hdc, nullPen)
	oldBrush, _, _ := procSelectObject.Call(hdc, trackBrush)
	radius := uintptr(scrollBarWidth - 2*scrollBarTrackInset)
	procRoundRect.Call(hdc, uintptr(track.Left), uintptr(track.Top), uintptr(track.Right+1), uintptr(track.Bottom+1), radius, radius)
	thumb := scrollThumbBounds(track, bar.horizontal, bar.min, bar.max, bar.page, bar.pos)
	procSelectObject.Call(hdc, thumbBrush)
	procRoundRect.Call(hdc, uintptr(thumb.Left), uintptr(thumb.Top), uintptr(thumb.Right+1), uintptr(thumb.Bottom+1), radius, radius)
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(trackBrush)
	procDeleteObject.Call(thumbBrush)
}

// contentViewport is the client area where scrolled content may appear: right of the
// sidebar, below the fixed title/profile bar, above the status bar and left/above visible scrollbars.
func contentViewport(client rect, verticalVisible, horizontalVisible bool) rect {
	view := rect{Left: sidebarWidth, Top: fixedProfileBottom, Right: client.Right, Bottom: statusBarBounds(client).Top}
	if verticalVisible {
		view.Right = verticalScrollBarBounds(client).Left
	}
	if horizontalVisible {
		view.Bottom = horizontalScrollBarBounds(client, verticalVisible).Top
	}
	if view.Right < view.Left {
		view.Right = view.Left
	}
	if view.Bottom < view.Top {
		view.Bottom = view.Top
	}
	return view
}

// statusBarBounds is the full-width copyright status bar at the bottom of the client area.
// It covers the bottom resize border; hit-testing still reports HTBOTTOM there.
func statusBarBounds(client rect) rect {
	top := client.Bottom - statusBarHeight
	if top < client.Top {
		top = client.Top
	}
	bottom := client.Bottom
	if bottom < top {
		bottom = top
	}
	return rect{Left: client.Left, Top: top, Right: client.Right, Bottom: bottom}
}

func horizontalScrollBarBounds(client rect, verticalVisible bool) rect {
	bottom := statusBarBounds(client).Top
	top := bottom - scrollBarWidth
	if top < client.Top {
		top = client.Top
	}
	if bottom < top {
		bottom = top
	}
	left := int32(sidebarWidth)
	right := client.Right - windowResizeBorder
	if verticalVisible {
		right -= scrollBarWidth
	}
	if right < left {
		right = left
	}
	return rect{Left: left, Top: top, Right: right, Bottom: bottom}
}

// childClipRect returns the visible part of child (parent coordinates) inside view,
// expressed in child-window coordinates. full=true means no clipping is needed.
func childClipRect(child, view rect) (clip rect, full bool) {
	visible := rect{
		Left:   max32(child.Left, view.Left),
		Top:    max32(child.Top, view.Top),
		Right:  min32(child.Right, view.Right),
		Bottom: min32(child.Bottom, view.Bottom),
	}
	if visible.Left == child.Left && visible.Top == child.Top && visible.Right == child.Right && visible.Bottom == child.Bottom {
		return rect{}, true
	}
	if visible.Right <= visible.Left || visible.Bottom <= visible.Top {
		return rect{}, false
	}
	return rect{
		Left:   visible.Left - child.Left,
		Top:    visible.Top - child.Top,
		Right:  visible.Right - child.Left,
		Bottom: visible.Bottom - child.Top,
	}, false
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// applyChildClip keeps scrolled child controls from painting over the fixed chrome.
// Siblings of the same parent cannot clip each other reliably, so each child gets a
// window region limited to the content viewport.
// The region is set without redraw: the child still sits at its old position here, so a
// synchronous repaint would use the new clip at the old place. Callers move the child and
// repaint afterwards. Returns true when the clip changed.
func (a *appState) applyChildClip(h uintptr, child, view rect) bool {
	if a.childClips == nil {
		a.childClips = make(map[uintptr]rect)
	}
	clip, full := childClipRect(child, view)
	previous, clipped := a.childClips[h]
	if full {
		if !clipped {
			return false
		}
		if ok, _, _ := procSetWindowRgn.Call(h, 0, 0); ok == 0 {
			return false
		}
		delete(a.childClips, h)
		return true
	}
	if clipped && previous == clip {
		return false
	}
	region, _, _ := procCreateRectRgn.Call(uintptr(clip.Left), uintptr(clip.Top), uintptr(clip.Right), uintptr(clip.Bottom))
	if region == 0 {
		return false
	}
	// On success the system owns the region handle.
	if ok, _, _ := procSetWindowRgn.Call(h, region, 0); ok == 0 {
		procDeleteObject.Call(region)
		return false
	}
	a.childClips[h] = clip
	return true
}

// clipControlDC mirrors the window-region clip into a control's paint DC.
// BUTTON/STATIC classes use CS_PARENTDC, whose DC ignores the child window region.
func (a *appState) clipControlDC(hdc, h uintptr) {
	clip, clipped := a.childClips[h]
	if !clipped || hdc == 0 {
		return
	}
	procIntersectClipRect.Call(hdc, uintptr(clip.Left), uintptr(clip.Top), uintptr(clip.Right), uintptr(clip.Bottom))
}

func (a *appState) currentContentViewport() rect {
	var rc rect
	procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&rc)))
	verticalVisible := a.verticalBar != nil && a.verticalBar.max > a.verticalBar.min
	horizontalVisible := a.horizontalBar != nil && a.horizontalBar.max > a.horizontalBar.min
	return contentViewport(rc, verticalVisible, horizontalVisible)
}
