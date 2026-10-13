//go:build windows

package main

import (
	"runtime"
	"strings"
	"unsafe"
)

const (
	liveCardTitle    = "LIVE OUTPUT"
	liveCardTimerID  = 3
	liveCardInsetX   = 8
	liveCardInsetTop = 32
	liveCardInsetBot = 8
	liveCardModeH    = 20
)

var procIsIconic = user32.NewProc("IsIconic")

// liveCardLayout is a compact variant of the overlay boxes that fits the
// 300px wide card next to the HOSAS stick cards.
var liveCardLayout = liveBoxLayout{size: 76, gap: 16, label: 20, value: 36}

func liveCardContentRect(card uiCard, scrollX, scrollY int) rect {
	return rect{
		Left:   card.bounds.Left + liveCardInsetX - int32(scrollX),
		Top:    card.bounds.Top + liveCardInsetTop - int32(scrollY),
		Right:  card.bounds.Right - liveCardInsetX - int32(scrollX),
		Bottom: card.bounds.Bottom - liveCardInsetBot - int32(scrollY),
	}
}

func liveCardVisible(card uiCard, section int, profile string) bool {
	return card.live && card.section == section &&
		(card.profile == "" || strings.EqualFold(card.profile, profile))
}

// liveCardBoxLayout centers the boxes horizontally inside a content area of the given width.
func liveCardBoxLayout(contentWidth int32) liveBoxLayout {
	layout := liveCardLayout
	layout.left = (contentWidth - layout.width()) / 2
	if layout.left < 0 {
		layout.left = 0
	}
	return layout
}

// liveCardShouldBeActive gates the live card/curve refresh and extra axis polling.
func liveCardShouldBeActive(minimized bool, section int, profile string) bool {
	return !minimized && section == idSectionSetup && strings.EqualFold(profile, "HOSAS")
}

// refreshLiveCard runs on its own timer and only repaints the card content
// when the Steuerung page is visible and a value changed visibly.
func (a *appState) refreshLiveCard() {
	iconic, _, _ := procIsIconic.Call(a.hwnd)
	profile := a.selectedControlMode()
	visible := liveCardShouldBeActive(iconic != 0, a.topNavigationID, profile)
	a.liveCardActive.Store(visible)
	if !visible {
		return
	}
	snap := a.overlaySnapshotNow()
	if !snap.differs(a.liveCardLast) {
		return
	}
	a.liveCardLast = snap
	for _, card := range a.cards {
		if !liveCardVisible(card, a.topNavigationID, profile) {
			continue
		}
		dirty := liveCardContentRect(card, a.scrollX, a.scrollY)
		procRedrawWindow.Call(a.hwnd, uintptr(unsafe.Pointer(&dirty)), 0, RDW_INVALIDATE|RDW_NOERASE|RDW_NOCHILDREN)
		runtime.KeepAlive(&dirty)
	}
	a.invalidateCurvePlots(profile)
}

// drawLiveCardContent renders the live boxes double-buffered so stick
// movement does not flicker the main window.
func (a *appState) drawLiveCardContent(hdc uintptr, card uiCard) {
	area := liveCardContentRect(card, a.scrollX, a.scrollY)
	width, height := area.Right-area.Left, area.Bottom-area.Top
	if width <= 0 || height <= 0 {
		return
	}
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

	local := rect{Right: width, Bottom: height}
	procFillRect.Call(memDC, uintptr(unsafe.Pointer(&local)), a.cardBrush)
	oldPen, _, _ := procSelectObject.Call(memDC, a.borderPen)
	oldBrush, _, _ := procSelectObject.Call(memDC, a.fieldBrush)
	layout := liveCardBoxLayout(width)
	snap := a.liveCardLast
	a.drawLiveBoxes(memDC, layout, snap)
	procSelectObject.Call(memDC, oldBrush)
	procSelectObject.Call(memDC, oldPen)
	modeTop := layout.top + layout.height() + 2
	mode := rect{Left: 0, Top: modeTop, Right: width, Bottom: modeTop + liveCardModeH}
	drawNativeText(memDC, snap.Mode, mode, uiAccent, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)

	procBitBlt.Call(hdc, uintptr(area.Left), uintptr(area.Top), uintptr(width), uintptr(height), memDC, 0, 0, SRCCOPY)
}
