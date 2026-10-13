//go:build windows

package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"
	"unsafe"
)

var (
	uiBackground = uiRGB(14, 17, 24)
	uiSidebar    = uiRGB(10, 13, 19)
	uiCardColor  = uiRGB(23, 28, 40)
	uiField      = uiRGB(14, 17, 24)
	uiBorder     = uiRGB(38, 45, 63)
	uiText       = uiRGB(230, 233, 242)
	uiMuted      = uiRGB(139, 148, 171)
	uiAccent     = uiRGB(255, 176, 32)
	uiClose      = uiRGB(190, 48, 56)
	uiGreen      = uiRGB(53, 208, 127)
	uiCyan       = uiRGB(82, 193, 235)
)

func uiRGB(r, g, b uint32) uintptr {
	return uintptr(r | g<<8 | b<<16)
}

func readCurrentProcessValue[T any](address uintptr) (T, bool) {
	var value T
	if address == 0 {
		return value, false
	}
	if err := procGetCurrentProcess.Find(); err != nil {
		return value, false
	}
	if err := procReadProcessMemory.Find(); err != nil {
		return value, false
	}
	process, _, _ := procGetCurrentProcess.Call()
	var copied uintptr
	ok, _, _ := procReadProcessMemory.Call(
		process,
		address,
		uintptr(unsafe.Pointer(&value)),
		unsafe.Sizeof(value),
		uintptr(unsafe.Pointer(&copied)),
	)
	runtime.KeepAlive(&value)
	if ok == 0 || copied != unsafe.Sizeof(value) {
		return value, false
	}
	return value, true
}

func (a *appState) initializeUITheme() {
	if a.uiColorsReady {
		return
	}
	createBrush := func(color uintptr) uintptr {
		brush, _, _ := procCreateSolidBrush.Call(color)
		if brush == 0 {
			panic("Could not create native UI brush.")
		}
		return brush
	}
	createPen := func(style, width int, color uintptr) uintptr {
		pen, _, _ := procCreatePen.Call(uintptr(style), uintptr(width), color)
		if pen == 0 {
			panic("Could not create native UI pen.")
		}
		return pen
	}
	a.bgBrush = createBrush(uiBackground)
	a.sidebarBrush = createBrush(uiSidebar)
	a.cardBrush = createBrush(uiCardColor)
	a.fieldBrush = createBrush(uiField)
	a.borderBrush = createBrush(uiBorder)
	a.accentBrush = createBrush(uiAccent)
	a.cyanBrush = createBrush(uiCyan)
	a.closeBrush = createBrush(uiClose)
	a.borderPen = createPen(PS_SOLID, 1, uiBorder)
	a.accentPen = createPen(PS_SOLID, 2, uiAccent)
	a.cyanPen = createPen(PS_SOLID, 2, uiCyan)
	a.mutedPen = createPen(PS_DASH, 1, uiMuted)
	a.uiColorsReady = true
}

func (a *appState) releaseUITheme() {
	for _, object := range []uintptr{
		a.bgBrush, a.sidebarBrush, a.cardBrush, a.fieldBrush, a.borderBrush, a.accentBrush, a.cyanBrush, a.closeBrush,
		a.borderPen, a.accentPen, a.cyanPen, a.mutedPen,
	} {
		if object != 0 {
			procDeleteObject.Call(object)
		}
	}
	a.bgBrush, a.sidebarBrush, a.cardBrush, a.fieldBrush, a.borderBrush, a.accentBrush, a.cyanBrush, a.closeBrush = 0, 0, 0, 0, 0, 0, 0, 0
	a.borderPen, a.accentPen, a.cyanPen, a.mutedPen = 0, 0, 0, 0
	a.uiColorsReady = false
}

func (a *appState) eraseWindowBackground(hdc uintptr) bool {
	if !a.uiColorsReady {
		return false
	}
	var bounds rect
	if ok, _, _ := procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&bounds))); ok == 0 {
		return false
	}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&bounds)), a.bgBrush)
	titleBar := rect{Right: bounds.Right, Bottom: customTitleBarHeight}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&titleBar)), a.sidebarBrush)
	sidebar := rect{Left: 0, Top: customTitleBarHeight, Right: sidebarWidth, Bottom: bounds.Bottom}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&sidebar)), a.sidebarBrush)
	status := statusBarBounds(bounds)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&status)), a.sidebarBrush)
	return true
}

func (a *appState) drawCustomFrame(hdc uintptr) {
	if !a.uiColorsReady {
		return
	}
	var bounds rect
	if ok, _, _ := procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&bounds))); ok == 0 {
		return
	}
	header := rect{Right: bounds.Right, Bottom: customTitleBarHeight}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&header)), a.sidebarBrush)
	drawNativeText(hdc, "GALACTIC RACER  |  HOSAS / HOTAS BRIDGE",
		rect{Left: sidebarWidth + 18, Top: 4, Right: bounds.Right - 160, Bottom: customTitleBarHeight - 4},
		uiText, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)

	status := statusBarBounds(bounds)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&status)), a.sidebarBrush)
	statusText := rect{Left: status.Left + 16, Top: status.Top + 1, Right: status.Right - 16, Bottom: status.Bottom - 1}
	drawNativeText(hdc, statusBarVersion, statusText, uiMuted, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	statusText.Right -= 80
	drawNativeText(hdc, statusBarCopyright, statusText, uiMuted, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|DT_END_ELLIPSIS)

	oldPen, _, _ := procSelectObject.Call(hdc, a.borderPen)
	procMoveToEx.Call(hdc, 0, uintptr(status.Top), 0)
	procLineTo.Call(hdc, uintptr(bounds.Right), uintptr(status.Top))
	procMoveToEx.Call(hdc, 0, customTitleBarHeight-1, 0)
	procLineTo.Call(hdc, uintptr(bounds.Right), customTitleBarHeight-1)
	procMoveToEx.Call(hdc, 0, 0, 0)
	procLineTo.Call(hdc, 0, uintptr(bounds.Bottom))
	procMoveToEx.Call(hdc, uintptr(bounds.Right-1), 0, 0)
	procLineTo.Call(hdc, uintptr(bounds.Right-1), uintptr(bounds.Bottom))
	procMoveToEx.Call(hdc, 0, uintptr(bounds.Bottom-1), 0)
	procLineTo.Call(hdc, uintptr(bounds.Right), uintptr(bounds.Bottom-1))
	procSelectObject.Call(hdc, oldPen)
}

func (a *appState) drawNativeUI(hdc uintptr) {
	if !a.uiColorsReady {
		return
	}
	saved, _, _ := procSaveDC.Call(hdc)
	view := a.currentContentViewport()
	procIntersectClipRect.Call(hdc, uintptr(view.Left), uintptr(view.Top), uintptr(view.Right), uintptr(view.Bottom))
	defer procRestoreDC.Call(hdc, saved)
	for _, card := range a.cards {
		if card.section != a.topNavigationID {
			continue
		}
		if card.profile != "" && !strings.EqualFold(card.profile, a.selectedControlMode()) {
			continue
		}
		a.drawCard(hdc, card)
	}
}

func (a *appState) drawCard(hdc uintptr, card uiCard) {
	r := rect{
		Left:   card.bounds.Left - int32(a.scrollX),
		Top:    card.bounds.Top - int32(a.scrollY),
		Right:  card.bounds.Right - int32(a.scrollX),
		Bottom: card.bounds.Bottom - int32(a.scrollY),
	}
	oldPen, _, _ := procSelectObject.Call(hdc, a.borderPen)
	oldBrush, _, _ := procSelectObject.Call(hdc, a.cardBrush)
	procRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 16, 16)
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)

	title := rect{Left: r.Left + 18, Top: r.Top + 10, Right: r.Right - 16, Bottom: r.Top + 30}
	drawNativeText(hdc, strings.ToUpper(card.title), title, uiMuted, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)

	if card.meter != "" {
		label := rect{Left: r.Left + 20, Top: r.Top + 148, Right: r.Right - 20, Bottom: r.Top + 167}
		if a.mappingActive.Load() {
			drawNativeText(hdc, "LIVE MONITOR PAUSED", label, uiMuted, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		} else {
			value := a.liveLeftThrust
			if card.meter == "right" {
				value = a.liveRightThrust
			}
			drawNativeText(hdc, "BACKWARD  <  0  >  FORWARD", label, uiMuted, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
			drawNativeText(hdc, fmt.Sprintf("%+3.0f%%", value*100), label, uiAccent, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
			bar := rect{Left: r.Left + 20, Top: r.Top + 170, Right: r.Right - 20, Bottom: r.Top + 180}
			a.drawBipolarMeter(hdc, bar, value)
		}
	}
	if card.live {
		a.drawLiveCardContent(hdc, card)
	}
	if card.curve {
		if a.selectedControlMode() == "HOSAS" {
			a.drawThrottleCurve(hdc, r)
		} else {
			a.drawHotasResponseHint(hdc, r)
		}
	}
}

func (a *appState) drawHotasResponseHint(hdc uintptr, card rect) {
	a.cfgMu.RLock()
	c := a.cfg
	a.cfgMu.RUnlock()

	message := "Linear RT throttle; braking disabled."
	switch normalizeHotasBrakeBehavior(c.HotasBrakeBehavior) {
	case "Digital at Idle":
		message = fmt.Sprintf("Full LT at <=%d%% idle; RT follows throttle above.", c.HotasBrakeThreshold)
	case "Analog Brake Zone":
		message = fmt.Sprintf("Analog LT brake in first %d%%; remaining travel drives RT.", c.HotasBrakeZone)
	}
	heading := rect{Left: card.Left + 570, Top: card.Top + 32, Right: card.Right - 20, Bottom: card.Top + 52}
	body := rect{Left: heading.Left, Top: heading.Bottom + 4, Right: heading.Right, Bottom: card.Bottom - 8}
	drawNativeText(hdc, "HOTAS THROTTLE RESPONSE", heading, uiMuted, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	drawNativeText(hdc, message, body, uiText, DT_LEFT|DT_VCENTER|DT_WORDBREAK|DT_NOPREFIX)
}

func bipolarMeterFill(bounds rect, value float64) (rect, bool) {
	if bounds.Right <= bounds.Left || bounds.Bottom <= bounds.Top {
		return rect{}, false
	}
	value = clamp(value, -1, 1)
	if value == 0 {
		return rect{}, false
	}
	center := bounds.Left + (bounds.Right-bounds.Left)/2
	halfWidth := (bounds.Right - bounds.Left) / 2
	magnitude := value
	if magnitude < 0 {
		magnitude = -magnitude
	}
	fillWidth := int32(float64(halfWidth) * magnitude)
	if fillWidth == 0 {
		return rect{}, false
	}
	if value < 0 {
		return rect{Left: center - fillWidth, Top: bounds.Top, Right: center, Bottom: bounds.Bottom}, true
	}
	return rect{Left: center, Top: bounds.Top, Right: center + fillWidth, Bottom: bounds.Bottom}, true
}

func (a *appState) drawBipolarMeter(hdc uintptr, bounds rect, value float64) {
	oldPen, _, _ := procSelectObject.Call(hdc, a.borderPen)
	oldBrush, _, _ := procSelectObject.Call(hdc, a.fieldBrush)
	procRoundRect.Call(hdc, uintptr(bounds.Left), uintptr(bounds.Top), uintptr(bounds.Right), uintptr(bounds.Bottom), 8, 8)
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)

	center := bounds.Left + (bounds.Right-bounds.Left)/2
	oldPen, _, _ = procSelectObject.Call(hdc, a.mutedPen)
	procMoveToEx.Call(hdc, uintptr(center), uintptr(bounds.Top+1), 0)
	procLineTo.Call(hdc, uintptr(center), uintptr(bounds.Bottom-1))
	procSelectObject.Call(hdc, oldPen)

	fill, ok := bipolarMeterFill(bounds, value)
	if !ok {
		return
	}
	pen, brush := a.accentPen, a.accentBrush
	if value < 0 {
		pen, brush = a.cyanPen, a.cyanBrush
	}
	oldPen, _, _ = procSelectObject.Call(hdc, pen)
	oldBrush, _, _ = procSelectObject.Call(hdc, brush)
	procRoundRect.Call(hdc, uintptr(fill.Left), uintptr(fill.Top), uintptr(fill.Right), uintptr(fill.Bottom), 8, 8)
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
}

func (a *appState) controlIntValue(id, fallback int, normalize func(int) int) int {
	n, err := strconv.Atoi(strings.TrimSpace(getText(a.controls[id])))
	if err != nil {
		return fallback
	}
	return normalize(n)
}

func (a *appState) throttleScaleValue() int {
	return a.controlIntValue(idThrottleScale, 100, normalizeScalePercent)
}

func (a *appState) steerScaleValue() int {
	return a.controlIntValue(idSteerScale, 100, normalizeSteerScalePercent)
}

func (a *appState) throttleExpoValue() int {
	return a.controlIntValue(idThrottleExpo, 0, clampExpoPercent)
}

func (a *appState) steerExpoValue() int {
	return a.controlIntValue(idSteerExpo, 0, clampExpoPercent)
}

func (a *appState) steerExpo2Value() int {
	return a.controlIntValue(idSteerExpo2, 0, clampExpoPercent)
}

func (a *appState) selectedControlMode() string {
	if a.comboSelected[idControlMode] == 1 {
		return "HOTAS"
	}
	return "HOSAS"
}

func (a *appState) activeNavigationID() int {
	if isSectionNavigation(a.topNavigationID) {
		return a.topNavigationID
	}
	return idSectionSetup
}

func (a *appState) drawOwnerButton(item drawItem) bool {
	if item.CtlType != ODT_BUTTON || item.HwndItem == 0 {
		return false
	}
	// WM_DRAWITEM must return the DC in its original state.
	saved, _, _ := procSaveDC.Call(item.HDC)
	if saved != 0 {
		defer procRestoreDC.Call(item.HDC, saved)
		a.clipControlDC(item.HDC, item.HwndItem)
	}
	id := int(item.CtlID)
	r := item.RcItem
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return true
	}

	oldPen, _, _ := procSelectObject.Call(item.HDC, a.borderPen)
	oldBrush, _, _ := procSelectObject.Call(item.HDC, a.cardBrush)
	textColor := uiText
	fillColor := uiCardColor

	if a.checkboxVisible[id] == item.HwndItem {
		procRoundRect.Call(item.HDC, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 8, 8)
		procSelectObject.Call(item.HDC, oldBrush)
		procSelectObject.Call(item.HDC, oldPen)
		a.drawCheckbox(item, id)
		return true
	}
	switch {
	case isSectionNavigation(id):
		if id == a.activeNavigationID() {
			fillColor = uiCardColor
		} else {
			fillColor = uiSidebar
			textColor = uiMuted
		}
	case id == idSelectHOSAS || id == idSelectHOTAS:
		active := id == idSelectHOSAS && a.selectedControlMode() == "HOSAS" ||
			id == idSelectHOTAS && a.selectedControlMode() == "HOTAS"
		if active {
			fillColor = uiAccent
			textColor = uiBackground
		} else {
			fillColor = uiField
			textColor = uiMuted
		}
	case isCaptionButton(id):
		fillColor = uiSidebar
		textColor = uiMuted
	case id == idStart || id == idSave:
		fillColor = uiAccent
		textColor = uiBackground
	case id == idLeftDot || id == idRightDot || id == idDriveSteerDot:
		fillColor = uiAccent
	case a.isComboID(id):
		fillColor = uiField
	}
	if item.ItemState&ODS_SELECTED != 0 {
		if isCaptionButton(id) {
			if id == idWindowClose {
				fillColor = uiClose
			} else {
				fillColor = uiBorder
			}
			textColor = uiText
		} else if id != idSelectHOSAS && id != idSelectHOTAS && id != idStart && id != idSave {
			fillColor = uiBorder
		}
	}
	if item.ItemState&ODS_DISABLED != 0 {
		textColor = uiMuted
	}
	brush := a.brushForColor(fillColor)
	procSelectObject.Call(item.HDC, brush)
	pen := a.borderPen
	if id == idStart || id == idSave || id == idSelectHOSAS || id == idSelectHOTAS {
		pen = a.accentPen
	}
	procSelectObject.Call(item.HDC, pen)
	if id == idLeftDot || id == idRightDot || id == idDriveSteerDot {
		procEllipse.Call(item.HDC, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom))
	} else if isCaptionButton(id) {
		procRoundRect.Call(item.HDC, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 4, 4)
	} else {
		procRoundRect.Call(item.HDC, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), 10, 10)
	}
	procSetBkMode.Call(item.HDC, TRANSPARENT)
	label := rect{Left: r.Left + 8, Top: r.Top + 2, Right: r.Right - 8, Bottom: r.Bottom - 2}
	flags := uint32(DT_CENTER | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX)
	if id >= idLearnBase+200 && id < idLearnBase+200+len(buttonOutputs) {
		flags = DT_CENTER | DT_VCENTER | DT_WORDBREAK | DT_NOPREFIX
	}
	if isSectionNavigation(id) {
		label.Left = r.Left + 18
		label.Right = r.Right - 8
		flags = DT_LEFT | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX
		if id == a.activeNavigationID() {
			accent := rect{Left: r.Left + 2, Top: r.Top + 7, Right: r.Left + 5, Bottom: r.Bottom - 7}
			procFillRect.Call(item.HDC, uintptr(unsafe.Pointer(&accent)), a.accentBrush)
		}
	}
	if id == idLeftDot || id == idRightDot || id == idDriveSteerDot {
		textColor = uiBackground
		label = rect{}
	} else if a.isComboID(id) {
		label.Right -= 16
		drawNativeText(item.HDC, getText(item.HwndItem), label, textColor, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	} else {
		drawNativeText(item.HDC, getText(item.HwndItem), label, textColor, flags)
	}
	if item.ItemState&ODS_FOCUS != 0 {
		focus := rect{Left: r.Left + 4, Top: r.Top + 4, Right: r.Right - 4, Bottom: r.Bottom - 4}
		procDrawFocusRect.Call(item.HDC, uintptr(unsafe.Pointer(&focus)))
	}
	procSelectObject.Call(item.HDC, oldBrush)
	procSelectObject.Call(item.HDC, oldPen)
	return true
}

func (a *appState) drawCheckbox(item drawItem, id int) {
	r := item.RcItem
	boxSize := int32(14)
	box := rect{Left: r.Left + 2, Top: r.Top + (r.Bottom-r.Top-boxSize)/2, Right: r.Left + 2 + boxSize, Bottom: r.Top + (r.Bottom-r.Top-boxSize)/2 + boxSize}
	oldPen, _, _ := procSelectObject.Call(item.HDC, a.borderPen)
	oldBrush, _, _ := procSelectObject.Call(item.HDC, a.fieldBrush)
	procRoundRect.Call(item.HDC, uintptr(box.Left), uintptr(box.Top), uintptr(box.Right), uintptr(box.Bottom), 4, 4)
	procSelectObject.Call(item.HDC, oldBrush)
	procSelectObject.Call(item.HDC, oldPen)
	color := uiText
	if isChecked(a.controls[id]) {
		oldBrush, _, _ = procSelectObject.Call(item.HDC, a.accentBrush)
		oldPen, _, _ = procSelectObject.Call(item.HDC, a.accentPen)
		procRoundRect.Call(item.HDC, uintptr(box.Left+2), uintptr(box.Top+2), uintptr(box.Right-2), uintptr(box.Bottom-2), 3, 3)
		procSelectObject.Call(item.HDC, oldBrush)
		procSelectObject.Call(item.HDC, oldPen)
		color = uiText
	}
	label := rect{Left: box.Right + 6, Top: r.Top, Right: r.Right - 2, Bottom: r.Bottom}
	drawNativeText(item.HDC, getText(item.HwndItem), label, color, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	if item.ItemState&ODS_FOCUS != 0 {
		procDrawFocusRect.Call(item.HDC, uintptr(unsafe.Pointer(&r)))
	}
}

func (a *appState) brushForColor(color uintptr) uintptr {
	switch color {
	case uiAccent:
		return a.accentBrush
	case uiClose:
		return a.closeBrush
	case uiField:
		return a.fieldBrush
	case uiSidebar:
		return a.sidebarBrush
	case uiBorder:
		return a.borderBrush
	default:
		return a.cardBrush
	}
}

func isSectionNavigation(id int) bool {
	switch id {
	case idSectionSetup, idSectionTwist, idSectionMonitor, idSectionBindings, idSectionFoot, idSectionDevices:
		return true
	default:
		return false
	}
}

func isCaptionButton(id int) bool {
	return id == idWindowMinimize || id == idWindowMaximize || id == idWindowClose
}

func (a *appState) controlColor(message uint32, wParam, lParam uintptr) uintptr {
	if !a.uiColorsReady {
		return 0
	}
	hdc := wParam
	a.clipControlDC(hdc, lParam)
	id, _, _ := procGetDlgCtrlID.Call(lParam)
	textColor := uiText
	if lParam == a.sidebarTitle {
		textColor = uiAccent
	} else if lParam == a.sidebarFooter {
		textColor = uiMuted
	}
	if int(id) == idHidHideStatus || int(id) == idXInputReadback || int(id) == idHotasDIAxisMonitor {
		textColor = uiMuted
	}
	if int(id) == idStatus && a.mappingActive.Load() {
		textColor = uiGreen
	}
	procSetTextColor.Call(hdc, textColor)
	if message == WM_CTLCOLOREDIT {
		procSetBkColor.Call(hdc, uiField)
		return a.fieldBrush
	}
	if message == WM_CTLCOLORLISTBOX {
		procSetBkColor.Call(hdc, uiField)
		procSetBkMode.Call(hdc, 2)
		return a.fieldBrush
	}
	if message == WM_CTLCOLORBTN {
		procSetBkColor.Call(hdc, uiCardColor)
		procSetBkMode.Call(hdc, 2)
		return a.cardBrush
	}
	brush := a.surfaceBrushForControl(lParam)
	background := uiBackground
	if brush == a.cardBrush {
		background = uiCardColor
	} else if brush == a.sidebarBrush {
		background = uiSidebar
	} else if brush == a.fieldBrush {
		background = uiField
	}
	procSetBkColor.Call(hdc, background)
	procSetBkMode.Call(hdc, 2)
	return brush
}

func (a *appState) surfaceBrushForControl(hwnd uintptr) uintptr {
	if hwnd == a.sidebarDivider {
		return a.borderBrush
	}
	if hwnd == a.sidebarTitle || hwnd == a.sidebarFooter {
		return a.sidebarBrush
	}
	r, ok := a.baseRects[hwnd]
	if !ok {
		return a.bgBrush
	}
	if r.Left < sidebarWidth {
		return a.sidebarBrush
	}
	for _, card := range a.cards {
		if r.Left >= card.bounds.Left && r.Right <= card.bounds.Right &&
			r.Top >= card.bounds.Top && r.Bottom <= card.bounds.Bottom &&
			(card.profile == "" || strings.EqualFold(card.profile, a.selectedControlMode())) {
			return a.cardBrush
		}
	}
	return a.bgBrush
}

func drawNativeText(hdc uintptr, text string, bounds rect, color uintptr, flags uint32) {
	if text == "" || bounds.Right <= bounds.Left || bounds.Bottom <= bounds.Top {
		return
	}
	encoded := utf16.Encode([]rune(text))
	if len(encoded) == 0 {
		return
	}
	procSetTextColor.Call(hdc, color)
	procSetBkMode.Call(hdc, TRANSPARENT)
	procDrawTextW.Call(
		hdc,
		uintptr(unsafe.Pointer(&encoded[0])),
		uintptr(len(encoded)),
		uintptr(unsafe.Pointer(&bounds)),
		uintptr(flags),
	)
	runtime.KeepAlive(encoded)
}
