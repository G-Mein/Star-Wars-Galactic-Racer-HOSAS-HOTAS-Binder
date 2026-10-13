package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestReadAxisSupportsSavedNamesAndIndexes(t *testing.T) {
	tests := []struct {
		name string
		json string
		want int
	}{
		{name: "WinMM X", json: `"X"`, want: 0},
		{name: "WinMM Y", json: `"Y"`, want: 1},
		{name: "WinMM Z", json: `"Z"`, want: 2},
		{name: "WinMM rotation R", json: `"R"`, want: 3},
		{name: "WinMM rotation U", json: `"U"`, want: 4},
		{name: "WinMM rotation V", json: `"V"`, want: 5},
		{name: "DirectInput Rx label", json: `" X Rotation (Rx) "`, want: 3},
		{name: "DirectInput Ry shorthand", json: `"ry"`, want: 4},
		{name: "DirectInput Rz label", json: `"Z Rotation (Rz)"`, want: 5},
		{name: "DirectInput slider one", json: `"Slider 1"`, want: 6},
		{name: "DirectInput slider two", json: `"Slider 2"`, want: 7},
		{name: "numeric axis", json: `6`, want: 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var value json.RawMessage
			if err := json.Unmarshal([]byte(tt.json), &value); err != nil {
				t.Fatal(err)
			}
			got := -1
			readAxis(map[string]json.RawMessage{"axis": value}, "axis", &got)
			if got != tt.want {
				t.Fatalf("readAxis(%s) = %d, want %d", tt.json, got, tt.want)
			}
		})
	}
}

func TestCustomFrameHitTest(t *testing.T) {
	bounds := rect{Left: 100, Top: 100, Right: 500, Bottom: 400}
	tests := []struct {
		name      string
		cursor    point
		maximized bool
		want      uintptr
	}{
		{name: "top left resize", cursor: point{101, 101}, want: HTTOPLEFT},
		{name: "top resize", cursor: point{300, 101}, want: HTTOP},
		{name: "top right resize", cursor: point{499, 101}, want: HTTOPRIGHT},
		{name: "right resize", cursor: point{499, 200}, want: HTRIGHT},
		{name: "bottom resize", cursor: point{300, 399}, want: HTBOTTOM},
		{name: "status bar body is client", cursor: point{300, 385}, want: HTCLIENT},
		{name: "bottom left resize", cursor: point{101, 399}, want: HTBOTTOMLEFT},
		{name: "bottom right resize", cursor: point{499, 399}, want: HTBOTTOMRIGHT},
		{name: "left resize", cursor: point{101, 200}, want: HTLEFT},
		{name: "title drag", cursor: point{300, 120}, want: HTCAPTION},
		{name: "caption buttons remain interactive", cursor: point{470, 120}, want: HTCLIENT},
		{name: "area beside caption buttons remains draggable", cursor: point{353, 120}, want: HTCAPTION},
		{name: "titlebar lower edge", cursor: point{300, 143}, want: HTCAPTION},
		{name: "content below titlebar", cursor: point{300, 144}, want: HTCLIENT},
		{name: "content", cursor: point{300, 200}, want: HTCLIENT},
		{name: "maximized title drag", cursor: point{300, 120}, maximized: true, want: HTCAPTION},
		{name: "maximized edge is not resize", cursor: point{101, 200}, maximized: true, want: HTCLIENT},
		{name: "maximized caption buttons remain interactive", cursor: point{470, 120}, maximized: true, want: HTCLIENT},
		{name: "outside window", cursor: point{99, 200}, want: HTNOWHERE},
		{name: "negative monitor coordinates", cursor: point{-100, -100}, want: HTNOWHERE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := customFrameHitTest(bounds, tt.cursor, tt.maximized, 8, 8)
			if got != tt.want {
				t.Fatalf("customFrameHitTest(%+v, %+v, %t) = %d, want %d", bounds, tt.cursor, tt.maximized, got, tt.want)
			}
		})
	}
}

func TestNavigationSectionForTitle(t *testing.T) {
	tests := []struct {
		title string
		want  int
	}{
		{title: "LEFT STICK", want: idSectionSetup},
		{title: "THROTTLE", want: idSectionSetup},
		{title: "DRIVING RESPONSE", want: idSectionSetup},
		{title: "INPUT & DEVICE TOOLS", want: idSectionDevices},
		{title: "TWIST AXES", want: idSectionTwist},
		{title: "RUDDER PEDALS", want: idSectionTwist},
		{title: "ON-FOOT CONTROLS", want: idSectionFoot},
		{title: "VIRTUAL CONTROLLER & LIVE MONITOR", want: idSectionMonitor},
		{title: "BUTTON MAPPINGS", want: idSectionBindings},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			if got := navigationSectionForTitle(tt.title); got != tt.want {
				t.Fatalf("navigationSectionForTitle(%q) = %d, want %d", tt.title, got, tt.want)
			}
		})
	}
}

func TestControlPageVisibilityUsesExplicitOwnershipAcrossAllSections(t *testing.T) {
	sections := []int{
		idSectionSetup,
		idSectionBindings,
		idSectionTwist,
		idSectionFoot,
		idSectionDevices,
		idSectionMonitor,
	}
	overlap := rect{Left: 230, Top: 145, Right: 350, Bottom: 173}
	controlSections := make(map[uintptr]int, len(sections))
	baseRects := make(map[uintptr]rect, len(sections)+1)
	for i, section := range sections {
		control := uintptr(i + 1)
		controlSections[control] = section
		baseRects[control] = overlap
	}
	const unownedControl uintptr = 99
	baseRects[unownedControl] = overlap

	state := appState{controlSections: controlSections, baseRects: baseRects}
	for _, activeSection := range sections {
		state.topNavigationID = activeSection
		for control, owner := range controlSections {
			if got := state.controlPageVisible(control); got != (owner == activeSection) {
				t.Errorf("control %d owned by section %d visible on section %d = %t", control, owner, activeSection, got)
			}
		}
		if state.controlPageVisible(unownedControl) {
			t.Errorf("control without explicit ownership should be hidden on section %d", activeSection)
		}
	}
	if !state.controlPageVisible(100) {
		t.Fatal("fixed control without a scrollable rect should remain visible")
	}
}

func TestNavigationScrollRange(t *testing.T) {
	cards := []uiCard{
		{section: idSectionSetup, bounds: rect{Top: 84, Bottom: 434}},
		{section: idSectionDevices, bounds: rect{Top: 84, Bottom: 248}},
		{section: idSectionTwist, bounds: rect{Top: 294, Bottom: 586}},
		{section: idSectionFoot, bounds: rect{Top: 604, Bottom: 744}},
		{section: idSectionMonitor, bounds: rect{Top: 884, Bottom: 1234}},
		{section: idSectionBindings, bounds: rect{Top: 1044, Bottom: 1826}},
	}
	fitSectionCardToControls(
		cards,
		idSectionBindings,
		map[uintptr]int{1: idSectionBindings},
		map[uintptr]rect{1: {Bottom: 1750}},
		16,
	)
	if got, want := cards[len(cards)-1].bounds.Bottom, int32(1766); got != want {
		t.Fatalf("fitted binding card bottom = %d, want %d", got, want)
	}
	tests := []struct {
		name    string
		section int
		height  int
		wantMin int
		wantMax int
	}{
		{name: "setup starts at top and fits after reflow", section: idSectionSetup, height: 600, wantMin: 0, wantMax: 0},
		{name: "device page starts at top", section: idSectionDevices, height: 600, wantMin: 0, wantMax: 0},
		{name: "twist page opens below fixed profile selector", section: idSectionTwist, height: 600, wantMin: 200, wantMax: 200},
		{name: "on-foot page opens below fixed profile selector", section: idSectionFoot, height: 600, wantMin: 510, wantMax: 510},
		{name: "monitor opens below fixed profile selector", section: idSectionMonitor, height: 600, wantMin: 790, wantMax: 790},
		{name: "bindings opens below profile and scrolls to last control", section: idSectionBindings, height: 600, wantMin: 950, wantMax: 1166},
		{name: "bindings remain scrollable in a short window", section: idSectionBindings, height: 400, wantMin: 950, wantMax: 1366},
		{name: "binding section fits at 900px height", section: idSectionBindings, height: 900, wantMin: 950, wantMax: 950},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := navigationScrollRange(cards, tt.section, tt.height)
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Fatalf("navigationScrollRange() = (%d, %d), want (%d, %d)", gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestVerticalScrollBarVisibilityAndBounds(t *testing.T) {
	if shouldShowVerticalScrollBar(950, 950) {
		t.Fatal("scrollbar should be hidden when the active page fits")
	}
	if !shouldShowVerticalScrollBar(950, 1366) {
		t.Fatal("scrollbar should be visible when the active page overflows")
	}

	got := verticalScrollBarBounds(rect{Left: 0, Top: 0, Right: 1380, Bottom: 400})
	want := rect{
		Left:   1380 - windowResizeBorder - scrollBarWidth,
		Top:    fixedProfileBottom + 2,
		Right:  1380 - windowResizeBorder,
		Bottom: 400 - statusBarHeight,
	}
	if got != want {
		t.Fatalf("verticalScrollBarBounds() = %+v, want %+v", got, want)
	}

	tiny := verticalScrollBarBounds(rect{Left: 0, Top: 0, Right: 10, Bottom: 20})
	if tiny.Right < tiny.Left || tiny.Bottom < tiny.Top {
		t.Fatalf("verticalScrollBarBounds() returned inverted bounds for a tiny client: %+v", tiny)
	}
}

func TestScrolledCardsStayBelowFixedProfileSelector(t *testing.T) {
	state := &appState{}
	state.createSectionPanels()
	for i := range state.cards {
		state.cards[i].bounds.Top += customTitleBarHeight
		state.cards[i].bounds.Bottom += customTitleBarHeight
	}
	cards := state.cards
	fitSectionCardToControls(
		cards,
		idSectionBindings,
		map[uintptr]int{1: idSectionBindings},
		map[uintptr]rect{1: {Bottom: 1750}},
		16,
	)
	for _, section := range []int{idSectionSetup, idSectionDevices, idSectionTwist, idSectionFoot, idSectionMonitor, idSectionBindings} {
		minimum, _ := navigationScrollRange(cards, section, 600)
		for _, card := range cards {
			if card.section != section {
				continue
			}
			if top := int(card.bounds.Top) - minimum; top < fixedProfileBottom+2 {
				t.Errorf("section %d card %q top = %d, want >= %d", section, card.title, top, fixedProfileBottom+2)
			}
		}
	}
}

func TestTwistControlsStayInsideCardTitleAndBottom(t *testing.T) {
	titleBottom := sectionTwistPanelY + 30
	if twistLeftRowY < titleBottom+6 {
		t.Fatalf("left twist controls start at %d, want at least %d", twistLeftRowY, titleBottom+6)
	}
	if twistRightDetailY+25 > sectionTwistPanelY+178 {
		t.Fatalf("right twist details end at %d, card ends at %d", twistRightDetailY+25, sectionTwistPanelY+178)
	}
}

func TestPedalControlsStayBelowTitleAndInsideCard(t *testing.T) {
	const scrollY = 200
	cardTop := sectionPedalPanelY + customTitleBarHeight - scrollY
	cardBottom := sectionPedalPanelY + 110 + customTitleBarHeight - scrollY
	firstControlTop := 178 + sectionContentYOffset(idSectionTwist) + customTitleBarHeight - scrollY
	lastSettingBottom := 212 + sectionContentYOffset(idSectionTwist) + customTitleBarHeight - scrollY + 25
	if firstControlTop < cardTop+36 {
		t.Fatalf("first pedal control top = %d, want >= %d", firstControlTop, cardTop+36)
	}
	if lastSettingBottom > cardBottom {
		t.Fatalf("last pedal setting bottom = %d, card bottom = %d", lastSettingBottom, cardBottom)
	}
}

func TestOnFootControlsStayInsideTheirCard(t *testing.T) {
	cardTop := sectionFootPanelY + customTitleBarHeight
	firstControlTop := 264 + sectionContentYOffset(idSectionFoot) + customTitleBarHeight
	lastControlBottom := 300 + sectionContentYOffset(idSectionFoot) + customTitleBarHeight + 52
	if firstControlTop < cardTop+36 {
		t.Fatalf("first on-foot control top = %d, want >= %d", firstControlTop, cardTop+36)
	}
	if lastControlBottom > cardTop+140 {
		t.Fatalf("on-foot monitor bottom = %d, card bottom = %d", lastControlBottom, cardTop+140)
	}
}

func TestDeviceCardAndControlsStayBelowFixedProfileSelector(t *testing.T) {
	state := &appState{}
	state.createSectionPanels()
	var deviceCard uiCard
	for _, card := range state.cards {
		if card.title == "INPUT & DEVICE TOOLS" {
			deviceCard = card
			break
		}
	}
	deviceCardTop := int(deviceCard.bounds.Top) + customTitleBarHeight
	if deviceCardTop < fixedProfileBottom+2 {
		t.Fatalf("device card top = %d, want >= %d", deviceCardTop, fixedProfileBottom+2)
	}
	deviceCardTitleBottom := deviceCardTop + 30
	deviceComboTop := customTitleBarHeight + 74
	if deviceComboTop < deviceCardTitleBottom+4 {
		t.Fatalf("device combo top = %d, want >= %d", deviceComboTop, deviceCardTitleBottom+4)
	}
}

func TestSectionContentYOffset(t *testing.T) {
	tests := []struct {
		name    string
		section int
		want    int
	}{
		{name: "twist controls fit their card", section: idSectionTwist, want: layoutShift + 16},
		{name: "on-foot controls fit their card", section: idSectionFoot, want: layoutShift + 42},
		{name: "driving response controls reflow below setup cards", section: idSectionSetup, want: layoutShift - 430},
		{name: "monitor controls stay below the card heading", section: idSectionMonitor, want: layoutShift + 116},
		{name: "mapping controls fit within the page card", section: idSectionBindings, want: layoutShift - 30},
		{name: "device controls use the normal content layout", section: idSectionDevices, want: layoutShift},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sectionContentYOffset(tt.section); got != tt.want {
				t.Fatalf("sectionContentYOffset(%d) = %d, want %d", tt.section, got, tt.want)
			}
		})
	}
}

func TestBipolarMeterFill(t *testing.T) {
	bounds := rect{Left: 0, Top: 0, Right: 100, Bottom: 10}
	tests := []struct {
		name string
		in   float64
		want rect
		ok   bool
	}{
		{name: "forward fills from neutral to right", in: 0.5, want: rect{Left: 50, Top: 0, Right: 75, Bottom: 10}, ok: true},
		{name: "reverse fills from neutral to left", in: -0.5, want: rect{Left: 25, Top: 0, Right: 50, Bottom: 10}, ok: true},
		{name: "forward clamps at full scale", in: 1.5, want: rect{Left: 50, Top: 0, Right: 100, Bottom: 10}, ok: true},
		{name: "reverse clamps at full scale", in: -1.5, want: rect{Left: 0, Top: 0, Right: 50, Bottom: 10}, ok: true},
		{name: "neutral has no fill", in: 0, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bipolarMeterFill(bounds, tt.in)
			if ok != tt.ok || (ok && got != tt.want) {
				t.Fatalf("bipolarMeterFill(%+v, %v) = (%+v, %t), want (%+v, %t)", bounds, tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
	if _, ok := bipolarMeterFill(rect{Left: 10, Right: 10, Bottom: 10}, 0.5); ok {
		t.Fatal("zero-width meter should not produce a fill")
	}
}

func TestBipolarMeterFillAtLiveMonitorResolution(t *testing.T) {
	bounds := rect{Left: 0, Top: 0, Right: 360, Bottom: 10}
	tests := []struct {
		name string
		in   float64
		want rect
	}{
		{name: "one percent forward remains visible", in: 0.01, want: rect{Left: 180, Top: 0, Right: 181, Bottom: 10}},
		{name: "one percent reverse remains visible", in: -0.01, want: rect{Left: 179, Top: 0, Right: 180, Bottom: 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bipolarMeterFill(bounds, tt.in)
			if !ok || got != tt.want {
				t.Fatalf("bipolarMeterFill(%+v, %v) = (%+v, %t), want (%+v, true)", bounds, tt.in, got, ok, tt.want)
			}
		})
	}
	for _, invalid := range []rect{
		{Left: 10, Top: 0, Right: 0, Bottom: 10},
		{Left: 0, Top: 10, Right: 10, Bottom: 0},
	} {
		if _, ok := bipolarMeterFill(invalid, 0.5); ok {
			t.Errorf("bipolarMeterFill(%+v, 0.5) should reject inverted bounds", invalid)
		}
	}
}

func TestLiveThrustMeterInvalidationIsScopedAndScrolled(t *testing.T) {
	left := uiCard{section: idSectionSetup, profile: "HOSAS", meter: "left"}
	right := uiCard{section: idSectionSetup, profile: "HOSAS", meter: "right"}
	hotas := uiCard{section: idSectionSetup, profile: "HOTAS", meter: "left"}
	wrongSection := uiCard{section: idSectionMonitor, profile: "HOSAS", meter: "left"}
	if !liveThrustMeterVisible(left, idSectionSetup, "HOSAS") ||
		!liveThrustMeterVisible(right, idSectionSetup, "HOSAS") {
		t.Fatal("HOSAS meters should be visible on the active HOSAS setup page")
	}
	if liveThrustMeterVisible(hotas, idSectionSetup, "HOSAS") ||
		liveThrustMeterVisible(wrongSection, idSectionSetup, "HOSAS") {
		t.Fatal("inactive profile or page meter should not be invalidated")
	}

	card := uiCard{bounds: rect{Left: 220, Top: 84, Right: 620, Bottom: 284}}
	got := liveThrustMeterDirtyRect(card, 35, 20)
	want := rect{Left: 205, Top: 212, Right: 565, Bottom: 245}
	if got != want {
		t.Fatalf("liveThrustMeterDirtyRect() = %+v, want %+v", got, want)
	}
}

func TestNormalizeThrottleScale(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "unset defaults to full", in: 0, want: 100},
		{name: "negative defaults to full", in: -10, want: 100},
		{name: "minimum is clamped", in: 1, want: 10},
		{name: "just below minimum is clamped", in: 9, want: 10},
		{name: "minimum boundary is preserved", in: 10, want: 10},
		{name: "valid value is preserved", in: 70, want: 70},
		{name: "100 is preserved", in: 100, want: 100},
		{name: "amplified value is preserved", in: 150, want: 150},
		{name: "maximum boundary is preserved", in: 200, want: 200},
		{name: "above range clamps to 200", in: 201, want: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeScalePercent(tt.in); got != tt.want {
				t.Fatalf("normalizeScalePercent(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
func TestNormalizeBindingKeyPreservesOnFootAssignments(t *testing.T) {
	for _, key := range []string{"WASD_W", "wasd_w", "WASDW"} {
		if got := normalizeBindingKey(key); got != "WASD_W" {
			t.Errorf("normalizeBindingKey(%q) = %q, want WASD_W", key, got)
		}
	}
	for _, key := range []string{"WASD_A", "WASDA", "WASD_S", "WASDS", "WASD_D", "WASDD"} {
		if got := normalizeBindingKey(key); !strings.HasPrefix(got, "WASD_") {
			t.Errorf("normalizeBindingKey(%q) = %q, want canonical on-foot key", key, got)
		}
	}
	if got := normalizeBindingKey("Xbox A"); got != "A" {
		t.Errorf("normalizeBindingKey(Xbox A) = %q, want A", got)
	}
}

func TestCalculateDriveSumsSticksWithSharedScale(t *testing.T) {
	tests := []struct {
		name        string
		left, right float64
		scale, expo int
		want        float64
	}{
		{name: "one stick full, other centered", left: 1, right: 0, scale: 100, want: 1},
		{name: "right stick full, left centered", left: 0, right: 1, scale: 100, want: 1},
		{name: "both sticks half forward", left: 0.5, right: 0.5, scale: 100, want: 1},
		{name: "full forward and full back cancel", left: 1, right: -1, scale: 100, want: 0},
		{name: "scale 80 limits single stick", left: 1, right: 0, scale: 80, want: 0.8},
		{name: "scale 80 both half forward", left: 0.5, right: 0.5, scale: 80, want: 0.8},
		{name: "drive scale 5 clamps to 10 percent", left: 1, right: 0, scale: 5, want: 0.1},
		{name: "drive scale 120 saturates single stick", left: 1, right: 0, scale: 120, want: 1},
		{name: "scale 200 keeps full thrust with right half back", left: 1, right: -0.5, scale: 200, want: 1},
		{name: "scale 150 amplifies partial counter", left: 1, right: -0.5, scale: 150, want: 0.75},
		{name: "scale 200 one stick half is full", left: 0.5, right: 0, scale: 200, want: 1},
		{name: "scale 200 full opposite still zero", left: 1, right: -1, scale: 200, want: 0},
		{name: "scale 200 reverse symmetric", left: -1, right: 0.5, scale: 200, want: -1},
		{name: "scale 200 curve applies after gain", left: 0.25, right: 0, scale: 200, expo: 100, want: 0.125},
		{name: "scale 300 clamps to 200", left: 0.5, right: 0, scale: 300, want: 1},
		{name: "expo above range defaults to linear", left: 0.5, right: 0, scale: 100, expo: 105, want: 0.5},
		{name: "expo below range defaults to linear", left: 0.5, right: 0, scale: 100, expo: -101, want: 0.5},
		{name: "negative curve boosts half input", left: 0.5, right: 0, scale: 100, expo: -100, want: 0.875},
		{name: "negative curve keeps full at max", left: 1, right: 0, scale: 80, expo: -100, want: 0.8},
		{name: "negative curve reverse symmetric", left: -0.5, right: 0, scale: 100, expo: -50, want: -0.6875},
		{name: "summed half sticks reach max with expo", left: 0.5, right: 0.5, scale: 75, expo: 80, want: 0.75},
		{name: "overshoot clamps to max with expo", left: 0.8, right: 0.6, scale: 70, expo: 60, want: 0.7},
		{name: "scale 50 limits single stick", left: 1, right: 0, scale: 50, want: 0.5},
		{name: "smooth release without deadzone jump", left: 0.75, right: 0, scale: 100, want: 0.75},
		{name: "reverse is symmetric", left: -1, right: 0, scale: 100, want: -1},
		{name: "reverse respects scale", left: 0, right: -1, scale: 50, want: -0.5},
		{name: "both full forward clamps", left: 1, right: 1, scale: 100, want: 1},
		{name: "both full back clamps", left: -1, right: -1, scale: 100, want: -1},
		{name: "invalid scale defaults to 100", left: 1, right: 0, scale: 0, want: 1},
		{name: "curve softens half input", left: 0.5, right: 0, scale: 100, expo: 100, want: 0.125},
		{name: "curve keeps full input at max", left: 1, right: 0, scale: 80, expo: 100, want: 0.8},
		{name: "curve 50 blends linear and cubic", left: 0.25, right: 0.25, scale: 100, expo: 50, want: 0.3125},
		{name: "curve reverse is symmetric", left: -0.5, right: 0, scale: 100, expo: 100, want: -0.125},
		{name: "curve both full forward clamps", left: 1, right: 1, scale: 100, expo: 60, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateDrive(tt.left, tt.right, tt.scale, tt.expo); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("calculateDrive(%v, %v, %d, %d) = %v, want %v", tt.left, tt.right, tt.scale, tt.expo, got, tt.want)
			}
		})
	}
}

func TestCalculateSteeringScaleAndExpo(t *testing.T) {
	tests := []struct {
		name        string
		left, right float64
		scale, expo int
		expo2       int
		want        float64
	}{
		{name: "default single stick linear", left: 0.5, right: 0, scale: 100, expo: 0, expo2: 0, want: 0.5},
		{name: "equal deflection is straight", left: 0.5, right: 0.5, scale: 100, expo: 0, expo2: 0, want: 0},
		{name: "clamp positive", left: 1, right: -1, scale: 100, expo: 0, expo2: 0, want: 1},
		{name: "clamp negative", left: -1, right: 1, scale: 100, expo: 0, expo2: 0, want: -1},
		{name: "left full right centered at max 50", left: 1, right: 0, scale: 50, expo: 0, expo2: 0, want: 0.5},
		{name: "full opposite reaches 100 at max 50", left: 1, right: -1, scale: 50, expo: 0, expo2: 0, want: 1},
		{name: "full opposite reaches 100 at max 70", left: 1, right: -1, scale: 70, expo: 0, expo2: 0, want: 1},
		{name: "half counter-stick fills linearly", left: 1, right: -0.5, scale: 70, expo: 0, expo2: 0, want: 0.85},
		{name: "counter-stick negative direction", left: -0.5, right: 1, scale: 60, expo: 0, expo2: 0, want: -0.8},
		{name: "full opposite with expo still 100", left: -1, right: 1, scale: 40, expo: 100, expo2: 100, want: -1},
		{name: "counter-stick follows curve", left: 1, right: -0.5, scale: 50, expo: 100, expo2: 100, want: 0.5 + 0.5*0.125},
		{name: "beyond range clamps", left: 1.5, right: -1.5, scale: 70, expo: 0, expo2: 0, want: 1},
		{name: "full expo keeps full deflection", left: 1, right: 0, scale: 100, expo: 100, expo2: 100, want: 1},
		{name: "steer scale 50 half counter is 75", left: 1, right: -0.5, scale: 50, expo: 0, expo2: 0, want: 0.75},
		{name: "full expo softens center", left: 0.5, right: 0, scale: 100, expo: 100, expo2: 100, want: 0.125},
		{name: "half expo blends over full travel", left: 0.5, right: 0, scale: 100, expo: 50, expo2: 50, want: 0.3125},
		{name: "expo is symmetric", left: 0, right: 0.5, scale: 100, expo: 100, expo2: 100, want: -0.125},
		{name: "expo then scale", left: 0.5, right: 0, scale: 50, expo: 100, expo2: 100, want: 0.0625},
		{name: "invalid scale defaults to 100", left: 1, right: 0, scale: 0, expo: 0, expo2: 0, want: 1},
		{name: "invalid expo defaults to linear", left: 0.5, right: 0, scale: 100, expo: 101, expo2: 101, want: 0.5},
		{name: "expo below range defaults to linear", left: 0.5, right: 0, scale: 100, expo: -101, expo2: -101, want: 0.5},
		{name: "negative curve boosts center", left: 0.5, right: 0, scale: 100, expo: -100, expo2: -100, want: 0.875},
		{name: "negative curve keeps max steering point", left: 1, right: 0, scale: 60, expo: -100, expo2: -100, want: 0.6},
		{name: "negative curve full counter is 100", left: -1, right: 1, scale: 50, expo: -70, expo2: -70, want: -1},
		{name: "full deflection with scale 50 and full expo", left: 1, right: 0, scale: 50, expo: 100, expo2: 100, want: 0.5},
		{name: "full negative deflection with scale 60 and expo 80", left: 0, right: 1, scale: 60, expo: 80, expo2: 80, want: -0.6},
		{name: "steer scale 5 clamps to 50 percent", left: 1, right: 0, scale: 5, expo: 0, expo2: 0, want: 0.5},
		{name: "steer scale 30 clamps to 50 percent", left: 1, right: 0, scale: 30, expo: 0, expo2: 0, want: 0.5},
		{name: "curve 2 does not affect one-stick segment", left: 0.5, right: 0, scale: 50, expo: 100, expo2: -100, want: 0.0625},
		{name: "curve 1 does not affect counter segment", left: 1, right: -0.5, scale: 70, expo: 100, expo2: 0, want: 0.85},
		{name: "soft start with inverted counter curve", left: 1, right: -0.5, scale: 50, expo: 100, expo2: -100, want: 0.5 + 0.5*0.875},
		{name: "curve 2 full counter still 100", left: 1, right: -1, scale: 50, expo: 60, expo2: -60, want: 1},
		{name: "scale 120 defaults to 100 percent", left: 1, right: 0, scale: 120, expo: 0, expo2: 0, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSteering(tt.left, tt.right, tt.scale, tt.expo, tt.expo2)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("calculateSteering(%v, %v, %d, %d, %d) = %v, want %v", tt.left, tt.right, tt.scale, tt.expo, tt.expo2, got, tt.want)
			}
		})
	}
}

func TestNormalizeExpoPercent(t *testing.T) {
	for in, want := range map[int]int{-101: 0, -100: -100, -5: -5, 0: 0, 1: 1, 35: 35, 100: 100, 101: 0} {
		if got := normalizeExpoPercent(in); got != want {
			t.Errorf("normalizeExpoPercent(%d) = %d, want %d", in, got, want)
		}
	}
	for in, want := range map[int]int{-150: -100, -5: -5, 0: 0, 35: 35, 100: 100, 150: 100} {
		if got := clampExpoPercent(in); got != want {
			t.Errorf("clampExpoPercent(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestSidebarNavigationFitsAboveStatusBar(t *testing.T) {
	if got := sidebarNavigationY(5); got != customTitleBarHeight+316 {
		t.Fatalf("last navigation y = %d, want %d", got, customTitleBarHeight+316)
	}
	statusTop := 400 - statusBarHeight
	if sidebarNavigationFits(5, statusTop) {
		t.Fatal("last navigation entry must be hidden when it would overlap the status bar")
	}
	if !sidebarNavigationFits(4, statusTop) {
		t.Fatal("entry ending above the status bar must stay visible")
	}
	if !sidebarNavigationFits(5, sidebarNavigationY(5)+sidebarNavigationHeight) {
		t.Fatal("entry ending exactly at the boundary must fit")
	}
}

func TestStatusBarTextCreditsAuthorsAndVersion(t *testing.T) {
	for _, want := range []string{"JaffaQuake and Mars", "Modified by G-Mein", "GPL-3.0"} {
		if !strings.Contains(statusBarCopyright, want) {
			t.Errorf("statusBarCopyright %q missing %q", statusBarCopyright, want)
		}
	}
	if statusBarVersion == "" || !strings.Contains(appTitle, statusBarVersion) {
		t.Errorf("statusBarVersion %q not part of appTitle %q", statusBarVersion, appTitle)
	}
}
