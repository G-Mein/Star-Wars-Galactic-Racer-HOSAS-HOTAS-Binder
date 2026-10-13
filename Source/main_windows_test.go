//go:build windows

package main

import (
	"os"
	"testing"
)

func TestLoadConfigMigrationPreservesSettingsWithNamedAxes(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	data := `{
		"input_backend":"DirectInput",
		"control_mode":"HOTAS",
		"hotas_throttle_id":7,
		"hotas_throttle_di_guid":"throttle-guid",
		"hotas_throttle_axis":"Y",
		"hotas_throttle_invert":true,
		"hotas_brake_behavior":"Analog Brake Zone",
		"hotas_brake_threshold":9,
		"hotas_brake_zone":33,
		"hotas_stick_id":8,
		"hotas_stick_di_guid":"stick-guid",
		"hotas_stick_x_axis":" X Rotation (Rx) ",
		"hotas_stick_y_axis":"Slider 2",
		"hotas_stick_invert_x":true,
		"hotas_stick_invert_y":true,
		"left_id":2,
		"left_di_guid":"left-guid",
		"right_id":3,
		"right_di_guid":"right-guid",
		"pedal_id":4,
		"pedal_di_guid":"pedal-guid",
		"twist_left":{
			"device_id":5,
			"di_guid":"left-twist-guid",
			"axis":"X Rotation (R)",
			"invert":true,
			"axis_type":"Lever",
			"deadzone":7,
			"mode":"Fine steering",
			"steer_strength":45,
			"camera_sensitivity":120,
			"camera_deadzone":8,
			"left_key":46,
			"right_key":34,
			"key_threshold":25
		},
		"twist_right":{
			"device_id":6,
			"di_guid":"right-twist-guid",
			"axis":"Z Rotation (Rz)",
			"axis_type":"Centered",
			"mode":"Move Camera"
		},
		"left_axis":"X",
		"right_axis":"Y",
		"pedal_axis":"Z",
		"left_invert":true,
		"right_invert":true,
		"left_throttle_mode":true,
		"right_throttle_mode":true,
		"pedal_invert":true,
		"pedal_mode":"Pitch",
		"deadzone":8,
		"throttle_scale":65,
		"throttle_expo":40,
		"steer_scale":70,
		"steer_expo":35,
		"pedal_steer_strength":44,
		"camera_sensitivity":123,
		"camera_deadzone":9,
		"pedal_left_key":46,
		"pedal_right_key":34,
		"pedal_key_threshold":28,
		"thumb_wasd_enabled":true,
		"thumb_wasd_device_id":9,
		"thumb_wasd_side_axis":"X",
		"thumb_wasd_forward_axis":"Y",
		"thumb_wasd_invert_side":true,
		"thumb_wasd_invert_forward":true,
		"thumb_wasd_threshold":31,
		"thumb_wasd_use_raw_hid":true,
		"thumb_hid_side_path":"side-path",
		"thumb_hid_forward_path":"forward-path",
		"thumb_hid_side_usage":48,
		"thumb_hid_forward_usage":49,
		"thumb_hid_side_center":-1234,
		"thumb_hid_forward_center":2345,
		"thumb_hid_side_span":4567,
		"thumb_hid_forward_span":5678,
		"on_foot_mode":"Buttons -> WASD",
		"thumb_di_side_guid":"thumb-side-guid",
		"thumb_di_forward_guid":"thumb-forward-guid",
		"thumb_di_device_name":"thumb-device",
		"thumb_di_side_axis":3,
		"thumb_di_forward_axis":4,
		"thumb_di_side_center":111,
		"thumb_di_forward_center":222,
		"thumb_di_side_span":333,
		"thumb_di_forward_span":444,
		"polling_hz":250,
		"bindings":{
			"PITCHUP":{"kind":"button","backend":"DirectInput","device_id":8,"di_guid":"pitch-guid","button":2}
		}
	}`
	if err := os.WriteFile(configPath(), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	var got config
	loadConfig(&got)

	if got.InputBackend != "DirectInput" || got.ControlMode != "HOTAS" ||
		got.HotasThrottleID != 7 || got.HotasThrottleDIGUID != "throttle-guid" ||
		got.HotasThrottleAxis != 1 || !got.HotasThrottleInvert ||
		got.HotasBrakeBehavior != "Analog Brake Zone" || got.HotasBrakeThreshold != 9 ||
		got.HotasBrakeZone != 33 || got.HotasStickID != 8 || got.HotasStickDIGUID != "stick-guid" ||
		got.HotasStickXAxis != 3 || got.HotasStickYAxis != 7 ||
		!got.HotasStickInvertX || !got.HotasStickInvertY {
		t.Errorf("HOTAS settings not preserved: %+v", got)
	}
	if got.LeftID != 2 || got.LeftDIGUID != "left-guid" || got.RightID != 3 ||
		got.RightDIGUID != "right-guid" || got.PedalID != 4 || got.PedalDIGUID != "pedal-guid" ||
		got.LeftAxis != 0 || got.RightAxis != 1 || got.PedalAxis != 2 ||
		!got.LeftInvert || !got.RightInvert || !got.LeftThrottleMode || !got.RightThrottleMode ||
		!got.PedalInvert || got.PedalMode != "Pitch" || got.Deadzone != 8 ||
		got.ThrottleScale != 65 || got.ThrottleExpo != 40 || got.SteerScale != 70 || got.SteerExpo != 35 {
		t.Errorf("HOSAS/pedal settings not preserved: %+v", got)
	}
	if got.TwistLeft.DeviceID != 5 || got.TwistLeft.DIGUID != "left-twist-guid" ||
		got.TwistLeft.Axis != 3 || !got.TwistLeft.Invert || got.TwistLeft.AxisType != "Lever" ||
		got.TwistLeft.Deadzone != 7 || got.TwistLeft.Mode != "Fine steering" ||
		got.TwistLeft.SteerStrength != 45 || got.TwistLeft.CameraSensitivity != 120 ||
		got.TwistLeft.CameraDeadzone != 8 || got.TwistLeft.LeftKey != 46 ||
		got.TwistLeft.RightKey != 34 || got.TwistLeft.KeyThreshold != 25 ||
		got.TwistRight.DeviceID != 6 || got.TwistRight.DIGUID != "right-twist-guid" ||
		got.TwistRight.Axis != 5 || got.TwistRight.Mode != "Move Camera" {
		t.Errorf("twist settings not preserved: left=%+v right=%+v", got.TwistLeft, got.TwistRight)
	}
	if got.PedalSteerStrength != 44 || got.CameraSensitivity != 123 ||
		got.CameraDeadzone != 9 || got.PedalLeftKey != 46 || got.PedalRightKey != 34 ||
		got.PedalKeyThreshold != 28 || !got.ThumbWASDEnabled || got.ThumbWASDDeviceID != 9 ||
		got.ThumbWASDSideAxis != 0 || got.ThumbWASDForwardAxis != 1 ||
		!got.ThumbWASDInvertSide || !got.ThumbWASDInvertForward || got.ThumbWASDThreshold != 31 ||
		!got.ThumbWASDUseRawHID || got.ThumbHIDSidePath != "side-path" ||
		got.ThumbHIDForwardPath != "forward-path" || got.ThumbHIDSideUsage != 48 ||
		got.ThumbHIDForwardUsage != 49 || got.ThumbHIDSideCenter != -1234 ||
		got.ThumbHIDForwardCenter != 2345 || got.ThumbHIDSideSpan != 4567 ||
		got.ThumbHIDForwardSpan != 5678 || got.OnFootMode != "Buttons -> WASD" {
		t.Errorf("on-foot settings not preserved: %+v", got)
	}
	if got.ThumbDISideGUID != "thumb-side-guid" || got.ThumbDIForwardGUID != "thumb-forward-guid" ||
		got.ThumbDIDeviceName != "thumb-device" || got.ThumbDISideAxis != 3 ||
		got.ThumbDIForwardAxis != 4 || got.ThumbDISideCenter != 111 ||
		got.ThumbDIForwardCenter != 222 || got.ThumbDISideSpan != 333 ||
		got.ThumbDIForwardSpan != 444 || got.PollingHz != 250 ||
		got.Bindings["PITCHUP"].DIGUID != "pitch-guid" || got.Bindings["PITCHUP"].Button != 2 {
		t.Errorf("DirectInput/on-foot mappings not preserved: %+v", got)
	}
}

func TestLoadConfigIgnoresLegacyThrottleKeysAndDefaultsScale(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	data := `{"control_mode":"HOSAS","left_full_throttle":60,"right_full_throttle":75,"preserve_thrust":true}`
	if err := os.WriteFile(configPath(), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	var got config
	loadConfig(&got)

	if got.ThrottleScale != 100 || got.ThrottleExpo != 0 || got.SteerScale != 100 || got.SteerExpo != 0 || got.SteerExpo2 != 0 {
		t.Fatalf("scales = throttle %d steer %d expo %d, want 100/100/0 for legacy config", got.ThrottleScale, got.SteerScale, got.SteerExpo)
	}
}

func TestLoadConfigNormalizesInvalidCurveValues(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	data := `{"control_mode":"HOSAS","throttle_scale":80,"throttle_expo":-105,"steer_scale":5,"steer_expo":120,"steer_expo2":-130}`
	if err := os.WriteFile(configPath(), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	var got config
	loadConfig(&got)

	if got.ThrottleScale != 80 || got.ThrottleExpo != 0 || got.SteerScale != 50 || got.SteerExpo != 0 || got.SteerExpo2 != 0 {
		t.Fatalf("got throttle %d/%d steer %d/%d, want 80/0 50/0", got.ThrottleScale, got.ThrottleExpo, got.SteerScale, got.SteerExpo)
	}
}

func TestOverlaySnapshotNowModeText(t *testing.T) {
	state := &appState{}
	state.cfg = config{ControlMode: "HOSAS", ThrottleScale: 80, ThrottleExpo: 0, SteerScale: 60, SteerExpo: 25, SteerExpo2: -40}
	state.monitor.LeftY = 0.5
	snap := state.overlaySnapshotNow()
	if snap.Mode != "HOSAS T80/0 S60/25/-40" || snap.LeftY != 0.5 {
		t.Fatalf("snapshot = %+v", snap)
	}
	state.cfg.ControlMode = "HOTAS"
	if got := state.overlaySnapshotNow().Mode; got != "HOTAS" {
		t.Fatalf("HOTAS mode = %q", got)
	}
}

func TestLiveCardFitsBesideRightStick(t *testing.T) {
	state := &appState{}
	state.createSectionPanels()
	var live, right uiCard
	for _, card := range state.cards {
		switch card.title {
		case liveCardTitle:
			live = card
		case "RIGHT STICK":
			right = card
		}
	}
	if !live.live || live.section != idSectionSetup || live.profile != "HOSAS" {
		t.Fatalf("live card = %+v", live)
	}
	if right.bounds.Right >= live.bounds.Left || live.bounds.Top != right.bounds.Top || live.bounds.Bottom != right.bounds.Bottom {
		t.Fatalf("live card %+v overlaps or misaligns with right stick %+v", live.bounds, right.bounds)
	}
	content := liveCardContentRect(live, 0, 0)
	width, height := content.Right-content.Left, content.Bottom-content.Top
	layout := liveCardBoxLayout(width)
	if layout.left < 0 || layout.left+layout.width() > width {
		t.Fatalf("boxes %d..%d exceed content width %d", layout.left, layout.left+layout.width(), width)
	}
	if layout.top+layout.height()+2+liveCardModeH > height {
		t.Fatalf("content height %d too small for boxes+mode %d", height, layout.top+layout.height()+2+liveCardModeH)
	}
	if !liveCardVisible(live, idSectionSetup, "hosas") || liveCardVisible(live, idSectionSetup, "HOTAS") || liveCardVisible(live, idSectionMonitor, "HOSAS") {
		t.Fatal("live card visibility wrong")
	}
	scrolled := liveCardContentRect(live, 30, 40)
	if scrolled.Left != content.Left-30 || scrolled.Top != content.Top-40 {
		t.Fatalf("scrolled content = %+v", scrolled)
	}
}

func TestLiveBoxLayoutMatchesOverlayBoxes(t *testing.T) {
	for i := 0; i < 3; i++ {
		if overlayLiveLayout.box(i) != overlayBoxRect(i) {
			t.Fatalf("box %d mismatch", i)
		}
	}
	w, _ := overlaySize()
	if overlayLiveLayout.left+overlayLiveLayout.width()+overlayMargin != w {
		t.Fatalf("overlay layout width %d does not match window %d", overlayLiveLayout.width(), w)
	}
}

func TestLoadConfigReadsSecondSteerCurve(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	data := `{"control_mode":"HOSAS","steer_expo":40,"steer_expo2":-60}`
	if err := os.WriteFile(configPath(), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	var got config
	loadConfig(&got)
	if got.SteerExpo != 40 || got.SteerExpo2 != -60 {
		t.Fatalf("steer curves = %d/%d, want 40/-60", got.SteerExpo, got.SteerExpo2)
	}
}
