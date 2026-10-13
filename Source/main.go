//go:build windows

// Galactic Racer HOSAS / HOTAS Bridge
// Copyright (C) 2026 JaffaQuake
// Modifications Copyright (C) 2026 G-Mein (https://github.com/G-Mein)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// The application icon is embedded directly into the Go binary for the window/taskbar.
// The same .ico is also shipped beside the EXE so Windows shortcuts can use it.
//
//go:embed GalacticRacerHOSAS.ico
var embeddedAppIcon []byte

const (
	appVersion = "v2.0"
	appTitle   = "Galactic Racer HOSAS / HOTAS Bridge " + appVersion + " — by JaffaQuake and Mars"
	// Status bar notice; must stay consistent with the README credits (GPL-3.0 section 5a).
	statusBarCopyright = "© 2026 JaffaQuake and Mars  ·  Modified by G-Mein (github.com/G-Mein)  ·  GPL-3.0-or-later"
	statusBarVersion   = appVersion

	WS_OVERLAPPED   = 0x00000000
	WS_CAPTION      = 0x00C00000
	WS_SYSMENU      = 0x00080000
	WS_THICKFRAME   = 0x00040000
	WS_MINIMIZEBOX  = 0x00020000
	WS_MAXIMIZEBOX  = 0x00010000
	WS_CHILD        = 0x40000000
	WS_VISIBLE      = 0x10000000
	WS_TABSTOP      = 0x00010000
	WS_BORDER       = 0x00800000
	WS_VSCROLL      = 0x00200000
	WS_HSCROLL      = 0x00100000
	WS_POPUP        = 0x80000000
	WS_CLIPCHILDREN = 0x02000000
	WS_CLIPSIBLINGS = 0x04000000
	HWND_TOP        = 0
	SBS_VERT        = 0x0001

	SC_SIZE     = 0xF000
	SC_MOVE     = 0xF010
	SC_MINIMIZE = 0xF020
	SC_MAXIMIZE = 0xF030
	SC_CLOSE    = 0xF060
	SC_RESTORE  = 0xF120

	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_NOACTIVATE  = 0x08000000

	TTS_ALWAYSTIP      = 0x01
	TTS_NOPREFIX       = 0x02
	TTF_IDISHWND       = 0x0001
	TTF_SUBCLASS       = 0x0010
	TTM_ADDTOOLW       = 0x0432
	TTM_SETMAXTIPWIDTH = 0x0418

	BS_PUSHBUTTON    = 0x00000000
	BS_MULTILINE     = 0x00002000
	BS_AUTOCHECKBOX  = 0x00000003
	BS_OWNERDRAW     = 0x0000000B
	CBS_DROPDOWNLIST = 0x00000003
	LBS_NOTIFY       = 0x00000001
	ES_NUMBER        = 0x00002000
	SS_LEFT          = 0x00000000
	ODT_BUTTON       = 4
	ODS_SELECTED     = 0x0001
	ODS_DISABLED     = 0x0004
	ODS_FOCUS        = 0x0010
	DT_LEFT          = 0x0000
	DT_CENTER        = 0x0001
	DT_RIGHT         = 0x0002
	DT_VCENTER       = 0x0004
	DT_WORDBREAK     = 0x0010
	DT_SINGLELINE    = 0x0020
	DT_NOPREFIX      = 0x0800
	DT_END_ELLIPSIS  = 0x8000
	TRANSPARENT      = 1
	PS_SOLID         = 0
	PS_DASH          = 1

	TBM_GETPOS      = 0x0400
	TBM_SETPOS      = 0x0405
	TBM_SETRANGE    = 0x0406
	TBM_SETRANGEMIN = 0x0407
	TBM_SETRANGEMAX = 0x0408
	TB_ENDTRACK     = 8

	CW_USEDEFAULT     = ^uintptr(0x7fffffff)
	SW_HIDE           = 0
	SW_MAXIMIZE       = 3
	SW_SHOWNOACTIVATE = 4
	SW_SHOW           = 5
	SW_MINIMIZE       = 6
	SW_RESTORE        = 9

	SIZE_RESTORED  = 0
	SIZE_MINIMIZED = 1
	SIZE_MAXIMIZED = 2

	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_SIZE            = 0x0005
	WM_CLOSE           = 0x0010
	WM_ERASEBKGND      = 0x0014
	WM_LBUTTONDOWN     = 0x0201
	WM_MOUSEWHEEL      = 0x020A
	WM_PARENTNOTIFY    = 0x0210
	WM_DRAWITEM        = 0x002B
	WM_PAINT           = 0x000F
	WM_CTLCOLORMSGBOX  = 0x0132
	WM_CTLCOLOREDIT    = 0x0133
	WM_CTLCOLORLISTBOX = 0x0134
	WM_CTLCOLORBTN     = 0x0135
	WM_CTLCOLORSTATIC  = 0x0138
	WM_COMMAND         = 0x0111
	WM_HSCROLL         = 0x0114
	WM_VSCROLL         = 0x0115
	WM_TIMER           = 0x0113
	WM_SETFONT         = 0x0030
	WM_SETICON         = 0x0080
	WM_NCCALCSIZE      = 0x0083
	WM_NCHITTEST       = 0x0084
	WM_SYSCOMMAND      = 0x0112
	WM_APP             = 0x8000

	HTCLIENT      = 1
	HTCAPTION     = 2
	HTLEFT        = 10
	HTRIGHT       = 11
	HTTOP         = 12
	HTTOPLEFT     = 13
	HTTOPRIGHT    = 14
	HTBOTTOM      = 15
	HTBOTTOMLEFT  = 16
	HTBOTTOMRIGHT = 17
	HTNOWHERE     = 0

	WM_APP_STATUS = WM_APP + 1
	WM_APP_DETECT = WM_APP + 2
	WM_APP_LEARN  = WM_APP + 3
	WM_APP_XBOX   = WM_APP + 4

	BM_GETCHECK = 0x00F0
	BM_SETCHECK = 0x00F1
	BM_SETSTATE = 0x00F3
	BST_CHECKED = 1

	SWP_NOSIZE       = 0x0001
	SWP_NOMOVE       = 0x0002
	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010
	SWP_FRAMECHANGED = 0x0020
	SWP_SHOWWINDOW   = 0x0040
	SWP_HIDEWINDOW   = 0x0080
	SWP_NOCOPYBITS   = 0x0100

	RDW_INVALIDATE  = 0x0001
	RDW_ERASE       = 0x0004
	RDW_NOERASE     = 0x0020
	RDW_NOCHILDREN  = 0x0040
	RDW_ALLCHILDREN = 0x0080
	RDW_UPDATENOW   = 0x0100

	CB_ADDSTRING    = 0x0143
	CB_RESETCONTENT = 0x014B
	CB_GETCURSEL    = 0x0147
	CB_SETCURSEL    = 0x014E
	LB_ADDSTRING    = 0x0180
	LB_RESETCONTENT = 0x0184
	LB_GETCURSEL    = 0x0188
	LB_SETCURSEL    = 0x0186

	BN_CLICKED    = 0
	CBN_SELCHANGE = 1
	LBN_SELCHANGE = 1
	LBN_DBLCLK    = 2
	LBN_KILLFOCUS = 5
	EN_CHANGE     = 0x0300
	EN_KILLFOCUS  = 0x0200

	DEFAULT_GUI_FONT = 17
	IDC_ARROW        = 32512
	ICON_SMALL       = 0
	ICON_BIG         = 1

	JOY_RETURNX       = 0x00000001
	JOY_RETURNY       = 0x00000002
	JOY_RETURNZ       = 0x00000004
	JOY_RETURNR       = 0x00000008
	JOY_RETURNU       = 0x00000010
	JOY_RETURNV       = 0x00000020
	JOY_RETURNPOV     = 0x00000040
	JOY_RETURNBUTTONS = 0x00000080
	JOY_RETURNALL     = 0x000000FF
	JOY_POVCENTERED   = 0xFFFF
	JOYERR_NOERROR    = 0

	RIM_TYPEHID         = 2
	RIDI_DEVICENAME     = 0x20000007
	GENERIC_READ        = 0x80000000
	FILE_SHARE_READ     = 0x00000001
	FILE_SHARE_WRITE    = 0x00000002
	OPEN_EXISTING       = 3
	HIDP_INPUT          = 0
	HIDP_STATUS_SUCCESS = 0x00110000

	XUSB_DPAD_UP        = 0x0001
	XUSB_DPAD_DOWN      = 0x0002
	XUSB_DPAD_LEFT      = 0x0004
	XUSB_DPAD_RIGHT     = 0x0008
	XUSB_START          = 0x0010
	XUSB_BACK           = 0x0020
	XUSB_LEFT_THUMB     = 0x0040
	XUSB_RIGHT_THUMB    = 0x0080
	XUSB_LEFT_SHOULDER  = 0x0100
	XUSB_RIGHT_SHOULDER = 0x0200
	XUSB_A              = 0x1000
	XUSB_B              = 0x2000
	XUSB_X              = 0x4000
	XUSB_Y              = 0x8000

	VK_END          = 0x23
	VK_DELETE       = 0x2E
	VK_NEXT         = 0x22 // Page Down
	KEYEVENTF_KEYUP = 0x0002
)

const (
	customTitleBarHeight = 44
	captionButtonWidth   = 46
	captionButtonHeight  = 32
	captionButtonInset   = 8
	windowResizeBorder   = 8
	scrollBarWidth       = 16
	statusBarHeight      = 24

	sidebarWidth         = 200
	contentOffset        = 210
	layoutShift          = 300
	sectionTwistPanelY   = 250
	sectionPedalPanelY   = 450
	sectionFootPanelY    = 560
	sectionDrivingPanelY = 250
	sectionMonitorPanelY = 840
	sectionBindingPanelY = 1000
	sectionDevicesPanelY = 40
	fixedProfileBottom   = customTitleBarHeight + 38
	sectionNavInset      = fixedProfileBottom - customTitleBarHeight + 12
	twistLeftRowY        = sectionTwistPanelY + 36
	twistLeftDetailY     = twistLeftRowY + 37
	twistRightRowY       = twistLeftRowY + 74
	twistRightDetailY    = twistRightRowY + 37
)

const (
	idControlMode = 1001 + iota
	idInputBackend
	idHotasThrottleDevice
	idHotasThrottleAxis
	idHotasThrottleInvert
	idHotasBrakeBehavior
	idHotasBrakeThreshold
	idHotasBrakeZone
	idHotasStickDevice
	idHotasStickXAxis
	idHotasStickYAxis
	idHotasStickInvertX
	idHotasStickInvertY
	idLeftDevice
	idLeftAxis
	idLeftInvert
	idLeftThrottleMode
	idDetectLeft
	idRightDevice
	idRightAxis
	idRightInvert
	idRightThrottleMode
	idDetectRight
	idPedalDevice
	idPedalAxis
	idPedalInvert
	idPedalMode
	idPedalStrength
	idDeadzone
	idThrottleScale
	idStart
	idTestA
	idTestCamLeft
	idTestCamRight
	idCameraSensitivity
	idCameraDeadzone
	idPedalLeftKey
	idPedalRightKey
	idPedalKeyThreshold
	idWASDEnable
	idWASDDevice
	idWASDSideAxis
	idWASDForwardAxis
	idWASDInvertSide
	idWASDInvertForward
	idWASDThreshold
	idDetectWASD
	idPollingRate
	idSave
	idRefresh
	idHidHideSetup
	idHidHideStatus
	idMonitor
	idStatus
	idXInputReadback
	idLeftDot
	idRightDot
	idDriveSteerDot
	idLeftVectorText
	idRightVectorText
	idDriveSteerVectorText
	idRawHIDMonitor
	idOnFootMode
	idDIThumbMonitor
	idHotasDIAxisMonitor
	idHelpText
	idTwistLeftDevice
	idTwistLeftAxis
	idTwistLeftInvert
	idTwistLeftAxisType
	idTwistLeftDeadzone
	idTwistLeftMode
	idTwistLeftStrength
	idTwistLeftCameraSensitivity
	idTwistLeftCameraDeadzone
	idTwistLeftLeftKey
	idTwistLeftRightKey
	idTwistLeftKeyThreshold
	idTwistRightDevice
	idTwistRightAxis
	idTwistRightInvert
	idTwistRightAxisType
	idTwistRightDeadzone
	idTwistRightMode
	idTwistRightStrength
	idTwistRightCameraSensitivity
	idTwistRightCameraDeadzone
	idTwistRightLeftKey
	idTwistRightRightKey
	idTwistRightKeyThreshold
	idTwistLeftOutputNegative
	idTwistRightOutputNegative
	idSteerScale
	idSteerExpo
	idThrottleExpo
	idSteerExpo2
	idComboList = 1700
	idLearnBase = 1200
)

const (
	idSectionSetup = 1710 + iota
	idSectionTwist
	idSectionMonitor
	idSectionBindings
	idSectionFoot
	idSectionDevices
	idSelectHOSAS
	idSelectHOTAS
)

const (
	idWindowMinimize = 1730 + iota
	idWindowMaximize
	idWindowClose
)

const (
	idWASDLearnBase       = 1500
	idWASDLabelBase       = 1510
	idWASDIndicatorBase   = 1520
	idLearnClearBase      = 1600
	idWASDClearBase       = 1650
	idThrottleScaleSlider = 1800
	idSteerScaleSlider    = 1801
	idSteerExpoSlider     = 1802
	idThrottleExpoSlider  = 1803
	idSteerExpo2Slider    = 1804
	idShowOverlay         = 1810
)

type point struct{ X, Y int32 }
type msg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}
type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type joyCaps struct {
	Mid                  uint16
	Pid                  uint16
	Pname                [32]uint16
	Xmin, Xmax           uint32
	Ymin, Ymax           uint32
	Zmin, Zmax           uint32
	NumButtons           uint32
	PeriodMin, PeriodMax uint32
	Rmin, Rmax           uint32
	Umin, Umax           uint32
	Vmin, Vmax           uint32
	Caps                 uint32
	MaxAxes              uint32
	NumAxes              uint32
	MaxButtons           uint32
	RegKey               [32]uint16
	OEMVxD               [260]uint16
}

type joyInfoEx struct {
	Size             uint32
	Flags            uint32
	X, Y, Z, R, U, V uint32
	Buttons          uint32
	ButtonNumber     uint32
	POV              uint32
	Reserved1        uint32
	Reserved2        uint32
}

type deviceInfo struct {
	ID   int
	Name string
	Caps joyCaps
}

type binding struct {
	Kind       string `json:"kind"`
	Backend    string `json:"backend,omitempty"`
	DeviceID   int    `json:"device_id"`
	DIGUID     string `json:"di_guid,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
	Button     int    `json:"button,omitempty"`
	POVIndex   int    `json:"pov_index,omitempty"`
	POV        int    `json:"pov,omitempty"`
}

type keyOption struct {
	Name string
	VK   int
}

// auxiliaryAxisConfig mirrors the pedal controls without sharing settings between axes.
// A lever is 0..1 travel; a centered control is -1..+1 around its midpoint.
type auxiliaryAxisConfig struct {
	DeviceID          int    `json:"device_id"`
	DIGUID            string `json:"di_guid"`
	Axis              int    `json:"axis"`
	Invert            bool   `json:"invert"`
	OutputNegative    bool   `json:"output_negative"`
	AxisType          string `json:"axis_type"`
	Deadzone          int    `json:"deadzone"`
	Mode              string `json:"mode"`
	SteerStrength     int    `json:"steer_strength"`
	CameraSensitivity int    `json:"camera_sensitivity"`
	CameraDeadzone    int    `json:"camera_deadzone"`
	LeftKey           int    `json:"left_key"`
	RightKey          int    `json:"right_key"`
	KeyThreshold      int    `json:"key_threshold"`
}

func defaultAuxiliaryAxis() auxiliaryAxisConfig {
	return auxiliaryAxisConfig{DeviceID: -1, AxisType: "Centered", Deadzone: 6, Mode: "Off",
		SteerStrength: 30, CameraSensitivity: 100, CameraDeadzone: 6,
		LeftKey: VK_DELETE, RightKey: VK_NEXT, KeyThreshold: 20}
}

func normalizeAxisType(s string) string {
	if strings.EqualFold(strings.TrimSpace(s), "Lever") {
		return "Lever"
	}
	return "Centered"
}

func validateAuxiliaryAxis(v *auxiliaryAxisConfig, present bool, maxAxis int) {
	if !present {
		*v = defaultAuxiliaryAxis()
		return
	}
	v.AxisType = normalizeAxisType(v.AxisType)
	v.Mode = normalizePedalMode(v.Mode)
	if v.DeviceID < -1 {
		v.DeviceID = -1
	}
	if v.Axis < 0 || v.Axis > maxAxis {
		v.Axis = 0
	}
	if v.Deadzone < 0 || v.Deadzone > 40 {
		v.Deadzone = 6
	}
	if v.SteerStrength < 0 || v.SteerStrength > 100 {
		v.SteerStrength = 30
	}
	if v.CameraSensitivity < 10 || v.CameraSensitivity > 250 {
		v.CameraSensitivity = 100
	}
	if v.CameraDeadzone < 0 || v.CameraDeadzone > 40 {
		v.CameraDeadzone = 6
	}
	if !validKeyboardVK(v.LeftKey) {
		v.LeftKey = VK_DELETE
	}
	if !validKeyboardVK(v.RightKey) {
		v.RightKey = VK_NEXT
	}
	if v.KeyThreshold < 5 || v.KeyThreshold > 95 {
		v.KeyThreshold = 20
	}
}

func normalizeAuxiliaryValue(centered float64, v auxiliaryAxisConfig, deadzone int) float64 {
	if normalizeAxisType(v.AxisType) == "Lever" {
		out := clamp((centered+1)/2, 0, 1)
		if v.Invert {
			out = 1 - out
		}
		dz := clamp(float64(deadzone)/100, 0, .4)
		if out <= dz {
			return 0
		}
		out = clamp((out-dz)/(1-dz), 0, 1)
		if v.OutputNegative {
			return -out
		}
		return out
	}
	if v.Invert {
		centered = -centered
	}
	return applyDeadzone(centered, deadzone)
}

type config struct {
	InputBackend           string              `json:"input_backend"`
	ControlMode            string              `json:"control_mode"`
	HotasThrottleID        int                 `json:"hotas_throttle_id"`
	HotasThrottleDIGUID    string              `json:"hotas_throttle_di_guid"`
	HotasThrottleAxis      int                 `json:"hotas_throttle_axis"`
	HotasThrottleInvert    bool                `json:"hotas_throttle_invert"`
	HotasBrakeBehavior     string              `json:"hotas_brake_behavior"`
	HotasBrakeThreshold    int                 `json:"hotas_brake_threshold"`
	HotasBrakeZone         int                 `json:"hotas_brake_zone"`
	HotasStickID           int                 `json:"hotas_stick_id"`
	HotasStickDIGUID       string              `json:"hotas_stick_di_guid"`
	HotasStickXAxis        int                 `json:"hotas_stick_x_axis"`
	HotasStickYAxis        int                 `json:"hotas_stick_y_axis"`
	HotasStickInvertX      bool                `json:"hotas_stick_invert_x"`
	HotasStickInvertY      bool                `json:"hotas_stick_invert_y"`
	LeftID                 int                 `json:"left_id"`
	LeftDIGUID             string              `json:"left_di_guid"`
	RightID                int                 `json:"right_id"`
	RightDIGUID            string              `json:"right_di_guid"`
	PedalID                int                 `json:"pedal_id"`
	PedalDIGUID            string              `json:"pedal_di_guid"`
	TwistLeft              auxiliaryAxisConfig `json:"twist_left"`
	TwistRight             auxiliaryAxisConfig `json:"twist_right"`
	LeftAxis               int                 `json:"left_axis"`
	RightAxis              int                 `json:"right_axis"`
	PedalAxis              int                 `json:"pedal_axis"`
	LeftInvert             bool                `json:"left_invert"`
	RightInvert            bool                `json:"right_invert"`
	LeftThrottleMode       bool                `json:"left_throttle_mode"`
	RightThrottleMode      bool                `json:"right_throttle_mode"`
	PedalInvert            bool                `json:"pedal_invert"`
	PedalMode              string              `json:"pedal_mode"`
	Deadzone               int                 `json:"deadzone"`
	ThrottleScale          int                 `json:"throttle_scale"`
	ThrottleExpo           int                 `json:"throttle_expo"`
	SteerScale             int                 `json:"steer_scale"`
	SteerExpo              int                 `json:"steer_expo"`
	SteerExpo2             int                 `json:"steer_expo2"`
	Bindings               map[string]binding  `json:"bindings"`
	PedalSteerStrength     int                 `json:"pedal_steer_strength"`
	CameraSensitivity      int                 `json:"camera_sensitivity"`
	CameraDeadzone         int                 `json:"camera_deadzone"`
	PedalLeftKey           int                 `json:"pedal_left_key"`
	PedalRightKey          int                 `json:"pedal_right_key"`
	PedalKeyThreshold      int                 `json:"pedal_key_threshold"`
	ThumbWASDEnabled       bool                `json:"thumb_wasd_enabled"`
	ThumbWASDDeviceID      int                 `json:"thumb_wasd_device_id"`
	ThumbWASDSideAxis      int                 `json:"thumb_wasd_side_axis"`
	ThumbWASDForwardAxis   int                 `json:"thumb_wasd_forward_axis"`
	ThumbWASDInvertSide    bool                `json:"thumb_wasd_invert_side"`
	ThumbWASDInvertForward bool                `json:"thumb_wasd_invert_forward"`
	ThumbWASDThreshold     int                 `json:"thumb_wasd_threshold"`
	ThumbWASDUseRawHID     bool                `json:"thumb_wasd_use_raw_hid"`
	ThumbHIDSidePath       string              `json:"thumb_hid_side_path"`
	ThumbHIDForwardPath    string              `json:"thumb_hid_forward_path"`
	ThumbHIDSideUsage      int                 `json:"thumb_hid_side_usage"`
	ThumbHIDForwardUsage   int                 `json:"thumb_hid_forward_usage"`
	ThumbHIDSideCenter     int32               `json:"thumb_hid_side_center"`
	ThumbHIDForwardCenter  int32               `json:"thumb_hid_forward_center"`
	ThumbHIDSideSpan       int32               `json:"thumb_hid_side_span"`
	ThumbHIDForwardSpan    int32               `json:"thumb_hid_forward_span"`
	OnFootMode             string              `json:"on_foot_mode"`
	ThumbDISideGUID        string              `json:"thumb_di_side_guid"`
	ThumbDIForwardGUID     string              `json:"thumb_di_forward_guid"`
	ThumbDIDeviceName      string              `json:"thumb_di_device_name"`
	ThumbDISideAxis        int                 `json:"thumb_di_side_axis"`
	ThumbDIForwardAxis     int                 `json:"thumb_di_forward_axis"`
	ThumbDISideCenter      int32               `json:"thumb_di_side_center"`
	ThumbDIForwardCenter   int32               `json:"thumb_di_forward_center"`
	ThumbDISideSpan        int32               `json:"thumb_di_side_span"`
	ThumbDIForwardSpan     int32               `json:"thumb_di_forward_span"`
	PollingHz              int                 `json:"polling_hz"`
}

type xusbReport struct {
	Buttons      uint16
	LeftTrigger  byte
	RightTrigger byte
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputGamepad struct {
	Buttons      uint16
	LeftTrigger  byte
	RightTrigger byte
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputState struct {
	PacketNumber uint32
	Gamepad      xinputGamepad
}

type monitorState struct {
	Left, Right, Pedal, TwistLeft, TwistRight, Drive, Steer float64
	Throttle                                                float64
	HotasBrake                                              bool
	HotasBrakeAmount                                        float64
	LeftX, LeftY, RightX, RightY                            float64
	CameraX                                                 float64
	OutputButtons                                           uint16
	OutputHighlights                                        uint32
	OnFootHighlights                                        uint8
	LastUpdate                                              time.Time
}

type detectResult struct {
	Side             string
	DeviceID         int
	Axis             int
	Axis2            int
	Invert           bool
	Invert2          bool
	DeviceName       string
	OK               bool
	Message          string
	UseRawHID        bool
	HIDSidePath      string
	HIDForwardPath   string
	HIDSideUsage     int
	HIDForwardUsage  int
	HIDSideCenter    int32
	HIDForwardCenter int32
	HIDSideSpan      int32
	HIDForwardSpan   int32
	UseDirectInput   bool
	DISideGUID       string
	DIForwardGUID    string
	DISideCenter     int32
	DIForwardCenter  int32
	DISideSpan       int32
	DIForwardSpan    int32
}

type learnResult struct {
	Key     string
	Binding binding
	OK      bool
	Message string
}

type rawInputDeviceList struct {
	HDevice uintptr
	DwType  uint32
	_       uint32
}

type hidpCaps struct {
	Usage                     uint16
	UsagePage                 uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

type hidAxisSnapshot struct {
	Name         string
	Path         string
	Values       map[uint16]int32
	Updated      time.Time
	LastActivity time.Time
}

type hidInputDevice struct {
	Path      string
	Name      string
	Handle    uintptr
	Preparsed uintptr
	ReportLen int
}

type hidManager struct {
	mu       sync.RWMutex
	states   map[string]*hidAxisSnapshot
	devices  []*hidInputDevice
	wg       sync.WaitGroup
	stopOnce sync.Once
}

var hidAxisUsages = []uint16{0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38}

func hidUsageName(u uint16) string {
	switch u {
	case 0x30:
		return "X"
	case 0x31:
		return "Y"
	case 0x32:
		return "Z"
	case 0x33:
		return "X Rotation (Rx)"
	case 0x34:
		return "Y Rotation (Ry)"
	case 0x35:
		return "Z Rotation (Rz)"
	case 0x36:
		return "Slider"
	case 0x37:
		return "Dial"
	case 0x38:
		return "Wheel"
	default:
		return fmt.Sprintf("Usage 0x%02X", u)
	}
}

func normalizeHIDAxis(current, center, positiveSpan int32) float64 {
	if positiveSpan == 0 {
		return 0
	}
	return clamp(float64(current-center)/float64(positiveSpan), -1, 1)
}

// DirectInput8 is used specifically for the on-foot thumbstick path in v1.6.
// joy.cpl sees VKB X Rotation/Y Rotation through DirectInput even when legacy
// WinMM/Raw HID paths do not expose those axes correctly.
type diGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type diDeviceInstanceW struct {
	DwSize          uint32
	GuidInstance    diGUID
	GuidProduct     diGUID
	DwDevType       uint32
	TszInstanceName [260]uint16
	TszProductName  [260]uint16
	GuidFFDriver    diGUID
	WUsagePage      uint16
	WUsage          uint16
}

type diJoyState struct {
	X, Y, Z, Rx, Ry, Rz int32
	Sliders             [2]int32
	POV                 [4]uint32
	Buttons             [128]byte
}

type diObjectDataFormat struct {
	Pguid   uintptr
	DwOfs   uint32
	DwType  uint32
	DwFlags uint32
}

type diDataFormat struct {
	DwSize     uint32
	DwObjSize  uint32
	DwFlags    uint32
	DwDataSize uint32
	DwNumObjs  uint32
	Rgodf      *diObjectDataFormat
}

type diPropHeader struct {
	DwSize       uint32
	DwHeaderSize uint32
	DwObj        uint32
	DwHow        uint32
}

type diPropRange struct {
	Header diPropHeader
	LMin   int32
	LMax   int32
}

type diDeviceDesc struct {
	GUID diGUID
	Name string
}

type diDevice struct {
	GUID         diGUID
	GUIDString   string
	Name         string
	Ptr          unsafe.Pointer
	LastAxes     [8]int32
	HaveLast     bool
	LastActivity time.Time
}

type diSnapshot struct {
	GUID         string
	Name         string
	Axes         [8]int32
	LastActivity time.Time
}

type directInputManager struct {
	mu      sync.Mutex
	di      unsafe.Pointer
	devices []*diDevice
	hwnd    uintptr
}

var diEnumMu sync.Mutex
var diEnumSink *[]diDeviceDesc
var diEnumErr error
var diEnumCallback = syscall.NewCallback(func(pInst, _ uintptr) uintptr {
	if pInst == 0 {
		return 1
	}
	var inst diDeviceInstanceW
	currentProcess, _, _ := procGetCurrentProcess.Call()
	bytesRead := uintptr(0)
	result, _, callErr := procReadProcessMemory.Call(
		currentProcess,
		pInst,
		uintptr(unsafe.Pointer(&inst)),
		unsafe.Sizeof(inst),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	if result == 0 || bytesRead != unsafe.Sizeof(inst) {
		diEnumMu.Lock()
		if result == 0 {
			diEnumErr = fmt.Errorf("ReadProcessMemory failed: %v", callErr)
		} else {
			diEnumErr = fmt.Errorf("ReadProcessMemory copied %d of %d bytes", bytesRead, unsafe.Sizeof(inst))
		}
		diEnumMu.Unlock()
		return 0
	}
	d := diDeviceDesc{GUID: inst.GuidInstance, Name: strings.TrimSpace(syscall.UTF16ToString(inst.TszProductName[:]))}
	if d.Name == "" {
		d.Name = strings.TrimSpace(syscall.UTF16ToString(inst.TszInstanceName[:]))
	}
	diEnumMu.Lock()
	if diEnumSink != nil {
		*diEnumSink = append(*diEnumSink, d)
	}
	diEnumMu.Unlock()
	return 1 // DIENUM_CONTINUE
})

var diAxisNames = []string{"X", "Y", "Z", "X Rotation (Rx)", "Y Rotation (Ry)", "Z Rotation (Rz)", "Slider 1", "Slider 2"}

// Standard DirectInput object GUIDs. Binding the data format to these exact
// object types prevents a generic "axis slot" from being mislabeled as Rx/Ry/Rz.
var (
	diGUIDXAxis  = diGUID{Data1: 0xA36D02E0, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
	diGUIDYAxis  = diGUID{Data1: 0xA36D02E1, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
	diGUIDZAxis  = diGUID{Data1: 0xA36D02E2, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
	diGUIDRzAxis = diGUID{Data1: 0xA36D02E3, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
	diGUIDSlider = diGUID{Data1: 0xA36D02E4, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
	diGUIDButton = diGUID{Data1: 0xA36D02F0, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
	diGUIDPOV    = diGUID{Data1: 0xA36D02F2, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
	diGUIDRxAxis = diGUID{Data1: 0xA36D02F4, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
	diGUIDRyAxis = diGUID{Data1: 0xA36D02F5, Data2: 0xC9F3, Data3: 0x11CF, Data4: [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
)

func diGUIDString(g diGUID) string {
	return fmt.Sprintf("%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		g.Data1, g.Data2, g.Data3, g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3], g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

func comMethod(iface unsafe.Pointer, slot int) uintptr {
	if iface == nil {
		return 0
	}
	vtbl := *(*unsafe.Pointer)(iface)
	if vtbl == nil {
		return 0
	}
	methodOffset := uintptr(slot) * unsafe.Sizeof(uintptr(0))
	return *(*uintptr)(unsafe.Add(vtbl, methodOffset))
}

func comCall(iface unsafe.Pointer, slot int, args ...uintptr) uintptr {
	fn := comMethod(iface, slot)
	if fn == 0 {
		return ^uintptr(0)
	}
	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, uintptr(iface))
	callArgs = append(callArgs, args...)
	r1, _, _ := syscall.SyscallN(fn, callArgs...)
	return r1
}

func buildDIDataFormat() ([]diObjectDataFormat, diDataFormat) {
	const (
		didftAnyInstance = 0x00FFFF00
		didftOptional    = 0x80000000
		didftAxis        = 0x00000003
		didftButton      = 0x0000000C
		didftPOV         = 0x00000010
		didfAbsAxis      = 0x00000001
	)
	axisType := uint32(didftAxis | didftAnyInstance | didftOptional)
	povType := uint32(didftPOV | didftAnyInstance | didftOptional)
	buttonType := uint32(didftButton | didftAnyInstance | didftOptional)
	objs := make([]diObjectDataFormat, 0, 140)
	// These offsets match diJoyState/DIJOYSTATE2's position fields exactly.
	axisDefs := []struct {
		guid *diGUID
		ofs  uint32
	}{
		{&diGUIDXAxis, 0}, {&diGUIDYAxis, 4}, {&diGUIDZAxis, 8},
		{&diGUIDRxAxis, 12}, {&diGUIDRyAxis, 16}, {&diGUIDRzAxis, 20},
		{&diGUIDSlider, 24}, {&diGUIDSlider, 28},
	}
	for _, d := range axisDefs {
		objs = append(objs, diObjectDataFormat{Pguid: uintptr(unsafe.Pointer(d.guid)), DwOfs: d.ofs, DwType: axisType})
	}
	for _, ofs := range []uint32{32, 36, 40, 44} {
		objs = append(objs, diObjectDataFormat{Pguid: uintptr(unsafe.Pointer(&diGUIDPOV)), DwOfs: ofs, DwType: povType})
	}
	for i := 0; i < 128; i++ {
		objs = append(objs, diObjectDataFormat{DwOfs: uint32(48 + i), DwType: buttonType})
	}
	fmtv := diDataFormat{
		DwSize: uint32(unsafe.Sizeof(diDataFormat{})), DwObjSize: uint32(unsafe.Sizeof(diObjectDataFormat{})),
		DwFlags: didfAbsAxis, DwDataSize: uint32(unsafe.Sizeof(diJoyState{})), DwNumObjs: uint32(len(objs)), Rgodf: &objs[0],
	}
	return objs, fmtv
}

func (m *directInputManager) Start(hwnd uintptr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
	m.hwnd = hwnd
	if err := procDirectInput8Create.Find(); err != nil {
		return fmt.Errorf("dinput8.dll unavailable: %v", err)
	}
	if err := procGetCurrentProcess.Find(); err != nil {
		return fmt.Errorf("kernel32!GetCurrentProcess unavailable: %v", err)
	}
	if err := procReadProcessMemory.Find(); err != nil {
		return fmt.Errorf("kernel32!ReadProcessMemory unavailable: %v", err)
	}
	hinst, _, _ := procGetModuleHandleW.Call(0)
	iid := diGUID{Data1: 0xBF798031, Data2: 0x483A, Data3: 0x4DA2, Data4: [8]byte{0xAA, 0x99, 0x5D, 0x64, 0xED, 0x36, 0x97, 0x00}}
	var di unsafe.Pointer
	hr, _, _ := procDirectInput8Create.Call(hinst, 0x0800, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&di)), 0)
	if uint32(hr) != 0 || di == nil {
		return fmt.Errorf("DirectInput8Create failed (0x%08X)", uint32(hr))
	}
	m.di = di

	descs := []diDeviceDesc{}
	diEnumMu.Lock()
	diEnumSink = &descs
	diEnumErr = nil
	diEnumMu.Unlock()
	// IDirectInput8::EnumDevices slot 4; GAMECTRL=4, ATTACHEDONLY=1.
	comCall(m.di, 4, 4, diEnumCallback, 0, 1)
	diEnumMu.Lock()
	enumErr := diEnumErr
	diEnumErr = nil
	diEnumSink = nil
	diEnumMu.Unlock()
	if enumErr != nil {
		m.stopLocked()
		return fmt.Errorf("DirectInput device enumeration failed: %w", enumErr)
	}

	for _, desc := range descs {
		var dev unsafe.Pointer
		hr = comCall(m.di, 3, uintptr(unsafe.Pointer(&desc.GUID)), uintptr(unsafe.Pointer(&dev)), 0)
		if uint32(hr) != 0 || dev == nil {
			continue
		}
		objs, fmtv := buildDIDataFormat()
		hrFmt := comCall(dev, 11, uintptr(unsafe.Pointer(&fmtv)))
		runtime.KeepAlive(objs) // SetDataFormat references the object-format backing array.
		if uint32(hrFmt) != 0 {
			comCall(dev, 2)
			continue
		}
		// Ask DirectInput to normalize every available analog axis to a predictable signed range.
		// DIPROP_RANGE is the special DirectInput property token 4; DIPH_BYOFFSET is 1.
		for _, ofs := range []uint32{0, 4, 8, 12, 16, 20, 24, 28} {
			pr := diPropRange{Header: diPropHeader{DwSize: uint32(unsafe.Sizeof(diPropRange{})), DwHeaderSize: uint32(unsafe.Sizeof(diPropHeader{})), DwObj: ofs, DwHow: 1}, LMin: -32768, LMax: 32767}
			_ = comCall(dev, 6, 4, uintptr(unsafe.Pointer(&pr)))
		}
		// DISCL_BACKGROUND | DISCL_NONEXCLUSIVE.
		if uint32(comCall(dev, 13, hwnd, 0x00000008|0x00000002)) != 0 {
			comCall(dev, 2)
			continue
		}
		_ = comCall(dev, 7) // Acquire. Retry also happens when polling.
		m.devices = append(m.devices, &diDevice{GUID: desc.GUID, GUIDString: diGUIDString(desc.GUID), Name: desc.Name, Ptr: dev})
	}
	return nil
}

func (m *directInputManager) Restart(hwnd uintptr) error { return m.Start(hwnd) }

func (m *directInputManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *directInputManager) stopLocked() {
	for _, d := range m.devices {
		if d.Ptr != nil {
			comCall(d.Ptr, 8) // Unacquire
			comCall(d.Ptr, 2) // Release
		}
	}
	m.devices = nil
	if m.di != nil {
		comCall(m.di, 2)
		m.di = nil
	}
}

func diAxes(st diJoyState) [8]int32 {
	return [8]int32{st.X, st.Y, st.Z, st.Rx, st.Ry, st.Rz, st.Sliders[0], st.Sliders[1]}
}

func (m *directInputManager) stateLocked(d *diDevice) (diJoyState, bool) {
	var st diJoyState
	if d == nil || d.Ptr == nil {
		return st, false
	}
	_ = comCall(d.Ptr, 25) // Poll
	hr := comCall(d.Ptr, 9, uintptr(unsafe.Sizeof(st)), uintptr(unsafe.Pointer(&st)))
	if uint32(hr) != 0 {
		_ = comCall(d.Ptr, 7) // Acquire again after focus/device changes.
		_ = comCall(d.Ptr, 25)
		hr = comCall(d.Ptr, 9, uintptr(unsafe.Sizeof(st)), uintptr(unsafe.Pointer(&st)))
	}
	return st, uint32(hr) == 0
}

func (m *directInputManager) State(guid string) ([8]int32, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.devices {
		if strings.EqualFold(d.GUIDString, guid) {
			st, ok := m.stateLocked(d)
			if !ok {
				return [8]int32{}, false
			}
			axes := diAxes(st)
			m.noteActivityLocked(d, axes)
			return axes, true
		}
	}
	return [8]int32{}, false
}

func (m *directInputManager) FullState(guid string) (diJoyState, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.devices {
		if strings.EqualFold(d.GUIDString, guid) {
			st, ok := m.stateLocked(d)
			if ok {
				m.noteActivityLocked(d, diAxes(st))
			}
			return st, d.Name, ok
		}
	}
	return diJoyState{}, "", false
}

type diFullSnapshot struct {
	GUID  string
	Name  string
	State diJoyState
}

func (m *directInputManager) FullSnapshots() []diFullSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]diFullSnapshot, 0, len(m.devices))
	for _, d := range m.devices {
		st, ok := m.stateLocked(d)
		if !ok {
			continue
		}
		m.noteActivityLocked(d, diAxes(st))
		out = append(out, diFullSnapshot{GUID: d.GUIDString, Name: d.Name, State: st})
	}
	return out
}

func (m *directInputManager) noteActivityLocked(d *diDevice, axes [8]int32) {
	if d.HaveLast {
		changed := false
		for i := range axes {
			if axes[i] != d.LastAxes[i] {
				changed = true
				break
			}
		}
		if changed {
			d.LastActivity = time.Now()
		}
	} else {
		d.LastActivity = time.Now()
		d.HaveLast = true
	}
	d.LastAxes = axes
}

func (m *directInputManager) Snapshots() []diSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]diSnapshot, 0, len(m.devices))
	for _, d := range m.devices {
		st, ok := m.stateLocked(d)
		if !ok {
			continue
		}
		axes := diAxes(st)
		m.noteActivityLocked(d, axes)
		out = append(out, diSnapshot{GUID: d.GUIDString, Name: d.Name, Axes: axes, LastActivity: d.LastActivity})
	}
	return out
}

func (m *directInputManager) MostActive() (diSnapshot, bool) {
	snaps := m.Snapshots()
	if len(snaps) == 0 {
		return diSnapshot{}, false
	}
	best := snaps[0]
	for _, s := range snaps[1:] {
		if s.LastActivity.After(best.LastActivity) {
			best = s
		}
	}
	return best, true
}

type diChoice struct {
	GUID string
	Name string
}

func (m *directInputManager) DeviceList() []diChoice {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]diChoice, 0, len(m.devices))
	for _, d := range m.devices {
		out = append(out, diChoice{GUID: d.GUIDString, Name: d.Name})
	}
	return out
}

func (m *directInputManager) StateByGUID(guid string) ([8]int32, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.devices {
		if strings.EqualFold(d.GUIDString, guid) {
			st, ok := m.stateLocked(d)
			if !ok {
				return [8]int32{}, d.Name, false
			}
			axes := diAxes(st)
			m.noteActivityLocked(d, axes)
			return axes, d.Name, true
		}
	}
	return [8]int32{}, "", false
}

func normalizeDIAbsoluteAxis(v int32, invert bool) float64 {
	// v1.96 asks DirectInput to expose analog axes as -32768..32767.
	var n float64
	if v < 0 {
		n = float64(v) / 32768.0
	} else {
		n = float64(v) / 32767.0
	}
	n = clamp(n, -1, 1)
	if invert {
		n = -n
	}
	return n
}

func normalizeDIAxis(current, center, positiveSpan int32) float64 {
	if positiveSpan == 0 {
		return 0
	}
	return clamp(float64(current-center)/float64(positiveSpan), -1, 1)
}

func (a *appState) detectDirectInputAxis(prompt, excludeGUID string, excludeAxis int) (guid, name string, axis int, center, span int32, score float64, ok bool) {
	a.postStatus(prompt)
	time.Sleep(180 * time.Millisecond)
	bases := a.di.Snapshots()
	if len(bases) == 0 {
		return "", "", 0, 0, 0, 0, false
	}
	baseMap := make(map[string][8]int32)
	nameMap := make(map[string]string)
	for _, s := range bases {
		baseMap[s.GUID] = s.Axes
		nameMap[s.GUID] = s.Name
	}
	deadline := time.Now().Add(6 * time.Second)
	bestScore := 0.0
	var bestGUID, bestName string
	bestAxis := 0
	var bestCenter, bestSpan int32
	for time.Now().Before(deadline) {
		for _, st := range a.di.Snapshots() {
			base, exists := baseMap[st.GUID]
			if !exists {
				baseMap[st.GUID] = st.Axes
				nameMap[st.GUID] = st.Name
				continue
			}
			for i, cur := range st.Axes {
				if strings.EqualFold(st.GUID, excludeGUID) && i == excludeAxis {
					continue
				}
				delta := int64(cur) - int64(base[i])
				// DI joystick axes are normally 0..65535. Use a fixed half-range score.
				sc := math.Abs(float64(delta)) / 32768.0
				if sc > bestScore {
					bestScore = sc
					bestGUID = st.GUID
					bestName = st.Name
					bestAxis = i
					bestCenter = base[i]
					bestSpan = int32(delta)
				}
			}
		}
		if bestScore >= 0.45 {
			break
		}
		time.Sleep(12 * time.Millisecond)
	}
	return bestGUID, bestName, bestAxis, bestCenter, bestSpan, bestScore, bestGUID != "" && bestScore >= 0.08 && bestSpan != 0
}

func (a *appState) refreshDirectInputMonitor() {
	h := a.controls[idDIThumbMonitor]
	if h == 0 {
		return
	}
	if vis, _, _ := procIsWindowVisible.Call(h); vis == 0 {
		return
	}
	if a.mappingActive.Load() {
		setText(h, "DirectInput axis monitor paused while game mapping is active.")
		return
	}
	st, ok := a.di.MostActive()
	if !ok {
		setText(h, "DirectInput axis monitor: no game-controller devices available.")
		return
	}
	parts := make([]string, 0, 8)
	for i, v := range st.Axes {
		parts = append(parts, fmt.Sprintf("%s %d", diAxisNames[i], v))
	}
	setText(h, fmt.Sprintf("DirectInput: %s | %s", st.Name, strings.Join(parts, "   ")))
}

func formatDIAxis5(axes [8]int32) string {
	vals := make([]string, 0, 5)
	for _, idx := range []int{3, 4, 5, 6, 7} {
		vals = append(vals, fmt.Sprintf("%s %+.2f", diAxisNames[idx], normalizeDIAbsoluteAxis(axes[idx], false)))
	}
	return strings.Join(vals, "  ")
}

func (a *appState) refreshHotasDirectInputMonitor() {
	h := a.controls[idHotasDIAxisMonitor]
	if h == 0 {
		return
	}
	a.cfgMu.RLock()
	c := a.cfg
	a.cfgMu.RUnlock()
	if !strings.EqualFold(normalizeControlMode(c.ControlMode), "HOTAS") {
		return
	}
	if a.mappingActive.Load() {
		setText(h, "DirectInput axes: live Rx/Ry/Rz/Slider values paused while Xbox output is active.")
		return
	}
	if normalizeInputBackend(c.InputBackend) == "WinMM" {
		setText(h, "DirectInput axes: select Auto or DirectInput to view Rx/Ry/Rz/Slider values here.")
		return
	}
	parts := []string{}
	if c.HotasStickDIGUID != "" {
		if ax, name, ok := a.di.StateByGUID(c.HotasStickDIGUID); ok {
			parts = append(parts, "Stick "+name+": "+formatDIAxis5(ax))
		}
	}
	if c.HotasThrottleDIGUID != "" && !strings.EqualFold(c.HotasThrottleDIGUID, c.HotasStickDIGUID) {
		if ax, name, ok := a.di.StateByGUID(c.HotasThrottleDIGUID); ok {
			parts = append(parts, "Throttle "+name+": "+formatDIAxis5(ax))
		}
	}
	if len(parts) == 0 {
		setText(h, "DirectInput axes: select a Traditional HOTAS stick/throttle device to view Rx/Ry/Rz/Slider values.")
		return
	}
	setText(h, strings.Join(parts, " | "))
}

type xboxBridge struct {
	mu                                                             sync.Mutex
	dll                                                            *syscall.DLL
	alloc, free, connect, disconnect                               *syscall.Proc
	targetAlloc, targetFree, targetAdd, targetRemove, targetUpdate *syscall.Proc
	client                                                         uintptr
	target                                                         uintptr
	ready                                                          bool
}

type appState struct {
	hwnd      uintptr
	font      uintptr
	cfgMu     sync.RWMutex
	cfg       config
	devicesMu sync.RWMutex
	devices   []deviceInfo

	controls         map[int]uintptr
	deviceComboIDs   map[int][]int
	deviceComboGUIDs map[int][]string
	axisNames        []string

	uiReady                bool
	visualsPaused          bool
	mappingActive          atomic.Bool
	mappingStarting        atomic.Bool
	xboxReady              atomic.Bool
	xboxSlot               atomic.Int32
	testUntilMu            sync.RWMutex
	testUntil              time.Time
	cameraTestMu           sync.RWMutex
	cameraTestUntil        time.Time
	cameraTestValue        float64
	monitorMu              sync.RWMutex
	monitor                monitorState
	overlayVisible         atomic.Bool
	overlayHwnd            uintptr
	overlayClassReady      bool
	overlayLast            overlaySnapshot
	overlayTicks           int
	liveCardActive         atomic.Bool
	liveCardLast           overlaySnapshot
	keyboardMu             sync.Mutex
	keyboardDown           map[uint16]bool
	tooltipHwnd            uintptr
	tooltipTexts           []*uint16
	tooltipHelp            map[uintptr]string
	tooltipPopup           uintptr // legacy field; v1.96 renders help inside the main window
	tooltipHover           uintptr
	tooltipControls        map[int]uintptr
	comboOptions           map[int][]string
	comboSelected          map[int]int
	comboList              uintptr
	comboActiveID          int
	comboHidden            []uintptr // controls temporarily hidden beneath the in-window selector
	baseRects              map[uintptr]rect
	contextVisible         map[uintptr]bool
	controlSections        map[uintptr]int
	activeControlSection   int
	cards                  []uiCard
	checkboxVisible        map[int]uintptr
	valueSliders           []valueSlider
	bgBrush                uintptr
	sidebarBrush           uintptr
	cardBrush              uintptr
	fieldBrush             uintptr
	borderBrush            uintptr
	accentBrush            uintptr
	cyanBrush              uintptr
	closeBrush             uintptr
	borderPen              uintptr
	accentPen              uintptr
	cyanPen                uintptr
	mutedPen               uintptr
	uiColorsReady          bool
	applyingUI             bool
	sliderChanging         bool
	topNavigationID        int
	liveLeftThrust         float64
	liveRightThrust        float64
	sidebarDivider         uintptr
	sidebarTitle           uintptr
	sidebarFooter          uintptr
	sidebarNavigation      []uintptr
	verticalBar            *customScrollBar
	horizontalBar          *customScrollBar
	scrollBars             map[uintptr]*customScrollBar
	scrollBarClassReady    bool
	childClips             map[uintptr]rect
	scrollX                int
	scrollY                int
	contentWidth           int
	contentHeight          int
	controlHOSASGroup      []uintptr
	controlHOTASGroup      []uintptr
	hotasBrakeDigitalGroup []uintptr
	hotasBrakeAnalogGroup  []uintptr
	hosasDrivingGroup      []uintptr
	pedalFineGroup         []uintptr
	pedalCameraGroup       []uintptr
	pedalKeyGroup          []uintptr
	twistLeftFineGroup     []uintptr
	twistLeftCameraGroup   []uintptr
	twistLeftKeyGroup      []uintptr
	twistRightFineGroup    []uintptr
	twistRightCameraGroup  []uintptr
	twistRightKeyGroup     []uintptr
	onFootThumbGroup       []uintptr
	onFootButtonGroup      []uintptr

	xbox     xboxBridge
	hid      hidManager
	di       directInputManager
	stopCh   chan struct{}
	stopOnce sync.Once
	pollWG   sync.WaitGroup
	winmmMu  sync.Mutex

	pendingMu   sync.Mutex
	statusQueue []string
	detectQueue []detectResult
	learnQueue  []learnResult
}

var app *appState

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	winmm    = syscall.NewLazyDLL("winmm.dll")
	hiddll   = syscall.NewLazyDLL("hid.dll")
	dinput8  = syscall.NewLazyDLL("dinput8.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procRegisterClassExW         = user32.NewProc("RegisterClassExW")
	procCreateWindowExW          = user32.NewProc("CreateWindowExW")
	procDefWindowProcW           = user32.NewProc("DefWindowProcW")
	procShowWindow               = user32.NewProc("ShowWindow")
	procUpdateWindow             = user32.NewProc("UpdateWindow")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procSetWindowTextW           = user32.NewProc("SetWindowTextW")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procBeginDeferWindowPos      = user32.NewProc("BeginDeferWindowPos")
	procDeferWindowPos           = user32.NewProc("DeferWindowPos")
	procEndDeferWindowPos        = user32.NewProc("EndDeferWindowPos")
	procRedrawWindow             = user32.NewProc("RedrawWindow")
	procSetTimer                 = user32.NewProc("SetTimer")
	procKillTimer                = user32.NewProc("KillTimer")
	procKeybdEvent               = user32.NewProc("keybd_event")
	procLoadCursorW              = user32.NewProc("LoadCursorW")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procIsZoomed                 = user32.NewProc("IsZoomed")
	procScreenToClient           = user32.NewProc("ScreenToClient")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procSetFocus                 = user32.NewProc("SetFocus")
	procGetDlgCtrlID             = user32.NewProc("GetDlgCtrlID")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentProcess        = kernel32.NewProc("GetCurrentProcess")
	procReadProcessMemory        = kernel32.NewProc("ReadProcessMemory")
	procGetStockObject           = gdi32.NewProc("GetStockObject")
	procCreateSolidBrush         = gdi32.NewProc("CreateSolidBrush")
	procCreatePen                = gdi32.NewProc("CreatePen")
	procDeleteObject             = gdi32.NewProc("DeleteObject")
	procSelectObject             = gdi32.NewProc("SelectObject")
	procSetTextColor             = gdi32.NewProc("SetTextColor")
	procSetBkColor               = gdi32.NewProc("SetBkColor")
	procSetBkMode                = gdi32.NewProc("SetBkMode")
	procFillRect                 = user32.NewProc("FillRect")
	procRoundRect                = gdi32.NewProc("RoundRect")
	procDrawTextW                = user32.NewProc("DrawTextW")
	procDrawFocusRect            = user32.NewProc("DrawFocusRect")
	procMoveToEx                 = gdi32.NewProc("MoveToEx")
	procLineTo                   = gdi32.NewProc("LineTo")
	procEllipse                  = gdi32.NewProc("Ellipse")
	procBeginPaint               = user32.NewProc("BeginPaint")
	procEndPaint                 = user32.NewProc("EndPaint")
	procInitCommonControlsEx     = comctl32.NewProc("InitCommonControlsEx")

	// WinMM device enumeration/capability probing is intentionally isolated in
	// GalacticRacerWinMMHelper.exe. The main process only polls known-good IDs.
	procJoyGetPosEx = winmm.NewProc("joyGetPosEx")

	procGetRawInputDeviceList  = user32.NewProc("GetRawInputDeviceList")
	procGetRawInputDeviceInfoW = user32.NewProc("GetRawInputDeviceInfoW")
	procCreateFileW            = kernel32.NewProc("CreateFileW")
	procReadFile               = kernel32.NewProc("ReadFile")
	procCloseHandle            = kernel32.NewProc("CloseHandle")
	procCancelIoEx             = kernel32.NewProc("CancelIoEx")
	procHidDGetPreparsedData   = hiddll.NewProc("HidD_GetPreparsedData")
	procHidDFreePreparsedData  = hiddll.NewProc("HidD_FreePreparsedData")
	procHidDGetProductString   = hiddll.NewProc("HidD_GetProductString")
	procHidPGetCaps            = hiddll.NewProc("HidP_GetCaps")
	procHidPGetUsageValue      = hiddll.NewProc("HidP_GetUsageValue")
	procDirectInput8Create     = dinput8.NewProc("DirectInput8Create")
	procShellExecuteW          = shell32.NewProc("ShellExecuteW")
)

var wndProcCallback = syscall.NewCallback(wndProc)

var buttonOutputs = []struct {
	Key, Label string
	Mask       uint16 // Xbox button mask; zero for keyboard-only outputs.
	VK         uint16 // Windows virtual key; zero for Xbox-only outputs.
}{
	{"A", "Xbox A", XUSB_A, 0}, {"B", "Xbox B", XUSB_B, 0},
	{"X", "Xbox X", XUSB_X, 0}, {"Y", "Xbox Y", XUSB_Y, 0},
	{"LB", "Left bumper", XUSB_LEFT_SHOULDER, 0}, {"RB", "Right bumper", XUSB_RIGHT_SHOULDER, 0},
	{"LT", "Left Trigger", 0, 0}, {"RT", "Right Trigger", 0, 0},
	{"BACK", "Back / View", XUSB_BACK, 0}, {"START", "Start / Menu", XUSB_START, 0},
	{"LTHUMB", "Left-stick click", XUSB_LEFT_THUMB, 0}, {"RTHUMB", "Right-stick click", XUSB_RIGHT_THUMB, 0},
	{"DUP", "D-pad Up", XUSB_DPAD_UP, 0}, {"DDOWN", "D-pad Down", XUSB_DPAD_DOWN, 0},
	{"DLEFT", "D-pad Left", XUSB_DPAD_LEFT, 0}, {"DRIGHT", "D-pad Right", XUSB_DPAD_RIGHT, 0},
	// Galactic Racer labels describe the equivalent Xbox control; Look outputs remain keyboard keys.
	{"PITCHUP", "Pitch Up / Left Stick ↑", 0, 0},
	{"PITCHDOWN", "Pitch Down / Left Stick ↓", 0, 0},
	{"LOOKLEFT", "Look Left / Right Stick ←", 0, VK_DELETE},
	{"LOOKRIGHT", "Look Right / Right Stick →", 0, VK_NEXT},
	{"LOOKBACK", "Look Back / Right Stick ↓", 0, VK_END},
}

var wasdButtonDefs = []struct {
	Key, Label string
	VK         uint16
}{
	{"WASD_W", "W", uint16('W')},
	{"WASD_A", "A", uint16('A')},
	{"WASD_S", "S", uint16('S')},
	{"WASD_D", "D", uint16('D')},
}

var keyboardKeyOptions = func() []keyOption {
	out := []keyOption{
		{"Delete", 0x2E}, {"Page Down", 0x22}, {"End", 0x23},
		{"Left Arrow", 0x25}, {"Right Arrow", 0x27}, {"Up Arrow", 0x26}, {"Down Arrow", 0x28},
		{"Page Up", 0x21}, {"Home", 0x24}, {"Insert", 0x2D}, {"Space", 0x20},
		{"Tab", 0x09}, {"Shift", 0x10}, {"Left Shift", 0xA0}, {"Right Shift", 0xA1},
		{"Left Control", 0xA2}, {"Right Control", 0xA3},
		{"Alt", 0x12}, {"Left Alt", 0xA4}, {"Right Alt", 0xA5},
		{"Enter", 0x0D}, {"Backspace", 0x08},
	}
	for ch := 'A'; ch <= 'Z'; ch++ {
		out = append(out, keyOption{Name: string(ch), VK: int(ch)})
	}
	for ch := '0'; ch <= '9'; ch++ {
		out = append(out, keyOption{Name: string(ch), VK: int(ch)})
	}
	for i := 1; i <= 12; i++ {
		out = append(out, keyOption{Name: fmt.Sprintf("F%d", i), VK: 0x6F + i})
	}
	for i := 0; i <= 9; i++ {
		out = append(out, keyOption{Name: fmt.Sprintf("Numpad %d", i), VK: 0x60 + i})
	}
	return out
}()

func utf16ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func loword(v uintptr) int { return int(v & 0xffff) }
func hiword(v uintptr) int { return int((v >> 16) & 0xffff) }
func signedLowWord(v uintptr) int32 {
	return int32(int16(uint16(v)))
}
func signedHighWord(v uintptr) int32 {
	return int32(int16(uint16(v >> 16)))
}

type initCommonControlsEx struct {
	DwSize uint32
	DwICC  uint32
}

type rect struct{ Left, Top, Right, Bottom int32 }

func customFrameHitTest(bounds rect, cursor point, maximized bool, borderX, borderY int32) uintptr {
	if cursor.X < bounds.Left || cursor.X >= bounds.Right || cursor.Y < bounds.Top || cursor.Y >= bounds.Bottom {
		return HTNOWHERE
	}
	if !maximized {
		left := cursor.X < bounds.Left+borderX
		right := cursor.X >= bounds.Right-borderX
		top := cursor.Y < bounds.Top+borderY
		bottom := cursor.Y >= bounds.Bottom-borderY
		switch {
		case left && top:
			return HTTOPLEFT
		case right && top:
			return HTTOPRIGHT
		case left && bottom:
			return HTBOTTOMLEFT
		case right && bottom:
			return HTBOTTOMRIGHT
		case left:
			return HTLEFT
		case right:
			return HTRIGHT
		case top:
			return HTTOP
		case bottom:
			return HTBOTTOM
		}
	}
	captionButtonLeft := bounds.Right - captionButtonInset - 3*captionButtonWidth
	captionButtonRight := bounds.Right - captionButtonInset
	captionButtonTop := bounds.Top + (customTitleBarHeight-captionButtonHeight)/2
	captionButtonBottom := captionButtonTop + captionButtonHeight
	if cursor.X >= captionButtonLeft && cursor.X < captionButtonRight &&
		cursor.Y >= captionButtonTop && cursor.Y < captionButtonBottom {
		return HTCLIENT
	}
	if cursor.Y < bounds.Top+customTitleBarHeight {
		return HTCAPTION
	}
	return HTCLIENT
}

type uiCard struct {
	title   string
	bounds  rect
	section int
	profile string
	curve   bool
	meter   string
	live    bool
}

type drawItem struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	HDC        uintptr
	RcItem     rect
	ItemData   uintptr
}

type paintStruct struct {
	HDC       uintptr
	Erase     int32
	PaintRect rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type toolInfo struct {
	CbSize     uint32
	UFlags     uint32
	Hwnd       uintptr
	UId        uintptr
	Rect       rect
	Hinst      uintptr
	LpszText   *uint16
	LParam     uintptr
	LpReserved uintptr
}

var pollingRateOptions = []int{100, 200, 250, 500, 1000}

func validPollingHz(hz int) bool {
	for _, v := range pollingRateOptions {
		if hz == v {
			return true
		}
	}
	return false
}
func pollingInterval(hz int) time.Duration {
	if !validPollingHz(hz) {
		hz = 500
	}
	return time.Second / time.Duration(hz)
}

func iconFromICO(data []byte, desired int) uintptr {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return 0
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	bestOff, bestSize, bestDiff := 0, 0, int(^uint(0)>>1)
	for i := 0; i < count; i++ {
		pos := 6 + i*16
		if pos+16 > len(data) {
			break
		}
		w := int(data[pos])
		if w == 0 {
			w = 256
		}
		h := int(data[pos+1])
		if h == 0 {
			h = 256
		}
		sz := int(binary.LittleEndian.Uint32(data[pos+8 : pos+12]))
		off := int(binary.LittleEndian.Uint32(data[pos+12 : pos+16]))
		if off < 0 || sz <= 0 || off+sz > len(data) {
			continue
		}
		diff := int(math.Abs(float64(w-desired)) + math.Abs(float64(h-desired)))
		if diff < bestDiff {
			bestDiff, bestOff, bestSize = diff, off, sz
		}
	}
	if bestSize == 0 {
		return 0
	}
	img := data[bestOff : bestOff+bestSize]
	h, _, _ := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&img[0])), uintptr(len(img)), 1, 0x00030000,
		uintptr(desired), uintptr(desired), 0,
	)
	return h
}

func (h *hidManager) Start() {
	h.mu.Lock()
	if h.states == nil {
		h.states = make(map[string]*hidAxisSnapshot)
	}
	h.mu.Unlock()

	var count uint32
	itemSize := uint32(unsafe.Sizeof(rawInputDeviceList{}))
	r, _, _ := procGetRawInputDeviceList.Call(0, uintptr(unsafe.Pointer(&count)), uintptr(itemSize))
	if uint32(r) == 0xFFFFFFFF || count == 0 {
		return
	}
	list := make([]rawInputDeviceList, count)
	r, _, _ = procGetRawInputDeviceList.Call(uintptr(unsafe.Pointer(&list[0])), uintptr(unsafe.Pointer(&count)), uintptr(itemSize))
	if uint32(r) == 0xFFFFFFFF {
		return
	}
	for _, ri := range list[:count] {
		if ri.DwType != RIM_TYPEHID || ri.HDevice == 0 {
			continue
		}
		path := rawInputDevicePath(ri.HDevice)
		if path == "" {
			continue
		}
		handle, _, _ := procCreateFileW.Call(
			uintptr(unsafe.Pointer(utf16ptr(path))),
			GENERIC_READ,
			FILE_SHARE_READ|FILE_SHARE_WRITE,
			0,
			OPEN_EXISTING,
			0,
			0,
		)
		if handle == 0 || handle == ^uintptr(0) {
			continue
		}
		var prep uintptr
		ok, _, _ := procHidDGetPreparsedData.Call(handle, uintptr(unsafe.Pointer(&prep)))
		if ok == 0 || prep == 0 {
			procCloseHandle.Call(handle)
			continue
		}
		var caps hidpCaps
		status, _, _ := procHidPGetCaps.Call(prep, uintptr(unsafe.Pointer(&caps)))
		if uint32(status) != HIDP_STATUS_SUCCESS || caps.UsagePage != 0x01 || (caps.Usage != 0x04 && caps.Usage != 0x05 && caps.Usage != 0x08) || caps.InputReportByteLength == 0 {
			procHidDFreePreparsedData.Call(prep)
			procCloseHandle.Call(handle)
			continue
		}
		name := hidProductName(handle)
		if name == "" {
			name = rawHIDShortName(path)
		}
		dev := &hidInputDevice{Path: path, Name: name, Handle: handle, Preparsed: prep, ReportLen: int(caps.InputReportByteLength)}
		h.mu.Lock()
		h.devices = append(h.devices, dev)
		h.states[path] = &hidAxisSnapshot{Name: name, Path: path, Values: make(map[uint16]int32)}
		h.mu.Unlock()
		h.wg.Add(1)
		go h.readLoop(dev)
	}
}

func (h *hidManager) Stop() {
	h.stopOnce.Do(func() {
		h.mu.RLock()
		devs := append([]*hidInputDevice(nil), h.devices...)
		h.mu.RUnlock()
		for _, d := range devs {
			if d.Handle != 0 && d.Handle != ^uintptr(0) {
				procCancelIoEx.Call(d.Handle, 0)
				procCloseHandle.Call(d.Handle)
			}
		}
		h.wg.Wait()
		for _, d := range devs {
			if d.Preparsed != 0 {
				procHidDFreePreparsedData.Call(d.Preparsed)
			}
		}
	})
}

func rawInputDevicePath(hDevice uintptr) string {
	var chars uint32
	procGetRawInputDeviceInfoW.Call(hDevice, RIDI_DEVICENAME, 0, uintptr(unsafe.Pointer(&chars)))
	if chars == 0 {
		return ""
	}
	buf := make([]uint16, int(chars)+2)
	r, _, _ := procGetRawInputDeviceInfoW.Call(hDevice, RIDI_DEVICENAME, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&chars)))
	if int32(r) < 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func hidProductName(handle uintptr) string {
	buf := make([]uint16, 256)
	r, _, _ := procHidDGetProductString.Call(handle, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2))
	if r == 0 {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func rawHIDShortName(path string) string {
	up := strings.ToUpper(path)
	vid, pid := "", ""
	if i := strings.Index(up, "VID_"); i >= 0 && i+8 <= len(up) {
		vid = up[i+4 : i+8]
	}
	if i := strings.Index(up, "PID_"); i >= 0 && i+8 <= len(up) {
		pid = up[i+4 : i+8]
	}
	if vid != "" || pid != "" {
		return fmt.Sprintf("HID VID_%s PID_%s", vid, pid)
	}
	return "Raw HID joystick"
}

func (h *hidManager) readLoop(d *hidInputDevice) {
	defer h.wg.Done()
	buf := make([]byte, d.ReportLen)
	for {
		var n uint32
		r, _, _ := procReadFile.Call(d.Handle, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)), 0)
		if r == 0 || n == 0 {
			return
		}
		values := make(map[uint16]int32)
		for _, usage := range hidAxisUsages {
			var v uint32
			status, _, _ := procHidPGetUsageValue.Call(
				HIDP_INPUT,
				0x01,
				0,
				uintptr(usage),
				uintptr(unsafe.Pointer(&v)),
				d.Preparsed,
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(n),
			)
			if uint32(status) == HIDP_STATUS_SUCCESS {
				values[usage] = int32(v)
			}
		}
		if len(values) == 0 {
			continue
		}
		now := time.Now()
		h.mu.Lock()
		st := h.states[d.Path]
		if st == nil {
			st = &hidAxisSnapshot{Name: d.Name, Path: d.Path, Values: make(map[uint16]int32)}
			h.states[d.Path] = st
		}
		changed := false
		for usage, v := range values {
			if old, ok := st.Values[usage]; !ok || old != v {
				changed = true
			}
			st.Values[usage] = v
		}
		st.Updated = now
		if changed {
			st.LastActivity = now
		}
		h.mu.Unlock()
	}
}

func (h *hidManager) snapshot() map[string]hidAxisSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]hidAxisSnapshot, len(h.states))
	for path, st := range h.states {
		cp := hidAxisSnapshot{Name: st.Name, Path: st.Path, Updated: st.Updated, LastActivity: st.LastActivity, Values: make(map[uint16]int32, len(st.Values))}
		for u, v := range st.Values {
			cp.Values[u] = v
		}
		out[path] = cp
	}
	return out
}

func (h *hidManager) axisValue(path string, usage uint16) (int32, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	st := h.states[path]
	if st == nil {
		return 0, false
	}
	v, ok := st.Values[usage]
	return v, ok
}

func (h *hidManager) mostActive() (hidAxisSnapshot, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var best *hidAxisSnapshot
	for _, st := range h.states {
		if len(st.Values) == 0 {
			continue
		}
		if best == nil || st.LastActivity.After(best.LastActivity) {
			best = st
		}
	}
	if best == nil {
		return hidAxisSnapshot{}, false
	}
	cp := hidAxisSnapshot{Name: best.Name, Path: best.Path, Updated: best.Updated, LastActivity: best.LastActivity, Values: make(map[uint16]int32, len(best.Values))}
	for u, v := range best.Values {
		cp.Values[u] = v
	}
	return cp, true
}

func (a *appState) detectRawHIDAxis(prompt, excludePath string, excludeUsage uint16) (path, name string, usage uint16, center, span int32, score float64, ok bool) {
	a.postStatus(prompt)
	time.Sleep(180 * time.Millisecond)
	bases := a.hid.snapshot()
	if len(bases) == 0 {
		return "", "", 0, 0, 0, 0, false
	}
	deadline := time.Now().Add(6 * time.Second)
	bestScore := 0.0
	var bestPath, bestName string
	var bestUsage uint16
	var bestCenter, bestSpan int32
	for time.Now().Before(deadline) {
		snaps := a.hid.snapshot()
		for p, st := range snaps {
			base, exists := bases[p]
			if !exists {
				bases[p] = st
				continue
			}
			for u, cur := range st.Values {
				if p == excludePath && u == excludeUsage {
					continue
				}
				b, exists := base.Values[u]
				if !exists {
					base.Values[u] = cur
					bases[p] = base
					continue
				}
				delta := int64(cur) - int64(b)
				denom := math.Max(math.Abs(float64(b)), 256.0)
				sc := math.Abs(float64(delta)) / denom
				if sc > bestScore {
					bestScore = sc
					bestPath, bestName, bestUsage = p, st.Name, u
					bestCenter, bestSpan = b, int32(delta)
				}
			}
		}
		if bestScore >= 0.45 {
			break
		}
		time.Sleep(12 * time.Millisecond)
	}
	return bestPath, bestName, bestUsage, bestCenter, bestSpan, bestScore, bestScore >= 0.08 && bestSpan != 0
}

func (a *appState) refreshRawHIDMonitor() {
	h := a.controls[idRawHIDMonitor]
	if h == 0 {
		return
	}
	if a.mappingActive.Load() {
		setText(h, "Raw HID axis monitor paused while game mapping is active.")
		return
	}
	st, ok := a.hid.mostActive()
	if !ok {
		setText(h, "Raw HID axis monitor: waiting for joystick reports...")
		return
	}
	parts := []string{}
	for _, u := range hidAxisUsages {
		if v, exists := st.Values[u]; exists {
			parts = append(parts, fmt.Sprintf("%s %d", hidUsageName(u), v))
		}
	}
	if len(parts) > 6 {
		parts = parts[:6]
	}
	setText(h, fmt.Sprintf("Raw HID: %s | %s", st.Name, strings.Join(parts, "   ")))
}

func main() {
	runtime.LockOSThread()
	icc := initCommonControlsEx{DwSize: uint32(unsafe.Sizeof(initCommonControlsEx{})), DwICC: 0x000000FF}
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
	app = &appState{
		controls:         make(map[int]uintptr),
		tooltipControls:  make(map[int]uintptr),
		checkboxVisible:  make(map[int]uintptr),
		deviceComboIDs:   make(map[int][]int),
		deviceComboGUIDs: make(map[int][]string),
		baseRects:        make(map[uintptr]rect),
		contextVisible:   make(map[uintptr]bool),
		controlSections:  make(map[uintptr]int),
		contentWidth:     contentOffset + 1130,
		contentHeight:    1740 + layoutShift,
		axisNames:        []string{"X", "Y", "Z", "X Rotation (R)", "Y Rotation (U)", "Z Rotation (V)"},
		tooltipHelp:      make(map[uintptr]string),
		keyboardDown:     make(map[uint16]bool),
		stopCh:           make(chan struct{}),
	}
	app.xboxSlot.Store(-1)
	app.cfg = defaultConfig()
	loadConfig(&app.cfg)
	// v1.96.1: do not probe legacy WinMM joystick slots during normal Auto/DirectInput startup.
	// WinMM device-capability queries are isolated in GalacticRacerWinMMHelper.exe and
	// are only requested when compatibility mode actually needs them.
	app.devices = nil
	if normalizeInputBackend(app.cfg.InputBackend) == "WinMM" {
		app.devices = enumerateDevices()
	}

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(IDC_ARROW))
	bigIcon := iconFromICO(embeddedAppIcon, 256)
	smallIcon := iconFromICO(embeddedAppIcon, 32)
	className := utf16ptr("GalacticRacerHOSASBridgeClass")
	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		LpfnWndProc:   wndProcCallback,
		HInstance:     hInstance,
		HIcon:         bigIcon,
		HCursor:       cursor,
		HbrBackground: 0,
		LpszClassName: className,
		HIconSm:       smallIcon,
	}
	if r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		panic(fmt.Sprintf("RegisterClassExW failed: %v", e))
	}
	style := uintptr(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX | WS_MAXIMIZEBOX | WS_THICKFRAME | WS_CLIPCHILDREN)
	hwnd, _, e := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16ptr(appTitle))), style,
		CW_USEDEFAULT, CW_USEDEFAULT, 1380, 616, 0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		panic(fmt.Sprintf("CreateWindowExW failed: %v", e))
	}
	app.hwnd = hwnd
	if bigIcon != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, bigIcon)
	}
	if smallIcon != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, smallIcon)
	}
	procShowWindow.Call(hwnd, SW_SHOW)
	procUpdateWindow.Call(hwnd)
	if err := app.di.Start(hwnd); err != nil {
		app.postStatus("DirectInput unavailable: " + err.Error())
	} else {
		app.populateCombos()
		app.applyConfigToUI()
		app.updateContextVisibility()
	}
	app.updateScrollbars()
	app.refreshHidHideUI()

	app.pollWG.Add(1)
	go app.pollLoop()

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	app.stopOnce.Do(func() { close(app.stopCh) })
	app.pollWG.Wait()
	app.releaseAllKeyboardOutputs()
	app.xbox.Stop()
	app.di.Stop()
}

func defaultConfig() config {
	return config{
		InputBackend:    "Auto",
		ControlMode:     "HOSAS",
		HotasThrottleID: -1, HotasThrottleAxis: 1, HotasBrakeBehavior: "Digital at Idle", HotasBrakeThreshold: 5, HotasBrakeZone: 20,
		HotasStickID: -1, HotasStickXAxis: 0, HotasStickYAxis: 1,
		LeftID: 0, RightID: 1, PedalID: -1,
		TwistLeft: defaultAuxiliaryAxis(), TwistRight: defaultAuxiliaryAxis(),
		LeftAxis: 1, RightAxis: 1, PedalAxis: 0,
		PedalMode: "Off", Deadzone: 6, ThrottleScale: 100, SteerScale: 100,
		PedalSteerStrength:     30,
		CameraSensitivity:      100,
		CameraDeadzone:         6,
		PedalLeftKey:           VK_DELETE,
		PedalRightKey:          VK_NEXT,
		PedalKeyThreshold:      20,
		ThumbWASDEnabled:       false,
		ThumbWASDDeviceID:      -1,
		ThumbWASDSideAxis:      0,
		ThumbWASDForwardAxis:   1,
		ThumbWASDInvertSide:    false,
		ThumbWASDInvertForward: false,
		ThumbWASDThreshold:     20,
		ThumbWASDUseRawHID:     false,
		OnFootMode:             "Off",
		ThumbDISideAxis:        3,
		ThumbDIForwardAxis:     4,
		PollingHz:              500,
		Bindings:               make(map[string]binding),
	}
}

func configPath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if b, err := os.UserConfigDir(); err == nil {
			base = b
		} else {
			base = "."
		}
	}
	dir := filepath.Join(base, "GalacticRacerHOSAS")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "hosas_config.json")
}

func loadConfig(c *config) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return
	}
	// First try normal decoding. The schema intentionally matches v0.6 field names.
	var tmp config
	if err := json.Unmarshal(data, &tmp); err == nil {
		if tmp.Bindings == nil {
			tmp.Bindings = make(map[string]binding)
		}
		if tmp.Deadzone < 0 || tmp.Deadzone > 40 {
			tmp.Deadzone = 6
		}
		tmp.ThrottleScale = normalizeScalePercent(tmp.ThrottleScale)
		tmp.SteerScale = normalizeSteerScalePercent(tmp.SteerScale)
		tmp.ThrottleExpo = normalizeExpoPercent(tmp.ThrottleExpo)
		tmp.SteerExpo = normalizeExpoPercent(tmp.SteerExpo)
		tmp.SteerExpo2 = normalizeExpoPercent(tmp.SteerExpo2)
		if tmp.PedalSteerStrength <= 0 || tmp.PedalSteerStrength > 100 {
			tmp.PedalSteerStrength = 30
		}
		if tmp.CameraSensitivity <= 0 || tmp.CameraSensitivity > 250 {
			tmp.CameraSensitivity = 100
		}
		if tmp.CameraDeadzone < 0 || tmp.CameraDeadzone > 40 {
			tmp.CameraDeadzone = 6
		}
		if !validKeyboardVK(tmp.PedalLeftKey) {
			tmp.PedalLeftKey = VK_DELETE
		}
		if !validKeyboardVK(tmp.PedalRightKey) {
			tmp.PedalRightKey = VK_NEXT
		}
		if tmp.PedalKeyThreshold < 5 || tmp.PedalKeyThreshold > 95 {
			tmp.PedalKeyThreshold = 20
		}
		// New v1.2 fields need explicit defaults when loading an older config,
		// because missing integer JSON fields otherwise decode as zero.
		var rawKeys map[string]json.RawMessage
		_ = json.Unmarshal(data, &rawKeys)
		if _, ok := rawKeys["input_backend"]; !ok {
			// Preserve the known-good behavior for existing users. New installs default to Auto.
			tmp.InputBackend = "WinMM"
		}
		tmp.InputBackend = normalizeInputBackend(tmp.InputBackend)
		if _, ok := rawKeys["control_mode"]; !ok {
			tmp.ControlMode = "HOSAS"
		}
		tmp.ControlMode = normalizeControlMode(tmp.ControlMode)
		if _, ok := rawKeys["hotas_throttle_id"]; !ok {
			tmp.HotasThrottleID = -1
		}
		if _, ok := rawKeys["hotas_stick_id"]; !ok {
			tmp.HotasStickID = -1
		}
		if _, ok := rawKeys["hotas_brake_behavior"]; !ok {
			tmp.HotasBrakeBehavior = "Digital at Idle"
		}
		tmp.HotasBrakeBehavior = normalizeHotasBrakeBehavior(tmp.HotasBrakeBehavior)
		if _, ok := rawKeys["hotas_brake_threshold"]; !ok {
			tmp.HotasBrakeThreshold = 5
		}
		if tmp.HotasBrakeThreshold < 0 || tmp.HotasBrakeThreshold > 50 {
			tmp.HotasBrakeThreshold = 5
		}
		if _, ok := rawKeys["hotas_brake_zone"]; !ok {
			tmp.HotasBrakeZone = 20
		}
		if tmp.HotasBrakeZone < 1 || tmp.HotasBrakeZone > 50 {
			tmp.HotasBrakeZone = 20
		}
		maxAxis := 7
		if normalizeInputBackend(tmp.InputBackend) == "WinMM" {
			maxAxis = 5
		}
		if tmp.HotasThrottleAxis < 0 || tmp.HotasThrottleAxis > maxAxis {
			tmp.HotasThrottleAxis = 1
		}
		if tmp.HotasStickXAxis < 0 || tmp.HotasStickXAxis > maxAxis {
			tmp.HotasStickXAxis = 0
		}
		if tmp.HotasStickYAxis < 0 || tmp.HotasStickYAxis > maxAxis {
			tmp.HotasStickYAxis = 1
		}
		if tmp.LeftAxis < 0 || tmp.LeftAxis > maxAxis {
			tmp.LeftAxis = 1
		}
		if tmp.RightAxis < 0 || tmp.RightAxis > maxAxis {
			tmp.RightAxis = 1
		}
		if tmp.PedalAxis < 0 || tmp.PedalAxis > maxAxis {
			tmp.PedalAxis = 0
		}
		if _, ok := rawKeys["thumb_wasd_device_id"]; !ok {
			tmp.ThumbWASDDeviceID = -1
		}
		if _, ok := rawKeys["thumb_wasd_side_axis"]; !ok {
			tmp.ThumbWASDSideAxis = 0
		}
		if _, ok := rawKeys["thumb_wasd_forward_axis"]; !ok {
			tmp.ThumbWASDForwardAxis = 1
		}
		if _, ok := rawKeys["on_foot_mode"]; !ok {
			if tmp.ThumbWASDEnabled {
				tmp.OnFootMode = "Thumbstick -> WASD"
			} else {
				tmp.OnFootMode = "Off"
			}
		}
		tmp.OnFootMode = normalizeOnFootMode(tmp.OnFootMode)
		if tmp.ThumbDISideAxis < 0 || tmp.ThumbDISideAxis > 7 {
			tmp.ThumbDISideAxis = 3
		}
		if tmp.ThumbDIForwardAxis < 0 || tmp.ThumbDIForwardAxis > 7 {
			tmp.ThumbDIForwardAxis = 4
		}
		if tmp.ThumbWASDThreshold < 5 || tmp.ThumbWASDThreshold > 95 {
			tmp.ThumbWASDThreshold = 20
		}
		if _, ok := rawKeys["polling_hz"]; !ok {
			tmp.PollingHz = 500
		}
		if !validPollingHz(tmp.PollingHz) {
			tmp.PollingHz = 500
		}
		if tmp.ThumbWASDSideAxis < 0 || tmp.ThumbWASDSideAxis > 5 {
			tmp.ThumbWASDSideAxis = 0
		}
		if tmp.ThumbWASDForwardAxis < 0 || tmp.ThumbWASDForwardAxis > 5 {
			tmp.ThumbWASDForwardAxis = 1
		}
		validateAuxiliaryAxis(&tmp.TwistLeft, rawKeys["twist_left"] != nil, maxAxis)
		validateAuxiliaryAxis(&tmp.TwistRight, rawKeys["twist_right"] != nil, maxAxis)
		tmp.PedalMode = normalizePedalMode(tmp.PedalMode)
		tmp.Bindings = normalizeBindingMap(tmp.Bindings)
		*c = tmp
		return
	}
	// Migration fallback for any old axis values that were saved as names instead of indices.
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return
	}
	d := defaultConfig()
	if b, ok := raw["input_backend"]; ok {
		_ = json.Unmarshal(b, &d.InputBackend)
	} else {
		d.InputBackend = "WinMM"
	}
	d.InputBackend = normalizeInputBackend(d.InputBackend)
	readString(raw, "control_mode", &d.ControlMode)
	d.ControlMode = normalizeControlMode(d.ControlMode)
	readInt(raw, "hotas_throttle_id", &d.HotasThrottleID)
	readString(raw, "hotas_throttle_di_guid", &d.HotasThrottleDIGUID)
	readAxis(raw, "hotas_throttle_axis", &d.HotasThrottleAxis)
	readBool(raw, "hotas_throttle_invert", &d.HotasThrottleInvert)
	readInt(raw, "hotas_stick_id", &d.HotasStickID)
	readString(raw, "hotas_stick_di_guid", &d.HotasStickDIGUID)
	readAxis(raw, "hotas_stick_x_axis", &d.HotasStickXAxis)
	readAxis(raw, "hotas_stick_y_axis", &d.HotasStickYAxis)
	readBool(raw, "hotas_stick_invert_x", &d.HotasStickInvertX)
	readBool(raw, "hotas_stick_invert_y", &d.HotasStickInvertY)
	readInt(raw, "left_id", &d.LeftID)
	readString(raw, "left_di_guid", &d.LeftDIGUID)
	readInt(raw, "right_id", &d.RightID)
	readString(raw, "right_di_guid", &d.RightDIGUID)
	readInt(raw, "pedal_id", &d.PedalID)
	readString(raw, "pedal_di_guid", &d.PedalDIGUID)
	readAxis(raw, "left_axis", &d.LeftAxis)
	readAxis(raw, "right_axis", &d.RightAxis)
	readAxis(raw, "pedal_axis", &d.PedalAxis)
	readBool(raw, "left_invert", &d.LeftInvert)
	readBool(raw, "right_invert", &d.RightInvert)
	readBool(raw, "left_throttle_mode", &d.LeftThrottleMode)
	readBool(raw, "right_throttle_mode", &d.RightThrottleMode)
	readBool(raw, "pedal_invert", &d.PedalInvert)
	readPedalMode(raw, "pedal_mode", &d.PedalMode)
	readDeadzone(raw, "deadzone", &d.Deadzone)
	readInt(raw, "throttle_scale", &d.ThrottleScale)
	readInt(raw, "steer_scale", &d.SteerScale)
	readInt(raw, "throttle_expo", &d.ThrottleExpo)
	readInt(raw, "steer_expo", &d.SteerExpo)
	readInt(raw, "steer_expo2", &d.SteerExpo2)
	readInt(raw, "pedal_steer_strength", &d.PedalSteerStrength)
	readInt(raw, "camera_sensitivity", &d.CameraSensitivity)
	readInt(raw, "camera_deadzone", &d.CameraDeadzone)
	readInt(raw, "pedal_left_key", &d.PedalLeftKey)
	readInt(raw, "pedal_right_key", &d.PedalRightKey)
	readInt(raw, "pedal_key_threshold", &d.PedalKeyThreshold)
	if b, ok := raw["hotas_brake_behavior"]; ok {
		_ = json.Unmarshal(b, &d.HotasBrakeBehavior)
	}
	readInt(raw, "hotas_brake_threshold", &d.HotasBrakeThreshold)
	readInt(raw, "hotas_brake_zone", &d.HotasBrakeZone)
	readBool(raw, "thumb_wasd_enabled", &d.ThumbWASDEnabled)
	if b, ok := raw["on_foot_mode"]; ok {
		_ = json.Unmarshal(b, &d.OnFootMode)
	} else if d.ThumbWASDEnabled {
		d.OnFootMode = "Thumbstick -> WASD"
	}
	readInt(raw, "thumb_wasd_device_id", &d.ThumbWASDDeviceID)
	readAxis(raw, "thumb_wasd_side_axis", &d.ThumbWASDSideAxis)
	readAxis(raw, "thumb_wasd_forward_axis", &d.ThumbWASDForwardAxis)
	readBool(raw, "thumb_wasd_invert_side", &d.ThumbWASDInvertSide)
	readBool(raw, "thumb_wasd_invert_forward", &d.ThumbWASDInvertForward)
	readInt(raw, "thumb_wasd_threshold", &d.ThumbWASDThreshold)
	readBool(raw, "thumb_wasd_use_raw_hid", &d.ThumbWASDUseRawHID)
	readString(raw, "thumb_hid_side_path", &d.ThumbHIDSidePath)
	readString(raw, "thumb_hid_forward_path", &d.ThumbHIDForwardPath)
	readInt(raw, "thumb_hid_side_usage", &d.ThumbHIDSideUsage)
	readInt(raw, "thumb_hid_forward_usage", &d.ThumbHIDForwardUsage)
	readInt32(raw, "thumb_hid_side_center", &d.ThumbHIDSideCenter)
	readInt32(raw, "thumb_hid_forward_center", &d.ThumbHIDForwardCenter)
	readInt32(raw, "thumb_hid_side_span", &d.ThumbHIDSideSpan)
	readInt32(raw, "thumb_hid_forward_span", &d.ThumbHIDForwardSpan)
	if b, ok := raw["thumb_di_side_guid"]; ok {
		_ = json.Unmarshal(b, &d.ThumbDISideGUID)
	}
	if b, ok := raw["thumb_di_forward_guid"]; ok {
		_ = json.Unmarshal(b, &d.ThumbDIForwardGUID)
	}
	if b, ok := raw["thumb_di_device_name"]; ok {
		_ = json.Unmarshal(b, &d.ThumbDIDeviceName)
	}
	readInt(raw, "thumb_di_side_axis", &d.ThumbDISideAxis)
	readInt(raw, "thumb_di_forward_axis", &d.ThumbDIForwardAxis)
	readInt32(raw, "thumb_di_side_center", &d.ThumbDISideCenter)
	readInt32(raw, "thumb_di_forward_center", &d.ThumbDIForwardCenter)
	readInt32(raw, "thumb_di_side_span", &d.ThumbDISideSpan)
	readInt32(raw, "thumb_di_forward_span", &d.ThumbDIForwardSpan)
	readInt(raw, "polling_hz", &d.PollingHz)
	maxAxis := 7
	if normalizeInputBackend(d.InputBackend) == "WinMM" {
		maxAxis = 5
	}
	for _, axis := range []struct {
		value    *int
		fallback int
	}{
		{&d.HotasThrottleAxis, 1},
		{&d.HotasStickXAxis, 0},
		{&d.HotasStickYAxis, 1},
		{&d.LeftAxis, 1},
		{&d.RightAxis, 1},
		{&d.PedalAxis, 0},
	} {
		if *axis.value < 0 || *axis.value > maxAxis {
			*axis.value = axis.fallback
		}
	}
	if d.CameraSensitivity <= 0 || d.CameraSensitivity > 250 {
		d.CameraSensitivity = 100
	}
	if d.CameraDeadzone < 0 || d.CameraDeadzone > 40 {
		d.CameraDeadzone = 6
	}
	d.ThrottleScale = normalizeScalePercent(d.ThrottleScale)
	d.SteerScale = normalizeSteerScalePercent(d.SteerScale)
	d.ThrottleExpo = normalizeExpoPercent(d.ThrottleExpo)
	d.SteerExpo = normalizeExpoPercent(d.SteerExpo)
	d.SteerExpo2 = normalizeExpoPercent(d.SteerExpo2)
	if !validKeyboardVK(d.PedalLeftKey) {
		d.PedalLeftKey = VK_DELETE
	}
	if !validKeyboardVK(d.PedalRightKey) {
		d.PedalRightKey = VK_NEXT
	}
	if d.PedalKeyThreshold < 5 || d.PedalKeyThreshold > 95 {
		d.PedalKeyThreshold = 20
	}
	d.HotasBrakeBehavior = normalizeHotasBrakeBehavior(d.HotasBrakeBehavior)
	if d.HotasBrakeThreshold < 0 || d.HotasBrakeThreshold > 50 {
		d.HotasBrakeThreshold = 5
	}
	if d.HotasBrakeZone < 1 || d.HotasBrakeZone > 50 {
		d.HotasBrakeZone = 20
	}
	if d.ThumbWASDThreshold < 5 || d.ThumbWASDThreshold > 95 {
		d.ThumbWASDThreshold = 20
	}
	if !validPollingHz(d.PollingHz) {
		d.PollingHz = 500
	}
	if d.ThumbWASDSideAxis < 0 || d.ThumbWASDSideAxis > 5 {
		d.ThumbWASDSideAxis = 0
	}
	if d.ThumbWASDForwardAxis < 0 || d.ThumbWASDForwardAxis > 5 {
		d.ThumbWASDForwardAxis = 1
	}
	readAuxiliaryAxis(raw, "twist_left", &d.TwistLeft)
	readAuxiliaryAxis(raw, "twist_right", &d.TwistRight)
	validateAuxiliaryAxis(&d.TwistLeft, raw["twist_left"] != nil, maxAxis)
	validateAuxiliaryAxis(&d.TwistRight, raw["twist_right"] != nil, maxAxis)
	d.PedalMode = normalizePedalMode(d.PedalMode)
	d.OnFootMode = normalizeOnFootMode(d.OnFootMode)
	if b, ok := raw["bindings"]; ok {
		_ = json.Unmarshal(b, &d.Bindings)
		d.Bindings = normalizeBindingMap(d.Bindings)
	}
	*c = d
}

func readInt(raw map[string]json.RawMessage, k string, dst *int) {
	b, ok := raw[k]
	if !ok {
		return
	}
	var n int
	if json.Unmarshal(b, &n) == nil {
		*dst = n
		return
	}
	var f float64
	if json.Unmarshal(b, &f) == nil {
		*dst = int(math.Round(f))
		return
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		if n, e := strconv.Atoi(s); e == nil {
			*dst = n
		}
	}
}
func readInt32(raw map[string]json.RawMessage, k string, dst *int32) {
	b, ok := raw[k]
	if !ok {
		return
	}
	var n int32
	if json.Unmarshal(b, &n) == nil {
		*dst = n
		return
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		if parsed, err := strconv.ParseInt(s, 10, 32); err == nil {
			*dst = int32(parsed)
		}
		return
	}
	var f float64
	if json.Unmarshal(b, &f) == nil {
		rounded := math.Round(f)
		if rounded >= -(1<<31) && rounded <= 1<<31-1 {
			*dst = int32(rounded)
		}
	}
}
func readBool(raw map[string]json.RawMessage, k string, dst *bool) {
	if b, ok := raw[k]; ok {
		_ = json.Unmarshal(b, dst)
	}
}
func readString(raw map[string]json.RawMessage, k string, dst *string) {
	if b, ok := raw[k]; ok {
		_ = json.Unmarshal(b, dst)
	}
}
func readAuxiliaryAxis(raw map[string]json.RawMessage, k string, dst *auxiliaryAxisConfig) {
	b, ok := raw[k]
	if !ok {
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil {
		return
	}
	readInt(fields, "device_id", &dst.DeviceID)
	readString(fields, "di_guid", &dst.DIGUID)
	readAxis(fields, "axis", &dst.Axis)
	readBool(fields, "invert", &dst.Invert)
	readString(fields, "axis_type", &dst.AxisType)
	readInt(fields, "deadzone", &dst.Deadzone)
	readPedalMode(fields, "mode", &dst.Mode)
	readInt(fields, "steer_strength", &dst.SteerStrength)
	readInt(fields, "camera_sensitivity", &dst.CameraSensitivity)
	readInt(fields, "camera_deadzone", &dst.CameraDeadzone)
	readInt(fields, "left_key", &dst.LeftKey)
	readInt(fields, "right_key", &dst.RightKey)
	readInt(fields, "key_threshold", &dst.KeyThreshold)
}

func normalizeInputBackend(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "directinput", "direct input", "di":
		return "DirectInput"
	case "winmm", "win mm", "legacy":
		return "WinMM"
	default:
		return "Auto"
	}
}

func normalizeControlMode(s string) string {
	u := strings.ToUpper(strings.TrimSpace(s))
	if strings.Contains(u, "HOTAS") {
		return "HOTAS"
	}
	return "HOSAS"
}

func normalizeHotasBrakeBehavior(s string) string {
	ls := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(ls, "analog") || strings.Contains(ls, "zone"):
		return "Analog Brake Zone"
	case strings.Contains(ls, "off") || strings.Contains(ls, "none"):
		return "Off"
	default:
		return "Digital at Idle"
	}
}

func normalizePedalMode(s string) string {
	ls := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(ls, "keyboard") || strings.Contains(ls, "key"):
		return "Keyboard keys"
	case strings.Contains(ls, "pitch") || strings.Contains(ls, "left stick y") || strings.Contains(ls, "ly"):
		return "Pitch"
	case strings.Contains(ls, "fine") || strings.Contains(ls, "steer"):
		return "Fine steering"
	case strings.Contains(ls, "camera") || strings.Contains(ls, "right") || strings.Contains(ls, "rx"):
		return "Move Camera"
	default:
		return "Off"
	}
}

func normalizeOnFootMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "thumbstick -> wasd", "thumbstick", "analog", "thumbstick to wasd":
		return "Thumbstick -> WASD"
	case "buttons -> wasd", "buttons", "button", "buttons to wasd":
		return "Buttons -> WASD"
	default:
		return "Off"
	}
}

func readPedalMode(raw map[string]json.RawMessage, k string, dst *string) {
	b, ok := raw[k]
	if !ok {
		return
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		*dst = normalizePedalMode(s)
		return
	}
	var n int
	if json.Unmarshal(b, &n) == nil {
		if n == 1 {
			*dst = "Fine steering"
		} else if n == 2 {
			*dst = "Move Camera"
		} else if n == 3 {
			*dst = "Keyboard keys"
		} else if n == 4 {
			*dst = "Pitch"
		} else {
			*dst = "Off"
		}
	}
}

func readDeadzone(raw map[string]json.RawMessage, k string, dst *int) {
	b, ok := raw[k]
	if !ok {
		return
	}
	var f float64
	if json.Unmarshal(b, &f) == nil {
		if f > 0 && f <= 1 {
			f *= 100
		}
		*dst = int(math.Round(f))
	}
}

func normalizeBindingMap(in map[string]binding) map[string]binding {
	out := make(map[string]binding)
	for k, v := range in {
		if strings.TrimSpace(v.Backend) == "" {
			v.Backend = "WinMM" // Existing configs predate backend-aware learning.
		}
		v.Backend = normalizeBindingBackend(v.Backend)
		out[normalizeBindingKey(k)] = v
	}
	// v1.94 and earlier called the manual LT mapping "BRAKE". Preserve it.
	if _, exists := out["LT"]; !exists {
		if old, ok := out["BRAKE"]; ok {
			out["LT"] = old
		}
	}
	delete(out, "BRAKE")
	return out
}

func normalizeBindingBackend(s string) string {
	if strings.EqualFold(strings.TrimSpace(s), "DirectInput") || strings.EqualFold(strings.TrimSpace(s), "DI") {
		return "DirectInput"
	}
	return "WinMM"
}

func validKeyboardVK(v int) bool {
	for _, k := range keyboardKeyOptions {
		if k.VK == v {
			return true
		}
	}
	return false
}

func keyOptionIndex(vk int) int {
	for i, k := range keyboardKeyOptions {
		if k.VK == vk {
			return i
		}
	}
	return 0
}

func selectedKeyVK(id int) int {
	i := comboIndex(id)
	if i >= 0 && i < len(keyboardKeyOptions) {
		return keyboardKeyOptions[i].VK
	}
	return keyboardKeyOptions[0].VK
}

func keyName(vk int) string {
	for _, k := range keyboardKeyOptions {
		if k.VK == vk {
			return k.Name
		}
	}
	return fmt.Sprintf("VK 0x%02X", vk)
}

func saveConfig() {
	app.cfgMu.RLock()
	c := app.cfg
	c.Bindings = make(map[string]binding, len(app.cfg.Bindings))
	for k, v := range app.cfg.Bindings {
		c.Bindings[k] = v
	}
	app.cfgMu.RUnlock()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	p := configPath()
	tmp := p + ".tmp"
	if os.WriteFile(tmp, data, 0644) == nil {
		_ = os.Remove(p)
		_ = os.Rename(tmp, p)
	}
}

func winMMHelperPath() string {
	candidates := make([]string, 0, 3)
	if exe, err := os.Executable(); err == nil && exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "GalacticRacerWinMMHelper.exe"))
	}
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		candidates = append(candidates, filepath.Join(base, "GalacticRacerHOSAS", "GalacticRacerWinMMHelper.exe"))
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		candidates = append(candidates, filepath.Join(wd, "GalacticRacerWinMMHelper.exe"))
	}
	seen := map[string]bool{}
	for _, p := range candidates {
		p = filepath.Clean(p)
		k := strings.ToLower(p)
		if seen[k] {
			continue
		}
		seen[k] = true
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

type winMMHelperCountReply struct {
	Count int `json:"count"`
}

type winMMHelperCapsReply struct {
	OK         bool   `json:"ok"`
	Result     uint32 `json:"result"`
	Name       string `json:"name"`
	Xmin       uint32 `json:"xmin"`
	Xmax       uint32 `json:"xmax"`
	Ymin       uint32 `json:"ymin"`
	Ymax       uint32 `json:"ymax"`
	Zmin       uint32 `json:"zmin"`
	Zmax       uint32 `json:"zmax"`
	Rmin       uint32 `json:"rmin"`
	Rmax       uint32 `json:"rmax"`
	Umin       uint32 `json:"umin"`
	Umax       uint32 `json:"umax"`
	Vmin       uint32 `json:"vmin"`
	Vmax       uint32 `json:"vmax"`
	NumButtons uint32 `json:"num_buttons"`
	Caps       uint32 `json:"caps"`
	MaxAxes    uint32 `json:"max_axes"`
	NumAxes    uint32 `json:"num_axes"`
	MaxButtons uint32 `json:"max_buttons"`
}

func runWinMMHelper(args ...string) ([]byte, error) {
	path := winMMHelperPath()
	if path == "" {
		return nil, fmt.Errorf("GalacticRacerWinMMHelper.exe was not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	// Keep the compatibility helper invisible. If a legacy WinMM driver corrupts
	// the helper process, Windows terminates only this child, not the main app.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("WinMM helper timed out")
	}
	return out, err
}

func enumerateDevices() []deviceInfo {
	// All joyGetDevCapsW calls live in the helper process. We intentionally query
	// one slot per helper invocation so a single broken/stale joystick driver can
	// crash that child without losing the main Galactic Racer application.
	b, err := runWinMMHelper("count")
	if err != nil {
		return nil
	}
	var count winMMHelperCountReply
	if json.Unmarshal(b, &count) != nil || count.Count <= 0 {
		return nil
	}
	if count.Count > 64 {
		count.Count = 64
	}
	out := make([]deviceInfo, 0)
	for i := 0; i < count.Count; i++ {
		b, err := runWinMMHelper("caps", strconv.Itoa(i))
		if err != nil {
			// A helper crash here is expected to be survivable. Skip only this slot.
			continue
		}
		var r winMMHelperCapsReply
		if json.Unmarshal(b, &r) != nil || !r.OK || r.Result != JOYERR_NOERROR {
			continue
		}
		name := strings.TrimSpace(r.Name)
		if name == "" {
			name = fmt.Sprintf("Joystick %d", i)
		}
		caps := joyCaps{
			Xmin: r.Xmin, Xmax: r.Xmax,
			Ymin: r.Ymin, Ymax: r.Ymax,
			Zmin: r.Zmin, Zmax: r.Zmax,
			Rmin: r.Rmin, Rmax: r.Rmax,
			Umin: r.Umin, Umax: r.Umax,
			Vmin: r.Vmin, Vmax: r.Vmax,
			NumButtons: r.NumButtons, Caps: r.Caps,
			MaxAxes: r.MaxAxes, NumAxes: r.NumAxes, MaxButtons: r.MaxButtons,
		}
		out = append(out, deviceInfo{ID: i, Name: name, Caps: caps})
	}
	return out
}

func utf16SliceToString(a []uint16) string {
	n := 0
	for n < len(a) && a[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(a[:n])
}

func readJoy(id int) (joyInfoEx, bool) {
	var j joyInfoEx
	j.Size = uint32(unsafe.Sizeof(j))
	j.Flags = JOY_RETURNALL
	app.winmmMu.Lock()
	r, _, _ := procJoyGetPosEx.Call(uintptr(id), uintptr(unsafe.Pointer(&j)))
	app.winmmMu.Unlock()
	return j, uint32(r) == JOYERR_NOERROR
}

func axisRaw(j joyInfoEx, axis int) uint32 {
	switch axis {
	case 0:
		return j.X
	case 1:
		return j.Y
	case 2:
		return j.Z
	case 3:
		return j.R
	case 4:
		return j.U
	case 5:
		return j.V
	}
	return j.Y
}
func axisRange(c joyCaps, axis int) (uint32, uint32) {
	switch axis {
	case 0:
		return c.Xmin, c.Xmax
	case 1:
		return c.Ymin, c.Ymax
	case 2:
		return c.Zmin, c.Zmax
	case 3:
		return c.Rmin, c.Rmax
	case 4:
		return c.Umin, c.Umax
	case 5:
		return c.Vmin, c.Vmax
	}
	return 0, 65535
}
func axisNormalized(deviceID, axis int, invert bool, j joyInfoEx) float64 {
	app.devicesMu.RLock()
	var caps *joyCaps
	for i := range app.devices {
		if app.devices[i].ID == deviceID {
			cp := app.devices[i].Caps
			caps = &cp
			break
		}
	}
	app.devicesMu.RUnlock()
	minv, maxv := uint32(0), uint32(65535)
	if caps != nil {
		minv, maxv = axisRange(*caps, axis)
	}
	if maxv <= minv {
		minv, maxv = 0, 65535
	}
	raw := axisRaw(j, axis)
	v := ((float64(raw)-float64(minv))/float64(maxv-minv))*2.0 - 1.0
	if invert {
		v = -v
	}
	return clamp(v, -1, 1)
}

// axisThrottleCenteredNormalized treats an absolute throttle axis as a bidirectional
// HOSAS input: 0% travel = -1, 50% travel = 0, 100% travel = +1.
// This is intentionally explicit even though many symmetric WinMM axes normalize
// similarly, because throttle devices are not spring-centered and users need a
// predictable midpoint-neutral mode.
func axisThrottleCenteredNormalized(deviceID, axis int, invert bool, j joyInfoEx) float64 {
	app.devicesMu.RLock()
	var caps *joyCaps
	for i := range app.devices {
		if app.devices[i].ID == deviceID {
			cp := app.devices[i].Caps
			caps = &cp
			break
		}
	}
	app.devicesMu.RUnlock()
	minv, maxv := uint32(0), uint32(65535)
	if caps != nil {
		minv, maxv = axisRange(*caps, axis)
	}
	if maxv <= minv {
		minv, maxv = 0, 65535
	}
	raw := float64(axisRaw(j, axis))
	minf, maxf := float64(minv), float64(maxv)
	mid := minf + (maxf-minf)*0.5
	var v float64
	if raw >= mid {
		span := maxf - mid
		if span > 0 {
			v = (raw - mid) / span
		}
	} else {
		span := mid - minf
		if span > 0 {
			v = (raw - mid) / span
		}
	}
	if invert {
		v = -v
	}
	return clamp(v, -1, 1)
}

func deviceNameByID(id int) string {
	app.devicesMu.RLock()
	defer app.devicesMu.RUnlock()
	for _, d := range app.devices {
		if d.ID == id {
			return d.Name
		}
	}
	return ""
}

func normalizedDeviceKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (a *appState) matchWinMMIDByName(name string) int {
	key := normalizedDeviceKey(name)
	if key == "" {
		return -1
	}
	a.devicesMu.RLock()
	defer a.devicesMu.RUnlock()
	best := -1
	for _, d := range a.devices {
		dk := normalizedDeviceKey(d.Name)
		if dk == key {
			return d.ID
		}
		if dk != "" && (strings.Contains(dk, key) || strings.Contains(key, dk)) {
			if best < 0 {
				best = d.ID
			}
		}
	}
	return best
}

func (a *appState) readConfiguredAxis(backend, guid string, deviceID, axis int, invert bool, winStates map[int]joyInfoEx, winOK map[int]bool) (float64, bool) {
	backend = normalizeInputBackend(backend)
	if backend != "WinMM" && guid != "" && axis >= 0 && axis < 8 {
		if axes, _, ok := a.di.StateByGUID(guid); ok {
			return normalizeDIAbsoluteAxis(axes[axis], invert), true
		}
		if backend == "DirectInput" {
			return 0, false
		}
	}
	if deviceID >= 0 && axis >= 0 && axis < 6 && winOK[deviceID] {
		return axisNormalized(deviceID, axis, invert, winStates[deviceID]), true
	}
	return 0, false
}

func (a *appState) readConfiguredThrottleCentered(backend, guid string, deviceID, axis int, invert bool, winStates map[int]joyInfoEx, winOK map[int]bool) (float64, bool) {
	backend = normalizeInputBackend(backend)
	if backend != "WinMM" && guid != "" && axis >= 0 && axis < 8 {
		if axes, _, ok := a.di.StateByGUID(guid); ok {
			// DirectInput absolute axes already normalize naturally to -1..+1 around midpoint.
			return normalizeDIAbsoluteAxis(axes[axis], invert), true
		}
		if backend == "DirectInput" {
			return 0, false
		}
	}
	if deviceID >= 0 && axis >= 0 && axis < 6 && winOK[deviceID] {
		return axisThrottleCenteredNormalized(deviceID, axis, invert, winStates[deviceID]), true
	}
	return 0, false
}

func applyDeadzone(v float64, pct int) float64 {
	dz := clamp(float64(pct)/100.0, 0, 0.40)
	a := math.Abs(v)
	if a <= dz {
		return 0
	}
	out := (a - dz) / (1 - dz)
	if v < 0 {
		out = -out
	}
	return clamp(out, -1, 1)
}

func monitorHorizontalAxis(forwardAxis int) int {
	if forwardAxis == 0 {
		return 1
	}
	return 0
}

func (a *appState) pollLoop() {
	defer a.pollWG.Done()
	a.cfgMu.RLock()
	currentInterval := pollingInterval(a.cfg.PollingHz)
	a.cfgMu.RUnlock()
	ticker := time.NewTicker(currentInterval)
	defer func() { ticker.Stop() }()
	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.cfgMu.RLock()
			c := a.cfg
			a.cfgMu.RUnlock()
			desiredInterval := pollingInterval(c.PollingHz)
			if desiredInterval != currentInterval {
				ticker.Stop()
				ticker = time.NewTicker(desiredInterval)
				currentInterval = desiredInterval
			}

			ids := []int{}
			if strings.EqualFold(normalizeControlMode(c.ControlMode), "HOTAS") {
				ids = append(ids, c.HotasThrottleID, c.HotasStickID)
			} else {
				ids = append(ids, c.LeftID, c.RightID)
			}
			if c.PedalID >= 0 {
				ids = append(ids, c.PedalID)
			}
			for _, tw := range []auxiliaryAxisConfig{c.TwistLeft, c.TwistRight} {
				if tw.DeviceID >= 0 {
					ids = append(ids, tw.DeviceID)
				}
			}
			for _, b := range c.Bindings {
				if normalizeBindingBackend(b.Backend) != "DirectInput" && b.DeviceID >= 0 {
					ids = append(ids, b.DeviceID)
				}
			}

			states := make(map[int]joyInfoEx)
			okmap := make(map[int]bool)
			for _, id := range ids {
				if id < 0 {
					continue
				}
				if _, exists := states[id]; exists {
					continue
				}
				st, ok := readJoy(id)
				states[id] = st
				okmap[id] = ok
			}

			diStates := make(map[string]diJoyState)
			diOK := make(map[string]bool)
			for _, b := range c.Bindings {
				if normalizeBindingBackend(b.Backend) != "DirectInput" || b.DIGUID == "" {
					continue
				}
				if _, exists := diOK[b.DIGUID]; exists {
					continue
				}
				st, _, ok := a.di.FullState(b.DIGUID)
				diStates[b.DIGUID] = st
				diOK[b.DIGUID] = ok
			}

			l, r := 0.0, 0.0
			hotasThrottle, hotasX, hotasY := 0.0, 0.0, 0.0
			hotasThrottleValid := false
			pedalRaw, p, cameraX := 0.0, 0.0, 0.0
			pedalValid := false
			var twistRaw [2]float64
			var twistValues [2]float64
			var twistValid [2]bool
			twistCfg := [2]auxiliaryAxisConfig{c.TwistLeft, c.TwistRight}
			isHOTAS := strings.EqualFold(normalizeControlMode(c.ControlMode), "HOTAS")
			backend := normalizeInputBackend(c.InputBackend)
			if isHOTAS {
				if raw, ok := a.readConfiguredAxis(backend, c.HotasThrottleDIGUID, c.HotasThrottleID, c.HotasThrottleAxis, c.HotasThrottleInvert, states, okmap); ok {
					hotasThrottle = clamp((raw+1.0)/2.0, 0, 1)
					hotasThrottleValid = true
				}
				if raw, ok := a.readConfiguredAxis(backend, c.HotasStickDIGUID, c.HotasStickID, c.HotasStickXAxis, c.HotasStickInvertX, states, okmap); ok {
					hotasX = applyDeadzone(raw, c.Deadzone)
				}
				if raw, ok := a.readConfiguredAxis(backend, c.HotasStickDIGUID, c.HotasStickID, c.HotasStickYAxis, c.HotasStickInvertY, states, okmap); ok {
					hotasY = applyDeadzone(raw, c.Deadzone)
				}
			} else {
				if c.LeftThrottleMode {
					if raw, ok := a.readConfiguredThrottleCentered(backend, c.LeftDIGUID, c.LeftID, c.LeftAxis, c.LeftInvert, states, okmap); ok {
						l = applyDeadzone(raw, c.Deadzone)
					}
				} else if raw, ok := a.readConfiguredAxis(backend, c.LeftDIGUID, c.LeftID, c.LeftAxis, c.LeftInvert, states, okmap); ok {
					l = applyDeadzone(raw, c.Deadzone)
				}
				if c.RightThrottleMode {
					if raw, ok := a.readConfiguredThrottleCentered(backend, c.RightDIGUID, c.RightID, c.RightAxis, c.RightInvert, states, okmap); ok {
						r = applyDeadzone(raw, c.Deadzone)
					}
				} else if raw, ok := a.readConfiguredAxis(backend, c.RightDIGUID, c.RightID, c.RightAxis, c.RightInvert, states, okmap); ok {
					r = applyDeadzone(raw, c.Deadzone)
				}
			}
			if raw, ok := a.readConfiguredAxis(backend, c.PedalDIGUID, c.PedalID, c.PedalAxis, c.PedalInvert, states, okmap); ok {
				pedalRaw = raw
				pedalValid = true
				p = applyDeadzone(pedalRaw, c.Deadzone)
				cameraX = clamp(
					applyDeadzone(pedalRaw, c.CameraDeadzone)*float64(c.CameraSensitivity)/100.0,
					-1, 1,
				)
			}

			for i, tw := range twistCfg {
				if tw.DeviceID < 0 && tw.DIGUID == "" {
					continue
				}
				if raw, ok := a.readConfiguredAxis(backend, tw.DIGUID, tw.DeviceID, tw.Axis, false, states, okmap); ok {
					twistRaw[i] = raw
					twistValues[i] = normalizeAuxiliaryValue(raw, tw, tw.Deadzone)
					twistValid[i] = true
				}
			}

			drive := calculateDrive(l, r, c.ThrottleScale, c.ThrottleExpo)
			steer := calculateSteering(l, r, c.SteerScale, c.SteerExpo, c.SteerExpo2)
			hotasBrake := false
			hotasBrakeAmount := 0.0
			if isHOTAS {
				if hotasThrottleValid {
					switch normalizeHotasBrakeBehavior(c.HotasBrakeBehavior) {
					case "Off":
						drive = hotasThrottle
					case "Analog Brake Zone":
						zone := clamp(float64(c.HotasBrakeZone)/100.0, 0.01, 0.50)
						if hotasThrottle <= zone {
							hotasBrakeAmount = clamp(1.0-hotasThrottle/zone, 0, 1)
							drive = -hotasBrakeAmount
						} else {
							// Rescale the remaining travel so the user still gets full 0..100% RT.
							drive = clamp((hotasThrottle-zone)/(1.0-zone), 0, 1)
						}
					default: // Digital at Idle
						threshold := clamp(float64(c.HotasBrakeThreshold)/100.0, 0, 0.50)
						if hotasThrottle <= threshold {
							hotasBrakeAmount = 1.0
							drive = -1.0
						} else {
							drive = hotasThrottle
						}
					}
				} else {
					drive = 0
				}
				hotasBrake = hotasBrakeAmount > 0.001
				steer = hotasX
			}
			if strings.EqualFold(c.PedalMode, "Fine steering") {
				steer = clamp(steer+p*float64(c.PedalSteerStrength)/100.0, -1, 1)
			}
			pitchValue, pitchAssigned := 0.0, false
			cameraValue, cameraAssigned := 0.0, false
			if pedalValid {
				if strings.EqualFold(c.PedalMode, "Pitch") {
					pitchValue += p
					pitchAssigned = true
				}
				if strings.EqualFold(c.PedalMode, "Move Camera") {
					cameraValue += cameraX
					cameraAssigned = true
				}
			}
			for i, tw := range twistCfg {
				if !twistValid[i] {
					continue
				}
				v := twistValues[i]
				switch normalizePedalMode(tw.Mode) {
				case "Fine steering":
					steer = clamp(steer+v*float64(tw.SteerStrength)/100, -1, 1)
				case "Pitch":
					pitchValue += v
					pitchAssigned = true
				case "Move Camera":
					cameraValue += normalizeAuxiliaryValue(twistRaw[i], tw, tw.CameraDeadzone) * float64(tw.CameraSensitivity) / 100
					cameraAssigned = true
				}
			}
			pitchValue = clamp(pitchValue, -1, 1)
			cameraValue = clamp(cameraValue, -1, 1)

			var buttonMask uint16
			var outputHighlights uint32
			manualLT, manualRT := false, false
			manualPitchUp, manualPitchDown := false, false
			keyboardWanted := make(map[uint16]bool)
			for i, bo := range buttonOutputs {
				pressed := false
				if b, ok := c.Bindings[bo.Key]; ok {
					pressed = bindingPressed(b, states, okmap, diStates, diOK)
				}
				if pressed {
					outputHighlights |= uint32(1) << uint(i)
					if bo.Mask != 0 {
						buttonMask |= bo.Mask
					}
					if bo.Key == "LT" {
						manualLT = true
					}
					if bo.Key == "RT" {
						manualRT = true
					}
					if bo.Key == "PITCHUP" {
						manualPitchUp = true
					}
					if bo.Key == "PITCHDOWN" {
						manualPitchDown = true
					}
				}
				if bo.VK != 0 {
					keyboardWanted[bo.VK] = keyboardWanted[bo.VK] || (pressed && a.mappingActive.Load())
				}
			}
			if strings.EqualFold(c.PedalMode, "Keyboard keys") && a.mappingActive.Load() && (c.PedalID >= 0 || c.PedalDIGUID != "") {
				threshold := clamp(float64(c.PedalKeyThreshold)/100.0, 0.05, 0.95)
				if pedalRaw <= -threshold {
					keyboardWanted[uint16(c.PedalLeftKey)] = true
				} else if pedalRaw >= threshold {
					keyboardWanted[uint16(c.PedalRightKey)] = true
				}
			}

			if a.mappingActive.Load() {
				for i, tw := range twistCfg {
					if !twistValid[i] || !strings.EqualFold(tw.Mode, "Keyboard keys") {
						continue
					}
					v := normalizeAuxiliaryValue(twistRaw[i], tw, 0)
					threshold := clamp(float64(tw.KeyThreshold)/100, .05, .95)
					if v <= -threshold {
						keyboardWanted[uint16(tw.LeftKey)] = true
					}
					if v >= threshold {
						keyboardWanted[uint16(tw.RightKey)] = true
					}
				}
			}

			// On-foot controls are selectable: DirectInput thumbstick axes or learned buttons.
			var onFootHighlights uint8
			onFootMode := normalizeOnFootMode(c.OnFootMode)
			if strings.EqualFold(onFootMode, "Thumbstick -> WASD") && a.mappingActive.Load() {
				var sideAxes, forwardAxes [8]int32
				gotSide, gotForward := false, false
				if c.ThumbDISideGUID != "" {
					sideAxes, gotSide = a.di.State(c.ThumbDISideGUID)
				}
				if c.ThumbDIForwardGUID != "" {
					if strings.EqualFold(c.ThumbDIForwardGUID, c.ThumbDISideGUID) && gotSide {
						forwardAxes, gotForward = sideAxes, true
					} else {
						forwardAxes, gotForward = a.di.State(c.ThumbDIForwardGUID)
					}
				}
				if gotSide && gotForward && c.ThumbDISideAxis >= 0 && c.ThumbDISideAxis < 8 && c.ThumbDIForwardAxis >= 0 && c.ThumbDIForwardAxis < 8 {
					side := normalizeDIAxis(sideAxes[c.ThumbDISideAxis], c.ThumbDISideCenter, c.ThumbDISideSpan)
					forward := normalizeDIAxis(forwardAxes[c.ThumbDIForwardAxis], c.ThumbDIForwardCenter, c.ThumbDIForwardSpan)
					if c.ThumbWASDInvertSide {
						side = -side
					}
					if c.ThumbWASDInvertForward {
						forward = -forward
					}
					threshold := clamp(float64(c.ThumbWASDThreshold)/100.0, 0.05, 0.95)
					if forward >= threshold {
						keyboardWanted[uint16('W')] = true
					} else if forward <= -threshold {
						keyboardWanted[uint16('S')] = true
					}
					if side <= -threshold {
						keyboardWanted[uint16('A')] = true
					} else if side >= threshold {
						keyboardWanted[uint16('D')] = true
					}
				}
			} else if strings.EqualFold(onFootMode, "Buttons -> WASD") {
				for i, def := range wasdButtonDefs {
					pressed := false
					if b, ok := c.Bindings[def.Key]; ok {
						pressed = bindingPressed(b, states, okmap, diStates, diOK)
					}
					if pressed {
						onFootHighlights |= uint8(1 << uint(i))
						if a.mappingActive.Load() {
							keyboardWanted[def.VK] = true
						}
					}
				}
			}
			a.applyKeyboardOutputs(keyboardWanted)

			// Live monitor visuals intentionally do not do the extra axis work while
			// game mapping is active, unless the overlay or Steuerung live card needs them.
			// The controller path remains at the selected polling rate.
			leftX, leftY, rightX, rightY := 0.0, l, 0.0, r
			active := a.mappingActive.Load()
			if isHOTAS {
				leftX, leftY = hotasX, hotasY
				rightX, rightY = 0, 0
			} else if !active || a.overlayVisible.Load() || a.liveCardActive.Load() {
				hax := monitorHorizontalAxis(c.LeftAxis)
				if raw, ok := a.readConfiguredAxis(backend, c.LeftDIGUID, c.LeftID, hax, false, states, okmap); ok {
					leftX = applyDeadzone(raw, c.Deadzone)
				}
				hax = monitorHorizontalAxis(c.RightAxis)
				if raw, ok := a.readConfiguredAxis(backend, c.RightDIGUID, c.RightID, hax, false, states, okmap); ok {
					rightX = applyDeadzone(raw, c.Deadzone)
				}
			}

			a.monitorMu.Lock()
			a.monitor = monitorState{
				Left:             l,
				Right:            r,
				Pedal:            p,
				TwistLeft:        twistValues[0],
				TwistRight:       twistValues[1],
				Drive:            drive,
				Steer:            steer,
				Throttle:         hotasThrottle,
				HotasBrake:       hotasBrake,
				HotasBrakeAmount: hotasBrakeAmount,
				LeftX:            leftX,
				LeftY:            leftY,
				RightX:           rightX,
				RightY:           rightY,
				CameraX:          cameraX,
				OutputButtons:    buttonMask,
				OutputHighlights: outputHighlights,
				OnFootHighlights: onFootHighlights,
				LastUpdate:       time.Now(),
			}
			a.monitorMu.Unlock()

			if a.xboxReady.Load() {
				var rep xusbReport

				a.testUntilMu.RLock()
				testA := time.Now().Before(a.testUntil)
				a.testUntilMu.RUnlock()

				a.cameraTestMu.RLock()
				cameraTest := time.Now().Before(a.cameraTestUntil)
				cameraTestValue := a.cameraTestValue
				a.cameraTestMu.RUnlock()

				if active {
					if isHOTAS {
						if drive >= 0 {
							rep.RightTrigger = byte(math.Round(clamp(drive, 0, 1) * 255))
						} else {
							rep.LeftTrigger = byte(math.Round(clamp(-drive, 0, 1) * 255))
						}
						rep.ThumbLX = floatToStick(steer)
						rep.ThumbLY = floatToStick(hotasY)
					} else {
						if drive >= 0 {
							rep.RightTrigger = byte(math.Round(clamp(drive, 0, 1) * 255))
						} else {
							rep.LeftTrigger = byte(math.Round(clamp(-drive, 0, 1) * 255))
						}
						rep.ThumbLX = floatToStick(steer)
					}
					if cameraAssigned {
						rep.ThumbRX = floatToStick(cameraValue)
					}
					// Each configured Pitch source contributes to the analog Xbox Left Stick Y.
					// Centered Invert reverses direction; Lever Output Negative controls polarity.
					if pitchAssigned {
						rep.ThumbLY = floatToStick(pitchValue)
					}
					// Learned trigger mappings are independent of the analog drive calculation.
					if manualLT {
						rep.LeftTrigger = 255
					}
					if manualRT {
						rep.RightTrigger = 255
					}
					// Digital Pitch Up/Down mappings are full-deflection Left Stick Y outputs.
					// If both are held, they cancel to neutral.
					if manualPitchUp && !manualPitchDown {
						rep.ThumbLY = floatToStick(1.0)
					} else if manualPitchDown && !manualPitchUp {
						rep.ThumbLY = floatToStick(-1.0)
					} else if manualPitchUp && manualPitchDown {
						rep.ThumbLY = 0
					}
					rep.Buttons = buttonMask
				}

				if cameraTest {
					rep.ThumbRX = floatToStick(cameraTestValue)
				}
				if testA {
					rep.Buttons |= XUSB_A
				}
				_ = a.xbox.Update(rep)
			}
		}
	}
}

func (a *appState) applyKeyboardOutputs(wanted map[uint16]bool) {
	a.keyboardMu.Lock()
	defer a.keyboardMu.Unlock()
	if a.keyboardDown == nil {
		a.keyboardDown = make(map[uint16]bool)
	}
	all := make(map[uint16]bool, len(wanted)+len(a.keyboardDown))
	for vk := range wanted {
		if vk != 0 {
			all[vk] = true
		}
	}
	for vk := range a.keyboardDown {
		if vk != 0 {
			all[vk] = true
		}
	}
	for vk := range all {
		want := wanted[vk]
		isDown := a.keyboardDown[vk]
		if want == isDown {
			continue
		}
		flags := uintptr(0)
		if !want {
			flags = KEYEVENTF_KEYUP
		}
		procKeybdEvent.Call(uintptr(byte(vk)), 0, flags, 0)
		a.keyboardDown[vk] = want
	}
}

func (a *appState) releaseAllKeyboardOutputs() {
	a.keyboardMu.Lock()
	defer a.keyboardMu.Unlock()
	for vk, down := range a.keyboardDown {
		if down {
			procKeybdEvent.Call(uintptr(byte(vk)), 0, KEYEVENTF_KEYUP, 0)
		}
		a.keyboardDown[vk] = false
	}
}

func floatToStick(v float64) int16 {
	v = clamp(v, -1, 1)
	if v < 0 {
		return int16(math.Round(v * 32768))
	}
	return int16(math.Round(v * 32767))
}

func bindingPressed(b binding, states map[int]joyInfoEx, oks map[int]bool, diStates map[string]diJoyState, diOK map[string]bool) bool {
	if normalizeBindingBackend(b.Backend) == "DirectInput" || b.DIGUID != "" {
		if b.DIGUID == "" || !diOK[b.DIGUID] {
			return false
		}
		st := diStates[b.DIGUID]
		if strings.EqualFold(b.Kind, "button") {
			if b.Button < 0 || b.Button >= len(st.Buttons) {
				return false
			}
			return st.Buttons[b.Button]&0x80 != 0
		}
		if strings.EqualFold(b.Kind, "pov") {
			idx := b.POVIndex
			if idx < 0 || idx >= len(st.POV) {
				idx = 0
			}
			v := st.POV[idx]
			if v == 0xFFFFFFFF {
				return false
			}
			return angleDistance(int(v), b.POV) <= 2300
		}
		return false
	}

	if !oks[b.DeviceID] {
		return false
	}
	st := states[b.DeviceID]
	if strings.EqualFold(b.Kind, "button") {
		if b.Button < 0 || b.Button > 31 {
			return false
		}
		return st.Buttons&(uint32(1)<<uint(b.Button)) != 0
	}
	if strings.EqualFold(b.Kind, "pov") {
		if st.POV == JOY_POVCENTERED {
			return false
		}
		d := angleDistance(int(st.POV), b.POV)
		return d <= 2300
	}
	return false
}
func angleDistance(a, b int) int {
	d := int(math.Abs(float64(a - b)))
	if d > 18000 {
		d = 36000 - d
	}
	return d
}

func (a *appState) startMappingAsync() {
	a.postStatus("Creating virtual Xbox controller...")
	before := xinputConnectedSlots()
	if err := a.xbox.Start(); err != nil {
		a.mappingStarting.Store(false)
		a.mappingActive.Store(false)
		a.xboxReady.Store(false)
		a.xboxSlot.Store(-1)
		a.postStatus("Xbox output ERROR: " + err.Error() + "  Run Install.cmd and restart Windows if ViGEmBus was just installed.")
		procPostMessageW.Call(a.hwnd, WM_APP_XBOX, 0, 0)
		return
	}

	// The target now exists. Enable output immediately; XInput slot detection can finish in parallel.
	a.xboxReady.Store(true)
	a.mappingActive.Store(true)

	slot := -1
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		after := xinputConnectedSlots()
		for _, s := range after {
			if !containsInt(before, s) {
				slot = s
				break
			}
		}
		if slot >= 0 {
			break
		}
	}
	if slot < 0 {
		after := xinputConnectedSlots()
		if len(after) > 0 && len(before) == 0 {
			slot = after[0]
		}
	}
	a.xboxSlot.Store(int32(slot))
	a.mappingStarting.Store(false)
	if slot >= 0 {
		a.postStatus(fmt.Sprintf("Mapping ACTIVE. Virtual Xbox created in Windows XInput slot %d.", slot+1))
	} else {
		a.postStatus("Mapping ACTIVE. Virtual Xbox created; XInput slot verification is inconclusive.")
	}
	procPostMessageW.Call(a.hwnd, WM_APP_XBOX, 0, 0)
}

func (a *appState) stopMapping() {
	// Stop output first so the polling loop no longer submits reports while the target is removed.
	a.mappingActive.Store(false)
	a.releaseAllKeyboardOutputs()
	a.xboxReady.Store(false)
	a.xboxSlot.Store(-1)

	a.testUntilMu.Lock()
	a.testUntil = time.Time{}
	a.testUntilMu.Unlock()
	a.cameraTestMu.Lock()
	a.cameraTestUntil = time.Time{}
	a.cameraTestValue = 0
	a.cameraTestMu.Unlock()

	a.xbox.Stop()
	a.postStatus("Mapping stopped. Virtual Xbox controller disconnected and released.")
	procPostMessageW.Call(a.hwnd, WM_APP_XBOX, 0, 0)
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

var (
	xinputOnce     sync.Once
	xinputDLL      *syscall.DLL
	xinputGetState *syscall.Proc
)

func getXInputGetState() *syscall.Proc {
	xinputOnce.Do(func() {
		for _, n := range []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"} {
			dll, err := syscall.LoadDLL(n)
			if err != nil {
				continue
			}
			p, err := dll.FindProc("XInputGetState")
			if err == nil {
				xinputDLL = dll // Keep loaded for the lifetime of the app.
				xinputGetState = p
				return
			}
			_ = dll.Release()
		}
	})
	return xinputGetState
}

func xinputConnectedSlots() []int {
	p := getXInputGetState()
	if p == nil {
		return nil
	}
	out := []int{}
	for i := 0; i < 4; i++ {
		var st xinputState
		r, _, _ := p.Call(uintptr(i), uintptr(unsafe.Pointer(&st)))
		if uint32(r) == 0 {
			out = append(out, i)
		}
	}
	return out
}

func xinputReadState(slot int) (xinputState, bool) {
	var st xinputState
	if slot < 0 || slot > 3 {
		return st, false
	}
	p := getXInputGetState()
	if p == nil {
		return st, false
	}
	r, _, _ := p.Call(uintptr(slot), uintptr(unsafe.Pointer(&st)))
	return st, uint32(r) == 0
}

func stickReadback(v int16) float64 {
	if v < 0 {
		return float64(v) / 32768.0
	}
	return float64(v) / 32767.0
}

func (x *xboxBridge) Start() error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.ready {
		return nil
	}

	// Prefer the DLL beside the running EXE, but do not assume a local/dev build
	// always copied it there. Fall back to the normal installed app directory.
	candidates := make([]string, 0, 3)
	if exe, err := os.Executable(); err == nil && exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "ViGEmClient.dll"))
	}
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		candidates = append(candidates, filepath.Join(base, "GalacticRacerHOSAS", "ViGEmClient.dll"))
	} else if base, err := os.UserConfigDir(); err == nil && base != "" {
		candidates = append(candidates, filepath.Join(base, "GalacticRacerHOSAS", "ViGEmClient.dll"))
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		candidates = append(candidates, filepath.Join(wd, "ViGEmClient.dll"))
	}

	seen := make(map[string]bool)
	checked := make([]string, 0, len(candidates))
	var dll *syscall.DLL
	var err error
	for _, path := range candidates {
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		checked = append(checked, clean)
		if _, statErr := os.Stat(clean); statErr != nil {
			continue
		}
		dll, err = syscall.LoadDLL(clean)
		if err == nil {
			break
		}
	}
	if dll == nil {
		if len(checked) == 0 {
			return fmt.Errorf("ViGEmClient.dll could not be located")
		}
		return fmt.Errorf("ViGEmClient.dll could not be loaded; checked: %s", strings.Join(checked, "; "))
	}
	req := func(n string) (*syscall.Proc, error) { return dll.FindProc(n) }
	if x.alloc, err = req("vigem_alloc"); err != nil {
		return err
	}
	if x.free, err = req("vigem_free"); err != nil {
		return err
	}
	if x.connect, err = req("vigem_connect"); err != nil {
		return err
	}
	if x.disconnect, err = req("vigem_disconnect"); err != nil {
		return err
	}
	if x.targetAlloc, err = req("vigem_target_x360_alloc"); err != nil {
		return err
	}
	if x.targetFree, err = req("vigem_target_free"); err != nil {
		return err
	}
	if x.targetAdd, err = req("vigem_target_add"); err != nil {
		return err
	}
	if x.targetRemove, err = req("vigem_target_remove"); err != nil {
		return err
	}
	if x.targetUpdate, err = req("vigem_target_x360_update"); err != nil {
		return err
	}
	x.dll = dll
	client, _, _ := x.alloc.Call()
	if client == 0 {
		return fmt.Errorf("vigem_alloc failed")
	}
	x.client = client
	r, _, _ := x.connect.Call(client)
	if uint32(r) != 0x20000000 {
		x.cleanupLocked()
		return fmt.Errorf("ViGEmBus connect failed (0x%X)", uint32(r))
	}
	target, _, _ := x.targetAlloc.Call()
	if target == 0 {
		x.cleanupLocked()
		return fmt.Errorf("could not create virtual Xbox controller")
	}
	x.target = target
	r, _, _ = x.targetAdd.Call(client, target)
	if uint32(r) != 0x20000000 {
		x.cleanupLocked()
		return fmt.Errorf("could not add virtual Xbox controller (0x%X)", uint32(r))
	}
	x.ready = true
	neutral := xusbReport{}
	_, _, _ = x.targetUpdate.Call(x.client, x.target, uintptr(unsafe.Pointer(&neutral)))
	return nil
}
func (x *xboxBridge) Update(r xusbReport) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if !x.ready {
		return fmt.Errorf("Xbox not ready")
	}
	rc, _, _ := x.targetUpdate.Call(x.client, x.target, uintptr(unsafe.Pointer(&r)))
	if uint32(rc) != 0x20000000 {
		return fmt.Errorf("update failed 0x%X", uint32(rc))
	}
	return nil
}
func (x *xboxBridge) Stop() { x.mu.Lock(); defer x.mu.Unlock(); x.cleanupLocked() }
func (x *xboxBridge) cleanupLocked() {
	if x.target != 0 && x.client != 0 && x.targetRemove != nil {
		x.targetRemove.Call(x.client, x.target)
	}
	if x.target != 0 && x.targetFree != nil {
		x.targetFree.Call(x.target)
	}
	if x.client != 0 && x.disconnect != nil {
		x.disconnect.Call(x.client)
	}
	if x.client != 0 && x.free != nil {
		x.free.Call(x.client)
	}
	if x.dll != nil {
		_ = x.dll.Release()
	}
	x.client = 0
	x.target = 0
	x.ready = false
	x.dll = nil
}

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		app.hwnd = hwnd
		app.initializeUITheme()
		app.createUI()
		procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
			SWP_NOMOVE|SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE|SWP_FRAMECHANGED)
		return 0
	case WM_NCCALCSIZE:
		if wParam != 0 {
			return 0
		}
	case WM_COMMAND:
		app.onCommand(loword(wParam), hiword(wParam))
		return 0
	case WM_DRAWITEM:
		if item, ok := readCurrentProcessValue[drawItem](lParam); ok && app.drawOwnerButton(item) {
			return 1
		}
	case WM_CTLCOLORSTATIC, WM_CTLCOLOREDIT, WM_CTLCOLORBTN, WM_CTLCOLORLISTBOX, WM_CTLCOLORMSGBOX:
		if brush := app.controlColor(message, wParam, lParam); brush != 0 {
			return brush
		}
	case WM_ERASEBKGND:
		if app.eraseWindowBackground(wParam) {
			return 1
		}
	case WM_PAINT:
		var ps paintStruct
		if hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps))); hdc != 0 {
			app.drawNativeUI(hdc)
			app.drawCustomFrame(hdc)
			procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
			return 0
		}
	case WM_NCHITTEST:
		var bounds rect
		if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds))); ok != 0 {
			maximized, _, _ := procIsZoomed.Call(hwnd)
			hit := customFrameHitTest(bounds, point{
				X: signedLowWord(lParam),
				Y: signedHighWord(lParam),
			}, maximized != 0, windowResizeBorder, windowResizeBorder)
			if hit != HTNOWHERE {
				return hit
			}
		}
	case WM_LBUTTONDOWN:
		app.closeComboIfPointerOutside()
		return 0
	case WM_MOUSEWHEEL:
		app.closeCombo(false)
		app.handleMouseWheel(wParam)
		return 0
	case WM_PARENTNOTIFY:
		// Mouse clicks on child controls notify the parent. If a custom selector is
		// open and the click is outside its list, dismiss it before handling the click.
		if loword(wParam) == WM_LBUTTONDOWN {
			app.closeComboIfPointerOutside()
		}
	case WM_SIZE:
		app.closeCombo(false)
		app.updateCaptionButtons(wParam)
		app.resizeCaptionButtons()
		app.resizeSidebar()
		app.updateScrollbars()
		return 0
	case WM_HSCROLL:
		if spec, ok := app.valueSliderForHandle(lParam); ok {
			app.handleValueSlider(spec, loword(wParam))
			return 0
		}
	case WM_TIMER:
		if wParam == overlayTimerID {
			app.refreshOverlay()
			return 0
		}
		if wParam == liveCardTimerID {
			app.refreshLiveCard()
			return 0
		}
		app.refreshMonitor()
		app.refreshDirectInputMonitor()
		app.refreshHotasDirectInputMonitor()
		app.updateHoverTooltip()
		return 0
	case WM_APP_STATUS:
		app.drainStatus()
		return 0
	case WM_APP_DETECT:
		app.drainDetect()
		return 0
	case WM_APP_LEARN:
		app.drainLearn()
		return 0
	case WM_APP_XBOX:
		app.refreshXboxUI()
		return 0
	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		procKillTimer.Call(hwnd, 1)
		app.hideOverlay(false)
		app.releaseUITheme()
		app.stopOnce.Do(func() { close(app.stopCh) })
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func (a *appState) createControl(class, text string, style uintptr, x, y, w, h, id int) uintptr {
	return a.createControlAt(class, text, style, x+contentOffset, y, w, h, id)
}

func (a *appState) createControlAt(class, text string, style uintptr, x, y, w, h, id int) uintptr {
	if strings.EqualFold(class, "BUTTON") && style&0xF == BS_PUSHBUTTON {
		style = style&^uintptr(0xF) | BS_OWNERDRAW
	}
	ctrl, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(utf16ptr(class))), uintptr(unsafe.Pointer(utf16ptr(text))), style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), a.hwnd, uintptr(id), 0, 0)
	if ctrl != 0 && a.font != 0 {
		procSendMessageW.Call(ctrl, WM_SETFONT, a.font, 1)
	}
	if id != 0 {
		a.controls[id] = ctrl
	}
	if ctrl != 0 && a.controlSections != nil && a.activeControlSection != 0 {
		a.controlSections[ctrl] = a.activeControlSection
	}
	if ctrl != 0 && a.baseRects != nil {
		a.baseRects[ctrl] = rect{Left: int32(x), Top: int32(y), Right: int32(x + w), Bottom: int32(y + h)}
	}
	if ctrl != 0 && a.contextVisible != nil {
		a.contextVisible[ctrl] = style&WS_VISIBLE != 0
	}
	return ctrl
}
func (a *appState) createLabel(text string, x, y, w, h int) uintptr {
	return a.createControl("STATIC", text, WS_CHILD|WS_VISIBLE|SS_LEFT, x, y, w, h, 0)
}
func (a *appState) createFixedControl(class, text string, style uintptr, x, y, w, h, id int) uintptr {
	hwnd := a.createControlAt(class, text, style, x, y, w, h, id)
	delete(a.baseRects, hwnd)
	return hwnd
}
func (a *appState) createButton(text string, x, y, w, h, id int) uintptr {
	return a.createControl("BUTTON", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, x, y, w, h, id)
}
func (a *appState) createCheck(text string, x, y, w, h, id int) uintptr {
	state := a.createControl("BUTTON", text, WS_CHILD|WS_TABSTOP|BS_AUTOCHECKBOX, x, y, w, h, id)
	visible := a.createControl("BUTTON", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, x, y, w, h, id)
	a.controls[id] = state
	if a.checkboxVisible == nil {
		a.checkboxVisible = make(map[int]uintptr)
	}
	if a.tooltipControls == nil {
		a.tooltipControls = make(map[int]uintptr)
	}
	a.checkboxVisible[id] = visible
	a.tooltipControls[id] = visible
	return visible
}
func (a *appState) createCombo(x, y, w, h, id int) uintptr {
	// Capture-friendly picker: the closed selector is a normal child button and the
	// option list is also a child of this window. Unlike a native COMBOBOX dropdown,
	// it remains visible in window-only screen sharing/capture.
	if a.comboOptions == nil {
		a.comboOptions = make(map[int][]string)
	}
	if a.comboSelected == nil {
		a.comboSelected = make(map[int]int)
	}
	a.comboOptions[id] = nil
	a.comboSelected[id] = 0
	return a.createButton("Select...  v", x, y, w, 26, id)
}
func (a *appState) isComboID(id int) bool {
	_, ok := a.comboOptions[id]
	return ok
}

func (a *appState) comboReset(id int) {
	a.comboOptions[id] = nil
	a.comboSelected[id] = 0
	if h := a.controls[id]; h != 0 {
		setText(h, "Select...  v")
	}
}

func (a *appState) comboAdd(id int, text string) {
	a.comboOptions[id] = append(a.comboOptions[id], text)
	if len(a.comboOptions[id]) == 1 {
		a.comboSelected[id] = 0
		setText(a.controls[id], text+"  v")
	}
}

func (a *appState) ensureComboList() uintptr {
	if a.comboList != 0 {
		return a.comboList
	}
	list, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16ptr("LISTBOX"))),
		0,
		WS_CHILD|WS_TABSTOP|WS_BORDER|WS_VSCROLL|LBS_NOTIFY,
		0, 0, 100, 100,
		a.hwnd, uintptr(idComboList), 0, 0,
	)
	a.comboList = list
	if list != 0 && a.font != 0 {
		procSendMessageW.Call(list, WM_SETFONT, a.font, 1)
	}
	return list
}

func (a *appState) toggleCombo(id int) {
	if !a.isComboID(id) {
		return
	}
	if a.comboActiveID == id && a.comboList != 0 {
		a.closeCombo(false)
		return
	}
	a.closeCombo(false)
	list := a.ensureComboList()
	if list == 0 {
		return
	}
	procSendMessageW.Call(list, LB_RESETCONTENT, 0, 0)
	opts := a.comboOptions[id]
	for _, opt := range opts {
		procSendMessageW.Call(list, LB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16ptr(opt))))
	}
	idx := a.comboSelected[id]
	if idx < 0 || idx >= len(opts) {
		idx = 0
	}
	if len(opts) > 0 {
		procSendMessageW.Call(list, LB_SETCURSEL, uintptr(idx), 0)
	}

	button := a.controls[id]
	var br rect
	if ok, _, _ := procGetWindowRect.Call(button, uintptr(unsafe.Pointer(&br))); ok == 0 {
		return
	}
	top := point{X: br.Left, Y: br.Top}
	bottom := point{X: br.Left, Y: br.Bottom}
	procScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&top)))
	procScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&bottom)))
	width := int(br.Right - br.Left)
	height := len(opts)*22 + 6
	if height < 56 {
		height = 56
	}
	if height > 220 {
		height = 220
	}
	var client rect
	procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&client)))
	y := int(bottom.Y)
	if y+height > int(client.Bottom) && int(top.Y)-height >= 0 {
		y = int(top.Y) - height
	}
	// The selector is intentionally an in-window child so window-only capture can see it.
	// Native sibling controls can otherwise paint over an overlapping child listbox on
	// some Windows configurations. Position it at the top of the child Z-order, hide
	// only the visible siblings underneath its rectangle, then bring it to the top again.
	procSetWindowPos.Call(list, 0, uintptr(int(top.X)), uintptr(y), uintptr(width), uintptr(height), SWP_SHOWWINDOW)
	a.hideControlsBehindCombo(list, button)
	procBringWindowToTop.Call(list)
	procSetFocus.Call(list)
	a.comboActiveID = id
}

func rectsOverlap(a, b rect) bool {
	return a.Left < b.Right && a.Right > b.Left && a.Top < b.Bottom && a.Bottom > b.Top
}

func (a *appState) restoreComboHiddenControls() {
	for _, h := range a.comboHidden {
		if h != 0 {
			procShowWindow.Call(h, SW_SHOW)
		}
	}
	a.comboHidden = a.comboHidden[:0]
}

func (a *appState) hideControlsBehindCombo(list, comboButton uintptr) {
	a.restoreComboHiddenControls()
	if list == 0 {
		return
	}
	var listRect rect
	if ok, _, _ := procGetWindowRect.Call(list, uintptr(unsafe.Pointer(&listRect))); ok == 0 {
		return
	}
	// baseRects tracks every child control created by createControl, including
	// labels that do not have command IDs and therefore are not in a.controls.
	for h := range a.baseRects {
		if h == 0 || h == list || h == comboButton {
			continue
		}
		if vis, _, _ := procIsWindowVisible.Call(h); vis == 0 {
			continue
		}
		var rc rect
		if ok, _, _ := procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&rc))); ok == 0 {
			continue
		}
		if rectsOverlap(listRect, rc) {
			procShowWindow.Call(h, SW_HIDE)
			a.comboHidden = append(a.comboHidden, h)
		}
	}
	procBringWindowToTop.Call(list)
}

func (a *appState) closeComboIfPointerOutside() {
	if a.comboList == 0 || a.comboActiveID == 0 {
		return
	}
	var pt point
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); ok == 0 {
		return
	}
	var rc rect
	if ok, _, _ := procGetWindowRect.Call(a.comboList, uintptr(unsafe.Pointer(&rc))); ok == 0 {
		return
	}
	if pt.X >= rc.Left && pt.X < rc.Right && pt.Y >= rc.Top && pt.Y < rc.Bottom {
		return
	}
	// Let a click on the selector button itself reach toggleCombo so it behaves
	// like a normal dropdown button (clicking it again closes the open list).
	if button := a.controls[a.comboActiveID]; button != 0 {
		var br rect
		if ok, _, _ := procGetWindowRect.Call(button, uintptr(unsafe.Pointer(&br))); ok != 0 {
			if pt.X >= br.Left && pt.X < br.Right && pt.Y >= br.Top && pt.Y < br.Bottom {
				return
			}
		}
	}
	a.closeCombo(false)
}

func (a *appState) closeCombo(apply bool) {
	if a.comboList == 0 || a.comboActiveID == 0 {
		return
	}
	id := a.comboActiveID
	if apply {
		r, _, _ := procSendMessageW.Call(a.comboList, LB_GETCURSEL, 0, 0)
		idx := int(int32(r))
		if idx >= 0 && idx < len(a.comboOptions[id]) {
			a.comboSelected[id] = idx
			setText(a.controls[id], a.comboOptions[id][idx]+"  v")
		}
	}
	procShowWindow.Call(a.comboList, SW_HIDE)
	a.restoreComboHiddenControls()
	a.comboActiveID = 0
	if apply {
		a.onCommand(id, CBN_SELCHANGE)
	}
}

func (a *appState) createEdit(text string, x, y, w, h, id int) uintptr {
	return a.createControl("EDIT", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_NUMBER, x, y, w, h, id)
}

// createSignedEdit allows a leading minus (ES_NUMBER would block it); values are validated on commit.
func (a *appState) createSignedEdit(text string, x, y, w, h, id int) uintptr {
	return a.createControl("EDIT", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER, x, y, w, h, id)
}

func (a *appState) initTooltips() {
	if a.tooltipHwnd != 0 {
		return
	}
	h, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(utf16ptr("tooltips_class32"))), 0, WS_POPUP|TTS_ALWAYSTIP|TTS_NOPREFIX, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, a.hwnd, 0, 0, 0)
	a.tooltipHwnd = h
	if h != 0 {
		procSendMessageW.Call(h, TTM_SETMAXTIPWIDTH, 0, 420)
	}
}

func (a *appState) addTooltip(control uintptr, text string) {
	if control == 0 || text == "" {
		return
	}
	if a.tooltipHelp == nil {
		a.tooltipHelp = make(map[uintptr]string)
	}
	// v1.96 deliberately keeps help inside the main application window. Native Win32
	// tooltip/dropdown popup windows can float over unrelated apps and can disappear
	// from window-only screen capture.
	a.tooltipHelp[control] = text
}

func (a *appState) tooltipForID(id int, text string) {
	h := a.controls[id]
	if a.tooltipControls[id] != 0 {
		h = a.tooltipControls[id]
	}
	a.addTooltip(h, text)
}

// updateHoverTooltip renders contextual help into the main window instead of a
// separate top-most tooltip window. This keeps help capture-friendly and prevents
// it from floating above unrelated applications.
func (a *appState) updateHoverTooltip() {
	if len(a.tooltipHelp) == 0 || a.hwnd == 0 || a.controls[idHelpText] == 0 {
		return
	}
	var pt point
	if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return
	}
	var hovered uintptr
	help := ""
	for h, text := range a.tooltipHelp {
		if vis, _, _ := procIsWindowVisible.Call(h); vis == 0 {
			continue
		}
		var rc rect
		if r, _, _ := procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&rc))); r == 0 {
			continue
		}
		if pt.X >= rc.Left && pt.X < rc.Right && pt.Y >= rc.Top && pt.Y < rc.Bottom {
			hovered, help = h, text
			break
		}
	}
	if hovered == 0 {
		if a.tooltipHover != 0 {
			setText(a.controls[idHelpText], "Help: hover over a control to see what it does.")
			a.tooltipHover = 0
		}
		return
	}
	if hovered != a.tooltipHover {
		setText(a.controls[idHelpText], "Help: "+help)
		a.tooltipHover = hovered
	}
}

func (a *appState) updateScrollbars() {
	if a.hwnd == 0 {
		return
	}
	var rc rect
	if ok, _, _ := procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&rc))); ok == 0 {
		return
	}
	minX, maxX, pageX := a.scrollRange(true)
	minY, maxY, pageY := a.scrollRange(false)
	a.scrollX = clampScrollPos(a.scrollX, minX, maxX)
	a.scrollY = clampScrollPos(a.scrollY, minY, maxY)
	showH := maxX > minX
	showV := shouldShowVerticalScrollBar(minY, maxY)
	a.placeScrollBar(a.verticalBar, verticalScrollBarBounds(rc), showV, minY, maxY, pageY, a.scrollY)
	a.placeScrollBar(a.horizontalBar, horizontalScrollBarBounds(rc, showV), showH, minX, maxX, pageX, a.scrollX)
	a.repositionScrolledChildren()
}

func (a *appState) placeScrollBar(bar *customScrollBar, bounds rect, visible bool, minPos, maxPos, page, pos int) {
	if bar == nil {
		return
	}
	if !visible {
		minPos, maxPos, pos = 0, 0, 0
	}
	procSetWindowPos.Call(
		bar.hwnd, HWND_TOP,
		uintptr(bounds.Left), uintptr(bounds.Top),
		uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top),
		SWP_NOACTIVATE,
	)
	a.setScrollBarState(bar, minPos, maxPos, page, pos)
	visibility := uintptr(SW_HIDE)
	if visible {
		visibility = SW_SHOW
	}
	procShowWindow.Call(bar.hwnd, visibility)
}

// scrollRange returns min, max and page size for one axis based on the current client size.
func (a *appState) scrollRange(horizontal bool) (int, int, int) {
	var rc rect
	procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&rc)))
	cw := int(rc.Right - rc.Left)
	// Content ends above the status bar, so vertical limits match contentViewport.
	ch := int(statusBarBounds(rc).Top - rc.Top)
	// Reserve the scrollbar strip plus resize border so the scroll limits match contentViewport.
	const barReserve = scrollBarWidth + windowResizeBorder
	maxX := a.contentWidth - (cw - barReserve)
	if maxX < 0 {
		maxX = 0
	}
	if horizontal {
		return 0, maxX, cw - sidebarWidth - barReserve
	}
	if maxX > 0 {
		ch -= scrollBarWidth
	}
	minY, maxY := navigationScrollRange(a.cards, a.topNavigationID, ch)
	if maxY < minY {
		maxY = minY
	}
	return minY, maxY, ch - fixedProfileBottom
}
func shouldShowVerticalScrollBar(minY, maxY int) bool {
	return maxY > minY
}

func verticalScrollBarBounds(client rect) rect {
	right := client.Right - windowResizeBorder
	if right < client.Left {
		right = client.Left
	}
	left := right - scrollBarWidth
	if left < client.Left {
		left = client.Left
	}
	top := int32(fixedProfileBottom + 2)
	if top < client.Top {
		top = client.Top
	}
	bottom := statusBarBounds(client).Top
	if bottom < top {
		bottom = top
	}
	return rect{
		Left:   left,
		Top:    top,
		Right:  right,
		Bottom: bottom,
	}
}

func (a *appState) redrawAfterScroll() {
	procRedrawWindow.Call(a.hwnd, 0, 0, RDW_INVALIDATE|RDW_ERASE|RDW_ALLCHILDREN|RDW_UPDATENOW)
}

func (a *appState) redrawLiveThrustMeters() {
	for _, card := range a.cards {
		if !liveThrustMeterVisible(card, a.topNavigationID, a.selectedControlMode()) {
			continue
		}
		dirty := liveThrustMeterDirtyRect(card, a.scrollX, a.scrollY)
		procRedrawWindow.Call(a.hwnd, uintptr(unsafe.Pointer(&dirty)), 0, RDW_INVALIDATE|RDW_NOERASE|RDW_NOCHILDREN)
		runtime.KeepAlive(&dirty)
	}
}

func liveThrustMeterVisible(card uiCard, section int, profile string) bool {
	return card.meter != "" && card.section == section &&
		(card.profile == "" || strings.EqualFold(card.profile, profile))
}

func liveThrustMeterDirtyRect(card uiCard, scrollX, scrollY int) rect {
	return rect{
		Left:   card.bounds.Left + 20 - int32(scrollX),
		Top:    card.bounds.Top + 148 - int32(scrollY),
		Right:  card.bounds.Right - 20 - int32(scrollX),
		Bottom: card.bounds.Top + 181 - int32(scrollY),
	}
}

func (a *appState) repositionScrolledChildren() {
	// Clip before moving and never copy old child pixels: a bit-copied move without the
	// new region painted scrolled controls over the title/profile bar.
	view := a.currentContentViewport()
	dx, dy := int32(a.scrollX), int32(a.scrollY)
	for h, r := range a.baseRects {
		if h != 0 {
			a.applyChildClip(h, rect{Left: r.Left - dx, Top: r.Top - dy, Right: r.Right - dx, Bottom: r.Bottom - dy}, view)
		}
	}
	const moveFlags = SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_NOCOPYBITS
	// Batch every child move into one layout transaction. Moving each child independently
	// caused visible tearing/stale separator pixels while dragging the scrollbars.
	hdwp, _, _ := procBeginDeferWindowPos.Call(uintptr(len(a.baseRects)))
	for h, r := range a.baseRects {
		if h == 0 {
			continue
		}
		x := int(r.Left) - a.scrollX
		y := int(r.Top) - a.scrollY
		if hdwp != 0 {
			// A failed DeferWindowPos invalidates the whole transaction (handle freed by the system).
			hdwp, _, _ = procDeferWindowPos.Call(hdwp, h, 0, uintptr(x), uintptr(y), 0, 0, moveFlags)
			if hdwp == 0 {
				break
			}
		}
	}
	if hdwp != 0 {
		procEndDeferWindowPos.Call(hdwp)
	} else {
		for h, r := range a.baseRects {
			if h == 0 {
				continue
			}
			procSetWindowPos.Call(h, 0, uintptr(int(r.Left)-a.scrollX), uintptr(int(r.Top)-a.scrollY), 0, 0, moveFlags)
		}
	}
	a.redrawAfterScroll()
}

func (a *appState) updateContentExtent() {
	var width, height int32
	for _, r := range a.baseRects {
		if r.Right > width {
			width = r.Right
		}
		if r.Bottom > height {
			height = r.Bottom
		}
	}
	for _, card := range a.cards {
		if card.bounds.Right > width {
			width = card.bounds.Right
		}
		if card.bounds.Bottom > height {
			height = card.bounds.Bottom
		}
	}
	a.contentWidth = int(width) + 20
	a.contentHeight = int(height) + 20
}

func (a *appState) handleMouseWheel(wParam uintptr) {
	delta := int(int16(hiword(wParam)))
	if delta == 0 {
		return
	}
	steps := delta / 120
	if steps == 0 {
		if delta > 0 {
			steps = 1
		} else {
			steps = -1
		}
	}
	// Wheel-up scrolls toward the top; three 20px logical lines per wheel notch.
	a.scrollTo(false, a.scrollY-steps*scrollBarLineStep)
}
func hidhidePaths() (cli, client string, installed bool) {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		return "", "", false
	}
	root := filepath.Join(pf, "Nefarius Software Solutions", "HidHide")
	candidatesCLI := []string{filepath.Join(root, "HidHideCLI.exe"), filepath.Join(root, "x64", "HidHideCLI.exe")}
	candidatesClient := []string{filepath.Join(root, "HidHideClient.exe"), filepath.Join(root, "x64", "HidHideClient.exe")}
	for _, p := range candidatesCLI {
		if _, err := os.Stat(p); err == nil {
			cli = p
			break
		}
	}
	for _, p := range candidatesClient {
		if _, err := os.Stat(p); err == nil {
			client = p
			break
		}
	}
	return cli, client, cli != "" && client != ""
}

func (a *appState) refreshHidHideUI() {
	h := a.controls[idHidHideStatus]
	if h == 0 {
		return
	}
	_, _, ok := hidhidePaths()
	if ok {
		setText(h, "HidHide: installed (optional)")
	} else {
		setText(h, "HidHide: not installed (optional)")
	}
}

func (a *appState) openHidHideSetup() {
	cli, client, ok := hidhidePaths()
	if !ok {
		a.postStatus("HidHide is not installed. Opening the official HidHide releases page. It is optional and used to prevent double input.")
		procShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16ptr("open"))), uintptr(unsafe.Pointer(utf16ptr("https://github.com/nefarius/HidHide/releases"))), 0, 0, SW_SHOW)
		return
	}
	exePath, err := os.Executable()
	if err != nil {
		exePath = ""
	}
	go func() {
		registered := false
		if exePath != "" {
			cmd := exec.Command(cli, "--inv-off", "--app-reg", exePath)
			if out, err := cmd.CombinedOutput(); err == nil {
				registered = true
			} else if len(out) > 0 {
				a.postStatus("HidHide is installed, but automatic app registration was busy/blocked. The configuration client will still open; add this EXE on the Applications tab if needed.")
			}
		}
		if err := exec.Command(client).Start(); err != nil {
			a.postStatus("Could not open HidHide Configuration Client: " + err.Error())
			return
		}
		if registered {
			a.postStatus("HidHide opened and this mapper was added to its application allowlist. Choose the physical flight devices to hide, enable device hiding, then reconnect them.")
		} else {
			a.postStatus("HidHide opened. Add this mapper on Applications, select the physical flight devices on Devices, enable hiding, then reconnect them.")
		}
	}()
}

func (a *appState) setGroupVisible(group []uintptr, visible bool) {
	for _, h := range group {
		if h != 0 {
			if a.contextVisible == nil {
				a.contextVisible = make(map[uintptr]bool)
			}
			a.contextVisible[h] = visible
			a.setControlVisibility(h, visible && a.controlPageVisible(h))
		}
	}
}

func (a *appState) setControlVisibility(hwnd uintptr, visible bool) {
	command := uintptr(SW_HIDE)
	if visible {
		command = SW_SHOW
	}
	procShowWindow.Call(hwnd, command)
}

func (a *appState) controlPageVisible(hwnd uintptr) bool {
	if section, ok := a.controlSections[hwnd]; ok {
		return section == a.topNavigationID
	}
	_, scrollable := a.baseRects[hwnd]
	return !scrollable
}

func (a *appState) applyNavigationVisibility() {
	for hwnd := range a.baseRects {
		contextVisible, exists := a.contextVisible[hwnd]
		if !exists {
			contextVisible = true
		}
		a.setControlVisibility(hwnd, contextVisible && a.controlPageVisible(hwnd))
	}
}

func (a *appState) updateContextVisibility() {
	if !a.uiReady {
		return
	}
	controlModes := []string{"HOSAS", "HOTAS"}
	ci := comboIndex(idControlMode)
	if ci < 0 || ci >= len(controlModes) {
		ci = 0
	}
	cm := controlModes[ci]
	a.setGroupVisible(a.controlHOSASGroup, strings.EqualFold(cm, "HOSAS"))
	isHOTAS := strings.EqualFold(cm, "HOTAS")
	a.setGroupVisible(a.controlHOTASGroup, isHOTAS)
	a.setGroupVisible(a.hosasDrivingGroup, strings.EqualFold(cm, "HOSAS"))
	brakeModes := []string{"Off", "Digital at Idle", "Analog Brake Zone"}
	bi := comboIndex(idHotasBrakeBehavior)
	if bi < 0 || bi >= len(brakeModes) {
		bi = 1
	}
	bm := brakeModes[bi]
	a.setGroupVisible(a.hotasBrakeDigitalGroup, isHOTAS && bm == "Digital at Idle")
	a.setGroupVisible(a.hotasBrakeAnalogGroup, isHOTAS && bm == "Analog Brake Zone")
	pedalModes := []string{"Off", "Fine steering", "Move Camera", "Pitch", "Keyboard keys"}
	pi := comboIndex(idPedalMode)
	if pi < 0 || pi >= len(pedalModes) {
		pi = 0
	}
	pm := pedalModes[pi]
	a.setGroupVisible(a.pedalFineGroup, strings.EqualFold(pm, "Fine steering"))
	a.setGroupVisible(a.pedalCameraGroup, strings.EqualFold(pm, "Move Camera"))
	a.setGroupVisible(a.pedalKeyGroup, strings.EqualFold(pm, "Keyboard keys"))
	for _, row := range []struct {
		modeID             int
		fine, camera, keys []uintptr
	}{
		{idTwistLeftMode, a.twistLeftFineGroup, a.twistLeftCameraGroup, a.twistLeftKeyGroup},
		{idTwistRightMode, a.twistRightFineGroup, a.twistRightCameraGroup, a.twistRightKeyGroup},
	} {
		mi := comboIndex(row.modeID)
		if mi < 0 || mi >= len(pedalModes) {
			mi = 0
		}
		a.setGroupVisible(row.fine, strings.EqualFold(pedalModes[mi], "Fine steering"))
		a.setGroupVisible(row.camera, strings.EqualFold(pedalModes[mi], "Move Camera"))
		a.setGroupVisible(row.keys, strings.EqualFold(pedalModes[mi], "Keyboard keys"))
	}
	// Negative polarity is a lever-only option. Invert continues to reverse travel.
	for _, twist := range []struct{ axisTypeID, negativeID int }{
		{idTwistLeftAxisType, idTwistLeftOutputNegative},
		{idTwistRightAxisType, idTwistRightOutputNegative},
	} {
		a.setGroupVisible([]uintptr{a.checkboxVisible[twist.negativeID]}, comboIndex(twist.axisTypeID) == 1)
	}

	onFootModes := []string{"Off", "Thumbstick -> WASD", "Buttons -> WASD"}
	oi := comboIndex(idOnFootMode)
	if oi < 0 || oi >= len(onFootModes) {
		oi = 0
	}
	om := onFootModes[oi]
	a.setGroupVisible(a.onFootThumbGroup, strings.EqualFold(om, "Thumbstick -> WASD"))
	a.setGroupVisible(a.onFootButtonGroup, strings.EqualFold(om, "Buttons -> WASD"))
}

func (a *appState) createTwistUI(left bool, y, detailY int) {
	ids := auxiliaryIDs(left)
	name := "Right Twist Axis"
	if left {
		name = "Left Twist Axis"
	}
	a.createLabel(name, 20, y+4, 125, 23)
	a.createCombo(150, y, 232, 250, ids[0])
	a.createCombo(392, y, 83, 210, ids[1])
	a.createCheck("Invert", 483, y+1, 75, 24, ids[2])
	a.createLabel("Type", 555, y+4, 38, 22)
	a.createCombo(594, y, 103, 120, ids[3])
	a.createLabel("DZ%", 703, y+4, 38, 22)
	a.createEdit("6", 741, y, 41, 25, ids[4])
	a.createLabel("Mode", 793, y+4, 43, 22)
	a.createCombo(840, y, 267, 160, ids[5])
	// Separate from Invert; placed on the detail row without overlapping mode options.
	// Only visible for Lever axis type (see updateContextVisibility).
	a.createCheck("Output Negative", 915, detailY+1, 176, 24, ids[12])
	var fine, camera, keys []uintptr
	fine = append(fine,
		a.createLabel("Fine-steer authority", 150, detailY+4, 135, 24),
		a.createEdit("30", 290, detailY, 52, 25, ids[6]),
		a.createLabel("%", 347, detailY+4, 20, 22))
	camera = append(camera,
		a.createLabel("Camera sensitivity", 150, detailY+4, 132, 24),
		a.createEdit("100", 285, detailY, 54, 25, ids[7]),
		a.createLabel("%", 343, detailY+4, 20, 22),
		a.createLabel("Camera deadzone", 387, detailY+4, 120, 24),
		a.createEdit("6", 511, detailY, 52, 25, ids[8]),
		a.createLabel("%", 569, detailY+4, 20, 22))
	keys = append(keys,
		a.createLabel("Negative key", 150, detailY+4, 89, 24),
		a.createCombo(244, detailY, 160, 230, ids[9]),
		a.createLabel("Positive key", 418, detailY+4, 91, 24),
		a.createCombo(513, detailY, 160, 230, ids[10]),
		a.createLabel("Threshold %", 692, detailY+4, 98, 24),
		a.createEdit("20", 792, detailY, 54, 25, ids[11]))
	if left {
		a.twistLeftFineGroup, a.twistLeftCameraGroup, a.twistLeftKeyGroup = fine, camera, keys
	} else {
		a.twistRightFineGroup, a.twistRightCameraGroup, a.twistRightKeyGroup = fine, camera, keys
	}
}

func (a *appState) createSectionPanel(title string, x, y, w, h int) {
	profile, meter := "", ""
	switch title {
	case "LEFT STICK":
		profile, meter = "HOSAS", "left"
	case "RIGHT STICK":
		profile, meter = "HOSAS", "right"
	case liveCardTitle:
		profile = "HOSAS"
	case "THROTTLE", "FLIGHT STICK":
		profile = "HOTAS"
	}
	a.cards = append(a.cards, uiCard{
		title:   title,
		bounds:  rect{Left: int32(contentOffset + x), Top: int32(y), Right: int32(contentOffset + x + w), Bottom: int32(y + h)},
		section: navigationSectionForTitle(title),
		profile: profile,
		curve:   title == "DRIVING RESPONSE",
		meter:   meter,
		live:    title == liveCardTitle,
	})
}

func navigationSectionForTitle(title string) int {
	switch title {
	case "INPUT & DEVICE TOOLS":
		return idSectionDevices
	case "TWIST AXES", "RUDDER PEDALS":
		return idSectionTwist
	case "ON-FOOT CONTROLS":
		return idSectionFoot
	case "VIRTUAL CONTROLLER & LIVE MONITOR":
		return idSectionMonitor
	case "BUTTON MAPPINGS":
		return idSectionBindings
	default:
		return idSectionSetup
	}
}

func navigationScrollRange(cards []uiCard, section, clientHeight int) (int, int) {
	if section == 0 {
		return 0, 0
	}
	top, bottom, found := 0, 0, false
	for _, card := range cards {
		if card.section != section {
			continue
		}
		if !found || int(card.bounds.Top) < top {
			top = int(card.bounds.Top)
		}
		if !found || int(card.bounds.Bottom) > bottom {
			bottom = int(card.bounds.Bottom)
		}
		found = true
	}
	if !found {
		return 0, 0
	}
	minimum := 0
	if section != idSectionSetup && section != idSectionDevices {
		minimum = top - customTitleBarHeight - sectionNavInset
		if minimum < 0 {
			minimum = 0
		}
	}
	maximum := bottom - clientHeight
	if maximum < minimum {
		maximum = minimum
	}
	return minimum, maximum
}

func fitSectionCardToControls(cards []uiCard, section int, controlSections map[uintptr]int, controlRects map[uintptr]rect, padding int32) {
	var contentBottom int32
	found := false
	for hwnd, owner := range controlSections {
		if owner != section {
			continue
		}
		bounds, ok := controlRects[hwnd]
		if ok && (!found || bounds.Bottom > contentBottom) {
			contentBottom = bounds.Bottom
			found = true
		}
	}
	if !found {
		return
	}
	for i := range cards {
		if cards[i].section == section {
			cards[i].bounds.Bottom = contentBottom + padding
			return
		}
	}
}

func sectionContentYOffset(section int) int {
	switch section {
	case idSectionTwist:
		return layoutShift + 16
	case idSectionFoot:
		return layoutShift + 42
	case idSectionSetup:
		return layoutShift - 430
	case idSectionMonitor:
		return layoutShift + 116
	case idSectionBindings:
		return layoutShift - 30
	default:
		return layoutShift
	}
}

func (a *appState) createSectionPanels() {
	a.createSectionPanel("LEFT STICK", 10, 40, 400, 200)
	a.createSectionPanel("RIGHT STICK", 415, 40, 400, 200)
	a.createSectionPanel(liveCardTitle, 820, 40, 300, 200)
	a.createSectionPanel("THROTTLE", 10, 40, 400, 200)
	a.createSectionPanel("FLIGHT STICK", 415, 40, 400, 200)
	a.createSectionPanel("INPUT & DEVICE TOOLS", 10, sectionDevicesPanelY, 1110, 164)
	a.createSectionPanel("TWIST AXES", 10, sectionTwistPanelY, 1110, 178)
	a.createSectionPanel("RUDDER PEDALS", 10, sectionPedalPanelY, 1110, 110)
	a.createSectionPanel("ON-FOOT CONTROLS", 10, sectionFootPanelY, 1110, 140)
	a.createSectionPanel("DRIVING RESPONSE", 10, sectionDrivingPanelY, 1110, 280)
	a.createSectionPanel("VIRTUAL CONTROLLER & LIVE MONITOR", 10, sectionMonitorPanelY, 1110, 350)
	a.createSectionPanel("BUTTON MAPPINGS", 10, sectionBindingPanelY, 1110, 782)
	a.updateContentExtent()
}

func (a *appState) createSidebar() {
	a.sidebarDivider = a.createFixedControl("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, sidebarWidth-1, customTitleBarHeight, 1, 900, 0)
	a.sidebarTitle = a.createFixedControl("STATIC", "GALACTIC RACER\r\nBRIDGE", WS_CHILD|WS_VISIBLE|SS_LEFT, 16, customTitleBarHeight+20, sidebarWidth-32, 54, 0)
	a.sidebarFooter = a.createFixedControl("STATIC", "NATIVE WINDOWS APP", WS_CHILD|WS_VISIBLE|SS_LEFT, 16, 0, sidebarWidth-32, 24, 0)
	items := []struct {
		id   int
		text string
	}{
		{idSectionSetup, "Steuerung"},
		{idSectionBindings, "Buttons"},
		{idSectionTwist, "Twist-Achsen"},
		{idSectionFoot, "Zu Fuss"},
		{idSectionDevices, "Eingabe & HidHide"},
		{idSectionMonitor, "Live Monitor"},
	}
	for i, item := range items {
		h := a.createFixedControl("BUTTON", item.text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 16, sidebarNavigationY(i), 168, sidebarNavigationHeight, item.id)
		a.sidebarNavigation = append(a.sidebarNavigation, h)
	}
}

func (a *appState) createCaptionButtons() {
	a.createFixedControl("BUTTON", "—", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 0, 6, captionButtonWidth, captionButtonHeight, idWindowMinimize)
	a.createFixedControl("BUTTON", "□", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 0, 6, captionButtonWidth, captionButtonHeight, idWindowMaximize)
	a.createFixedControl("BUTTON", "×", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 0, 6, captionButtonWidth, captionButtonHeight, idWindowClose)
}

func (a *appState) updateCaptionButtons(sizeState uintptr) {
	if maximize := a.controls[idWindowMaximize]; maximize != 0 {
		if sizeState == SIZE_MAXIMIZED {
			setText(maximize, "❐")
		} else if sizeState == SIZE_RESTORED {
			setText(maximize, "□")
		}
	}
}

func (a *appState) resizeCaptionButtons() {
	if a.hwnd == 0 {
		return
	}
	var client rect
	if ok, _, _ := procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return
	}
	right := int(client.Right) - captionButtonInset
	for _, id := range []int{idWindowClose, idWindowMaximize, idWindowMinimize} {
		h := a.controls[id]
		if h == 0 {
			continue
		}
		right -= captionButtonWidth
		procSetWindowPos.Call(h, HWND_TOP, uintptr(right), 6, captionButtonWidth, captionButtonHeight, SWP_NOACTIVATE)
	}
}

const (
	sidebarNavigationTop    = customTitleBarHeight + 86
	sidebarNavigationStep   = 46
	sidebarNavigationHeight = 36
)

func sidebarNavigationY(index int) int {
	return sidebarNavigationTop + index*sidebarNavigationStep
}

// sidebarNavigationFits reports whether navigation entry index ends above bottom.
func sidebarNavigationFits(index, bottom int) bool {
	return sidebarNavigationY(index)+sidebarNavigationHeight <= bottom
}

func (a *appState) resizeSidebar() {
	if a.hwnd == 0 || a.sidebarTitle == 0 {
		return
	}
	var client rect
	if ok, _, _ := procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return
	}
	top := customTitleBarHeight
	height := int(statusBarBounds(client).Top) - top
	if height < 0 {
		height = 0
	}
	procSetWindowPos.Call(a.sidebarDivider, HWND_TOP, sidebarWidth-1, uintptr(top), 1, uintptr(height), SWP_NOACTIVATE)
	procSetWindowPos.Call(a.sidebarTitle, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
	for i, h := range a.sidebarNavigation {
		// Child windows are not clipped by the parent paint, so hide entries that would cover the status bar.
		visibility := uintptr(SWP_SHOWWINDOW)
		if !sidebarNavigationFits(i, top+height) {
			visibility = SWP_HIDEWINDOW
		}
		procSetWindowPos.Call(h, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|visibility)
	}
	if a.sidebarFooter != 0 {
		footerY := top + height - 32
		if footerY < 0 {
			footerY = 0
		}
		procSetWindowPos.Call(a.sidebarFooter, HWND_TOP, 16, uintptr(footerY), 168, 24, SWP_NOACTIVATE)
	}
}

// valueSlider couples a numeric edit with a native trackbar (HOSAS driving response).
type valueSlider struct {
	editID    int
	min, max  int
	name      string
	normalize func(int) int
	hwnd      uintptr
}

func (a *appState) createValueSlider(x, y, id int, spec valueSlider, initial int) uintptr {
	h := a.createControl("msctls_trackbar32", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP, x, y, 170, 28, id)
	if h == 0 {
		panic("Could not create native " + spec.name + " trackbar.")
	}
	procSendMessageW.Call(h, TBM_SETRANGEMIN, 0, uintptr(spec.min))
	procSendMessageW.Call(h, TBM_SETRANGEMAX, 1, uintptr(spec.max))
	procSendMessageW.Call(h, TBM_SETPOS, 1, uintptr(initial))
	spec.hwnd = h
	a.valueSliders = append(a.valueSliders, spec)
	return h
}

func (a *appState) valueSliderForHandle(hwnd uintptr) (valueSlider, bool) {
	for _, s := range a.valueSliders {
		if hwnd != 0 && s.hwnd == hwnd {
			return s, true
		}
	}
	return valueSlider{}, false
}

func (a *appState) valueSliderForEdit(id int) (valueSlider, bool) {
	for _, s := range a.valueSliders {
		if s.editID == id {
			return s, true
		}
	}
	return valueSlider{}, false
}

func (a *appState) setValueSlider(editID, value int) {
	if s, ok := a.valueSliderForEdit(editID); ok {
		procSendMessageW.Call(s.hwnd, TBM_SETPOS, 1, uintptr(value))
	}
}

func (a *appState) handleValueSlider(spec valueSlider, code int) {
	value, _, _ := procSendMessageW.Call(spec.hwnd, TBM_GETPOS, 0, 0)
	n := int(int32(value))
	if n < spec.min {
		n = spec.min
	}
	if n > spec.max {
		n = spec.max
	}
	a.sliderChanging = true
	setText(a.controls[spec.editID], strconv.Itoa(n))
	a.sliderChanging = false
	a.syncConfigFromUI()
	procRedrawWindow.Call(a.hwnd, 0, 0, RDW_INVALIDATE)
	if code == TB_ENDTRACK {
		saveConfig()
	}
}

func (a *appState) navigateToSection(id int) {
	a.closeCombo(false)
	a.topNavigationID = id
	a.scrollX = 0
	a.scrollY, _ = navigationScrollRange(a.cards, id, 0)
	a.applyNavigationVisibility()
	a.raiseLiveMonitorDots()
	a.updateScrollbars()
	a.redrawAfterScroll()
}

func (a *appState) createUI() {
	f, _, _ := procGetStockObject.Call(DEFAULT_GUI_FONT)
	a.font = f

	a.createSidebar()
	a.createCaptionButtons()
	a.verticalBar = a.createCustomScrollBar(false)
	a.horizontalBar = a.createCustomScrollBar(true)
	a.activeControlSection = idSectionSetup
	profileLabel := a.createLabel("CONTROL PROFILE", 20, 12, 140, 24)
	modeCombo := a.createCombo(830, 44, 185, 160, idControlMode)
	procShowWindow.Call(modeCombo, SW_HIDE)
	delete(a.baseRects, modeCombo)
	hosasProfileButton := a.createButton("HOSAS / Podracer", 165, 8, 154, 30, idSelectHOSAS)
	hotasProfileButton := a.createButton("HOTAS / Traditional", 324, 8, 170, 30, idSelectHOTAS)
	a.activeControlSection = idSectionDevices
	a.createLabel("Input API", 20, 80, 90, 22)
	a.createCombo(115, 74, 185, 140, idInputBackend)
	a.createButton("Refresh devices", 20, 107, 120, 28, idRefresh)
	a.createButton("HidHide Setup", 150, 107, 150, 28, idHidHideSetup)
	a.createControl("STATIC", "HidHide: checking...", WS_CHILD|WS_VISIBLE|SS_LEFT, 20, 136, 280, 18, idHidHideStatus)

	a.activeControlSection = idSectionSetup
	// Podracer Style controls. Hidden automatically when Traditional HOTAS is selected.
	a.controlHOSASGroup = append(a.controlHOSASGroup,
		a.createLabel("Device", 30, 72, 150, 20),
		a.createLabel("Forward axis", 220, 72, 100, 20),
		a.createCombo(30, 92, 180, 260, idLeftDevice),
		a.createCombo(220, 92, 65, 220, idLeftAxis),
		a.createCheck("Invert", 294, 93, 74, 24, idLeftInvert),
		a.createCheck("Throttle axis", 30, 122, 112, 24, idLeftThrottleMode),
		a.createButton("Detect Left", 154, 119, 105, 28, idDetectLeft),
		a.createLabel("Device", 435, 72, 150, 20),
		a.createLabel("Forward axis", 625, 72, 100, 20),
		a.createCombo(435, 92, 180, 260, idRightDevice),
		a.createCombo(625, 92, 65, 220, idRightAxis),
		a.createCheck("Invert", 699, 93, 74, 24, idRightInvert),
		a.createCheck("Throttle axis", 435, 122, 112, 24, idRightThrottleMode),
		a.createButton("Detect Right", 559, 119, 105, 28, idDetectRight))

	// Traditional HOTAS controls occupy the same space as the Podracer rows. Throttle behavior is configurable;
	// the selected flight stick maps directly to the virtual Xbox left thumbstick.
	a.controlHOTASGroup = append(a.controlHOTASGroup,
		a.createLabel("Throttle device", 30, 72, 150, 20),
		a.createLabel("Axis", 220, 72, 55, 20),
		a.createCombo(30, 92, 180, 260, idHotasThrottleDevice),
		a.createCombo(220, 92, 65, 220, idHotasThrottleAxis),
		a.createCheck("Invert", 294, 93, 74, 24, idHotasThrottleInvert),
		a.createCombo(30, 124, 165, 160, idHotasBrakeBehavior),
		a.createLabel("Flight stick device", 435, 72, 175, 20),
		a.createLabel("X axis", 625, 72, 55, 20),
		a.createLabel("Y axis", 700, 72, 55, 20),
		a.createCombo(435, 92, 180, 260, idHotasStickDevice),
		a.createCombo(625, 92, 65, 220, idHotasStickXAxis),
		a.createCombo(700, 92, 65, 220, idHotasStickYAxis),
		a.createCheck("Invert X", 435, 124, 80, 24, idHotasStickInvertX),
		a.createCheck("Invert Y", 525, 124, 80, 24, idHotasStickInvertY))

	a.hotasBrakeDigitalGroup = append(a.hotasBrakeDigitalGroup,
		a.createLabel("Idle %", 200, 127, 48, 22),
		a.createEdit("5", 250, 124, 40, 25, idHotasBrakeThreshold))
	a.hotasBrakeAnalogGroup = append(a.hotasBrakeAnalogGroup,
		a.createLabel("Zone %", 200, 127, 48, 22),
		a.createEdit("20", 250, 124, 40, 25, idHotasBrakeZone))
	for _, h := range a.hotasBrakeDigitalGroup {
		a.controlHOTASGroup = append(a.controlHOTASGroup, h)
	}
	for _, h := range a.hotasBrakeAnalogGroup {
		a.controlHOTASGroup = append(a.controlHOTASGroup, h)
	}
	a.activeControlSection = idSectionTwist
	a.createLabel("Pedal device", 20, 182, 115, 24)
	a.createCombo(150, 178, 240, 260, idPedalDevice)
	a.createCombo(405, 178, 85, 220, idPedalAxis)
	a.createCheck("Invert", 500, 179, 75, 24, idPedalInvert)
	a.createLabel("Pedal use", 585, 182, 80, 20)
	a.createCombo(665, 178, 170, 180, idPedalMode)

	// Pedal-specific settings share one row and are shown only for the selected pedal mode.
	a.pedalFineGroup = append(a.pedalFineGroup,
		a.createLabel("Fine-steer authority", 20, 216, 130, 24),
		a.createEdit("30", 150, 212, 55, 25, idPedalStrength),
		a.createLabel("%", 210, 216, 25, 22))
	a.pedalCameraGroup = append(a.pedalCameraGroup,
		a.createLabel("Camera sensitivity", 20, 216, 120, 24),
		a.createEdit("100", 145, 212, 55, 25, idCameraSensitivity),
		a.createLabel("%", 205, 216, 25, 22),
		a.createLabel("Camera deadzone", 285, 216, 115, 24),
		a.createEdit("6", 405, 212, 55, 25, idCameraDeadzone),
		a.createLabel("%", 465, 216, 25, 22))
	a.pedalKeyGroup = append(a.pedalKeyGroup,
		a.createLabel("Pedal left key", 20, 216, 110, 24),
		a.createCombo(130, 212, 145, 260, idPedalLeftKey),
		a.createLabel("Pedal right key", 295, 216, 115, 24),
		a.createCombo(410, 212, 145, 260, idPedalRightKey),
		a.createLabel("Threshold", 575, 216, 75, 24),
		a.createEdit("20", 650, 212, 55, 25, idPedalKeyThreshold),
		a.createLabel("%", 710, 216, 25, 22))

	a.activeControlSection = idSectionFoot
	// On-foot controls can use either an analog thumbstick (DirectInput) or learned buttons/hats.
	a.createLabel("On-foot controls", 20, 264, 120, 22)
	a.createLabel("Mode", 145, 264, 45, 22)
	a.createCombo(190, 258, 190, 180, idOnFootMode)

	// Thumbstick -> WASD contextual controls.
	a.onFootThumbGroup = append(a.onFootThumbGroup,
		a.createLabel("Threshold", 400, 264, 70, 20),
		a.createEdit("20", 472, 258, 45, 25, idWASDThreshold),
		a.createLabel("%", 520, 264, 20, 20),
		a.createCheck("Invert side", 555, 260, 92, 24, idWASDInvertSide),
		a.createCheck("Invert fwd", 655, 260, 90, 24, idWASDInvertForward),
		a.createButton("Detect Thumbstick", 765, 256, 145, 30, idDetectWASD))
	diMon := a.createControl("STATIC", "DirectInput axis monitor: waiting for game-controller data...", WS_CHILD|WS_VISIBLE|SS_LEFT, 20, 300, 1100, 52, idDIThumbMonitor)
	a.onFootThumbGroup = append(a.onFootThumbGroup, diMon)

	// Buttons -> WASD contextual controls. Each direction learns a controller button/hat/switch.
	for i, def := range wasdButtonDefs {
		x := 20 + i*275
		indicator := a.createControl("BUTTON", def.Label, WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, x, 300, 45, 28, idWASDIndicatorBase+i)
		label := a.createControl("STATIC", "Not assigned", WS_CHILD|WS_VISIBLE|SS_LEFT, x+50, 305, 95, 22, idWASDLabelBase+i)
		learn := a.createButton("Learn", x+150, 300, 50, 28, idWASDLearnBase+i)
		clear := a.createButton("Clear", x+205, 300, 50, 28, idWASDClearBase+i)
		a.onFootButtonGroup = append(a.onFootButtonGroup, indicator, label, learn, clear)
	}

	a.activeControlSection = idSectionSetup
	a.createLabel("Deadzone", 20, 420, 80, 22)
	a.createEdit("6", 100, 416, 50, 25, idDeadzone)
	a.createLabel("%", 155, 420, 25, 22)
	// Shared HOSAS driving response applies to both sticks, so it lives in Driving Response.
	a.controlHOSASGroup = append(a.controlHOSASGroup,
		a.createLabel("Max throttle", 20, 459, 90, 22),
		a.createEdit("100", 100, 455, 50, 25, idThrottleScale),
		a.createLabel("%", 155, 459, 25, 22),
		a.createValueSlider(180, 453, idThrottleScaleSlider, valueSlider{editID: idThrottleScale, min: 10, max: 200, name: "max throttle", normalize: normalizeScalePercent}, 100),
		a.createLabel("Throttle curve", 20, 494, 80, 22),
		a.createSignedEdit("0", 100, 490, 50, 25, idThrottleExpo),
		a.createLabel("%", 155, 494, 25, 22),
		a.createValueSlider(180, 488, idThrottleExpoSlider, valueSlider{editID: idThrottleExpo, min: -100, max: 100, name: "throttle curve", normalize: clampExpoPercent}, 0),
		a.createLabel("Max steering", 20, 529, 90, 22),
		a.createEdit("100", 100, 525, 50, 25, idSteerScale),
		a.createLabel("%", 155, 529, 25, 22),
		a.createValueSlider(180, 523, idSteerScaleSlider, valueSlider{editID: idSteerScale, min: 50, max: 100, name: "max steering", normalize: normalizeSteerScalePercent}, 100),
		a.createLabel("Steer curve 1", 20, 564, 80, 22),
		a.createSignedEdit("0", 100, 560, 50, 25, idSteerExpo),
		a.createLabel("%", 155, 564, 25, 22),
		a.createValueSlider(180, 558, idSteerExpoSlider, valueSlider{editID: idSteerExpo, min: -100, max: 100, name: "steer curve 1", normalize: clampExpoPercent}, 0),
		a.createLabel("Steer curve 2", 20, 599, 80, 22),
		a.createSignedEdit("0", 100, 595, 50, 25, idSteerExpo2),
		a.createLabel("%", 155, 599, 25, 22),
		a.createValueSlider(180, 593, idSteerExpo2Slider, valueSlider{editID: idSteerExpo2, min: -100, max: 100, name: "steer curve 2", normalize: clampExpoPercent}, 0))

	a.activeControlSection = idSectionMonitor
	a.createButton("Initiate Xbox Controller", 20, 460, 180, 32, idStart)
	a.createButton("Test Xbox A", 210, 460, 105, 32, idTestA)
	a.createButton("Camera Left", 325, 460, 105, 32, idTestCamLeft)
	a.createButton("Camera Right", 440, 460, 110, 32, idTestCamRight)
	a.createButton("Save settings", 560, 460, 115, 32, idSave)
	a.createLabel("Polling rate", 700, 467, 85, 22)
	a.createCombo(785, 460, 105, 180, idPollingRate)
	a.createLabel("Hz", 895, 467, 30, 22)
	a.createButton("Show overlay", 935, 460, 150, 32, idShowOverlay)

	a.createLabel("LIVE INPUT MONITOR", 20, 508, 170, 20)
	a.createControl("STATIC", "Waiting for devices...", WS_CHILD|WS_VISIBLE|SS_LEFT, 20, 533, 545, 56, idMonitor)
	a.createControl("STATIC", "Virtual Xbox disconnected. Click Initiate Xbox Controller to create it.", WS_CHILD|WS_VISIBLE|SS_LEFT, 20, 593, 545, 45, idStatus)
	a.createControl("STATIC", "XInput readback: disconnected", WS_CHILD|WS_VISIBLE|SS_LEFT, 20, 643, 545, 28, idXInputReadback)
	hotasDI := a.createControl("STATIC", "HOTAS DirectInput live: waiting for selected devices...", WS_CHILD|WS_VISIBLE|SS_LEFT, 20, 675, 1090, 42, idHotasDIAxisMonitor)
	a.controlHOTASGroup = append(a.controlHOTASGroup, hotasDI)
	a.createControl("STATIC", "Help: hover over a control to see what it does.", WS_CHILD|WS_VISIBLE|WS_BORDER|SS_LEFT, 20, 730, 1090, 32, idHelpText)

	const boxSize = 126
	leftBoxX, boxY := 600, 542
	rightBoxX := 770
	outputBoxX := 940
	a.createLabel("LEFT STICK", leftBoxX+25, 506, 95, 20)
	a.createLabel("RIGHT STICK", rightBoxX+18, 506, 105, 20)
	a.createLabel("DRIVE / STEER", outputBoxX+9, 506, 120, 20)
	for _, x := range []int{leftBoxX, rightBoxX, outputBoxX} {
		a.createControl("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER, x, boxY, boxSize, boxSize, 0)
		a.createControl("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_CLIPSIBLINGS, x+boxSize/2, boxY, 1, boxSize, 0)
		a.createControl("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_CLIPSIBLINGS, x, boxY+boxSize/2, boxSize, 1, 0)
		a.createLabel("F", x+boxSize/2-4, boxY-18, 15, 18)
		a.createLabel("B", x+boxSize/2-4, boxY+boxSize+2, 15, 18)
		a.createLabel("L", x-14, boxY+boxSize/2-8, 14, 18)
		a.createLabel("R", x+boxSize+3, boxY+boxSize/2-8, 14, 18)
	}
	a.createControl("BUTTON", "", WS_CHILD|WS_VISIBLE|BS_OWNERDRAW, leftBoxX+boxSize/2-6, boxY+boxSize/2-6, 12, 12, idLeftDot)
	a.createControl("BUTTON", "", WS_CHILD|WS_VISIBLE|BS_OWNERDRAW, rightBoxX+boxSize/2-6, boxY+boxSize/2-6, 12, 12, idRightDot)
	a.createControl("BUTTON", "", WS_CHILD|WS_VISIBLE|BS_OWNERDRAW, outputBoxX+boxSize/2-6, boxY+boxSize/2-6, 12, 12, idDriveSteerDot)
	a.createControl("STATIC", "Side +0.00  F +0.00", WS_CHILD|WS_VISIBLE|SS_LEFT, leftBoxX, boxY+boxSize+22, 155, 20, idLeftVectorText)
	a.createControl("STATIC", "Side +0.00  F +0.00", WS_CHILD|WS_VISIBLE|SS_LEFT, rightBoxX, boxY+boxSize+22, 155, 20, idRightVectorText)
	a.createControl("STATIC", "Drive +0.00 Neutral\r\nSteer +0.00 Straight", WS_CHILD|WS_VISIBLE|SS_LEFT, outputBoxX-5, boxY+boxSize+20, 185, 38, idDriveSteerVectorText)

	a.activeControlSection = idSectionBindings
	// v1.98.1: preserve two mapping columns, but show full physical assignments.
	// v1.98 had a 148x22 label and 37px rows, causing VKB names to wrap
	// into clipped second lines. Each row now reserves two text lines and
	// extra horizontal label width; Galactic Racer labels may wrap as well.
	const bindingRowHeight = 54
	const bindingTop = 800
	const secondBindingColumnX = 588 // 50px of separation between column groups.
	a.createLabel("STANDARD XBOX MAPPINGS", 20, 770, 800, 22)
	standardCount := len(buttonOutputs) - 5
	drawBinding := func(i, x, y int) {
		bo := buttonOutputs[i]
		a.createControl("BUTTON", bo.Label, WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON|BS_MULTILINE, x, y, 145, 44, idLearnBase+200+i)
		a.createControl("STATIC", "Not assigned", WS_CHILD|WS_VISIBLE|SS_LEFT, x+154, y+3, 230, 44, idLearnBase+100+i)
		a.createButton("Learn", x+394, y+7, 60, 30, idLearnBase+i)
		a.createButton("Clear", x+463, y+7, 55, 30, idLearnClearBase+i)
	}
	for i := 0; i < standardCount; i++ {
		col := i / 8
		row := i % 8
		x := 20
		if col > 0 {
			x = secondBindingColumnX
		}
		drawBinding(i, x, bindingTop+row*bindingRowHeight)
	}
	specificHeadingY := bindingTop + 8*bindingRowHeight + 17
	a.createLabel("GALACTIC RACER SPECIFIC MAPPINGS", 20, specificHeadingY, 800, 23)
	for i := standardCount; i < len(buttonOutputs); i++ {
		n := i - standardCount
		x := 20
		if n/3 > 0 {
			x = secondBindingColumnX
		}
		y := specificHeadingY + 32 + (n%3)*bindingRowHeight
		drawBinding(i, x, y)
	}

	// Reserve separate rows for the stick cards, twist axes, and lower settings.
	// The layout engine uses baseRects to handle scrolling and window resizing.
	for h, r := range a.baseRects {
		if r.Top < 170 {
			continue
		}
		section, ok := a.controlSections[h]
		if !ok {
			panic("scrollable control has no navigation section")
		}
		offset := sectionContentYOffset(section)
		r.Top += int32(offset)
		r.Bottom += int32(offset)
		a.baseRects[h] = r
		procSetWindowPos.Call(h, 0, uintptr(r.Left), uintptr(r.Top), 0, 0, SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE)
	}
	// Centered/Lever ranges are already explained by the Axis Type tooltip.
	a.activeControlSection = idSectionTwist
	a.createTwistUI(true, twistLeftRowY, twistLeftDetailY)
	a.createTwistUI(false, twistRightRowY, twistRightDetailY)
	a.createSectionPanels()
	for h, r := range a.baseRects {
		r.Top += customTitleBarHeight
		r.Bottom += customTitleBarHeight
		a.baseRects[h] = r
		procSetWindowPos.Call(h, 0, uintptr(r.Left), uintptr(r.Top), 0, 0, SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE)
	}
	for _, hwnd := range []uintptr{profileLabel, hosasProfileButton, hotasProfileButton} {
		if bounds, ok := a.baseRects[hwnd]; ok {
			delete(a.baseRects, hwnd)
			delete(a.controlSections, hwnd)
			procSetWindowPos.Call(hwnd, HWND_TOP, uintptr(bounds.Left), uintptr(bounds.Top), uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), SWP_NOACTIVATE)
		}
	}
	for i := range a.cards {
		a.cards[i].bounds.Top += customTitleBarHeight
		a.cards[i].bounds.Bottom += customTitleBarHeight
	}
	fitSectionCardToControls(a.cards, idSectionBindings, a.controlSections, a.baseRects, 16)
	a.activeControlSection = 0
	a.updateContentExtent()

	// Hover help.
	a.tooltipForID(idControlMode, "Podracer Style uses two flight controls as a differential pair: both contribute to thrust and their difference controls steering. Traditional HOTAS uses a dedicated throttle for acceleration/braking behavior and a flight stick for conventional Xbox left-stick steering.")
	a.tooltipForID(idSelectHOSAS, "Select the dual-stick Podracer profile. The stick cards below show the shared max-throttle setting.")
	a.tooltipForID(idSelectHOTAS, "Select the traditional throttle-and-flight-stick profile. HOSAS-only thrust controls are hidden; all HOTAS brake options remain available.")
	a.tooltipForID(idInputBackend, "Axis input backend. Auto uses DirectInput first and falls back to WinMM when possible. DirectInput exposes X/Y/Z, Rx/Ry/Rz and Slider 1/2. WinMM keeps the legacy six-axis path and is isolated in a helper process so problematic legacy joystick drivers cannot take down the main app. Learned buttons/hats use the selected Input API. Existing WinMM bindings remain WinMM; DirectInput bindings are stored separately by device GUID.")
	a.tooltipForID(idHidHideSetup, "Optional double-input protection. If HidHide is installed, this registers this mapper on HidHide's allowlist and opens the official configuration client so you can hide the physical controllers from games. v1.96 does not automatically choose devices to hide.")
	a.tooltipForID(idHotasThrottleDevice, "HOTAS throttle device. Its selected axis is converted to Xbox Right Trigger from 0% to 100% forward thrust only.")
	a.tooltipForID(idHotasThrottleAxis, "Physical throttle axis. Full aft maps to RT 0%; full forward maps to RT 100%. Use Invert if the direction is backwards.")
	a.tooltipForID(idHotasBrakeBehavior, "Choose Traditional HOTAS throttle braking: Off, Digital at Idle, or Analog Brake Zone. Digital applies full LT at the idle threshold. Analog reserves the bottom throttle zone for proportional LT braking and rescales the remaining travel to full RT acceleration.")
	a.tooltipForID(idHotasBrakeThreshold, "Idle Brake Threshold %. Used only by Digital at Idle. At or below this throttle percentage, LT is 100%; above it, brake releases and RT follows the throttle normally.")
	a.tooltipForID(idHotasBrakeZone, "Brake Zone %. Used only by Analog Brake Zone. The bottom part of throttle travel becomes proportional LT braking: minimum = 100% brake, fading to 0% across the zone. Remaining travel is rescaled to full 0-100% RT.")
	a.tooltipForID(idHotasStickDevice, "HOTAS flight stick device. Its selected X/Y axes map directly to the Xbox left thumbstick.")
	a.tooltipForID(idHotasStickXAxis, "Physical flight-stick horizontal axis mapped to Xbox Left Stick X.")
	a.tooltipForID(idHotasStickYAxis, "Physical flight-stick vertical axis mapped to Xbox Left Stick Y.")
	a.tooltipForID(idDetectLeft, "Move the LEFT flight stick forward after clicking. The app identifies the device, forward/back axis, and inversion automatically.")
	a.tooltipForID(idDetectRight, "Move the RIGHT flight stick forward after clicking. The app identifies the device, forward/back axis, and inversion automatically.")
	a.tooltipForID(idLeftThrottleMode, "Treat the selected LEFT HOSAS axis as a throttle lever: 0% = -1, 50% = neutral (0), 100% = +1. Useful for non-spring-centered throttle hardware.")
	a.tooltipForID(idRightThrottleMode, "Treat the selected RIGHT HOSAS axis as a throttle lever: 0% = -1, 50% = neutral (0), 100% = +1. Useful for non-spring-centered throttle hardware.")
	a.tooltipForID(idThrottleScaleSlider, "Max throttle: both sticks are summed. Up to 100 it caps thrust (one stick full + other centered = this value). Above 100 the sum is amplified so full thrust stays while steering with the other stick back (200: left full + right half back = 100%). Full forward + full back = 0.")
	a.tooltipForID(idRefresh, "Rescan Windows for connected controller devices and restart the DirectInput device list.")
	a.tooltipForID(idDetectWASD, "DirectInput thumbstick detection: push the small stick RIGHT, return to center, then push it FORWARD. This reads the same X/Y Rotation path that joy.cpl uses.")
	a.tooltipForID(idOnFootMode, "Choose how on-foot W/A/S/D is produced: analog thumbstick, learned stick buttons/hats, or off.")
	a.tooltipForID(idStart, "Creates the virtual Xbox controller and begins sending configured controller/keyboard outputs to the game. Deactivate Xbox Controller removes the virtual controller.")
	a.tooltipForID(idTestA, "Send a short Xbox A-button pulse through the virtual controller. Initiate Xbox Controller must be active.")
	a.tooltipForID(idTestCamLeft, "Force virtual Xbox Right Stick X fully left briefly. Useful for camera-output diagnostics.")
	a.tooltipForID(idTestCamRight, "Force virtual Xbox Right Stick X fully right briefly. Useful for camera-output diagnostics.")
	a.tooltipForID(idShowOverlay, "Minimize this window and show Left Stick, Right Stick and Drive/Steer as an always-on-top overlay at the top right of the monitor. Close it with X to return. Works over borderless/windowed games, not exclusive fullscreen.")
	a.tooltipForID(idSave, "Save the current device, axis, deadzone, polling-rate, twist, pedal, on-foot, and learned-button settings.")
	a.tooltipForID(idThrottleScale, "Max throttle %. Drive = (left + right) x this value, capped at 100% output. Example 80: left full forward + right centered = 80%. Example 150: left full + right 1/3 back = 100%. Range 10-200, default 100.")
	a.tooltipForID(idThrottleExpo, "Throttle curve (expo) %. 0 = linear; positive = finer control at low throttle, negative = more aggressive response at low throttle. Full stick sum still reaches max throttle; reverse is symmetric. Range -100 to 100, default 0.")
	a.tooltipForID(idThrottleExpoSlider, "Throttle curve: -100 = most aggressive around zero, 0 = linear, 100 = strongest softening around zero throttle. Full stick sum always reaches max throttle.")
	a.tooltipForID(idSteerScale, "Max steering = steering with one stick full and the other centered. Counter-steering with the second stick adds the rest up to full lock (costs speed). 100 = one stick gives full lock; 50 = full lock needs full counter-steer. Range 50-100, default 100.")
	a.tooltipForID(idSteerScaleSlider, "Max steering 50-100: how much counter-steer you need for full lock. 100 = one stick is enough, 50 = full counter-steer needed.")
	a.tooltipForID(idSteerExpo, "Steer curve 1 (expo) %: one-stick segment from center to max steering. 0 = linear; positive = soft start, negative = aggressive start. Range -100 to 100, default 0.")
	a.tooltipForID(idSteerExpoSlider, "Steer curve 1: -100 = aggressive, 0 = linear, 100 = soft start. Ends exactly at max steering.")
	a.tooltipForID(idSteerExpo2, "Steer curve 2 (expo) %: counter-stick segment from max steering to 100%. 0 = linear; positive = counter-stick bites late, negative = counter-stick adds steering early. Range -100 to 100, default 0.")
	a.tooltipForID(idSteerExpo2Slider, "Steer curve 2: -100 = aggressive, 0 = linear, 100 = soft. Full counter-steer always reaches 100%.")
	a.tooltipForID(idPollingRate, "How often the controller loop requests fresh input/output updates. Higher rates reduce interval but use more CPU; hardware/Windows update rates may still be lower.")
	for _, left := range []bool{true, false} {
		ids := auxiliaryIDs(left)
		name := "Right Twist Axis"
		if left {
			name = "Left Twist Axis"
		}
		a.tooltipForID(ids[0], name+": choose a physical joystick/twist/lever device, or None to disable it.")
		a.tooltipForID(ids[1], "Select the physical analog axis (X/Y/Z/Rx/Ry/Rz/Slider) for this twist input.")
		a.tooltipForID(ids[2], "Invert axis direction. Centered reverses sign; Lever reverses 0-100% travel.")
		a.tooltipForID(ids[3], "Centered: -100..+100 around midpoint. Lever: 0..100 travel; enable Output Negative for 0..-100. Invert reverses travel independently.")
		a.tooltipForID(ids[4], "Independent twist deadzone percentage. For Lever, deadzone applies near 0% travel.")
		a.tooltipForID(ids[5], "Same modes as rudder pedals: Off, Fine steering, Move Camera, Pitch, Keyboard keys. Camera and Pitch are analog Xbox outputs; Keyboard keys emits native key presses.")
		a.tooltipForID(ids[9], "Keyboard key held when this twist input moves negative past threshold. Lever axes can use this with Output Negative enabled.")
		a.tooltipForID(ids[10], "Keyboard key held when this twist input moves positive past threshold.")
		a.tooltipForID(ids[12], "Lever mode only: output 0 to -100% instead of 0 to +100%. Invert still independently reverses lever travel. Does not change Centered behavior.")
	}
	a.tooltipForID(idPedalMode, "Choose how the rudder axis is used. Pitch maps the pedal axis continuously to Xbox Left Stick Y; positive pedal input is Pitch Up and Pedal Invert reverses the direction. Only settings relevant to the selected mode are shown.")
	a.tooltipForID(idPedalLeftKey, "Keyboard key held when the pedal axis moves left past the configured threshold. Includes Shift, left/right Control, Alt, Tab and other common keys.")
	a.tooltipForID(idPedalRightKey, "Keyboard key held when the pedal axis moves right past the configured threshold. Includes Shift, left/right Control, Alt, Tab and other common keys.")
	for i, bo := range buttonOutputs {
		a.tooltipForID(idLearnBase+i, "Click Learn, then press the controller button/hat/switch you want assigned to "+bo.Label+".")
		a.tooltipForID(idLearnBase+200+i, "Live indicator for "+bo.Label+". It highlights when the learned physical input is pressed while the visual monitor is active.")
		a.tooltipForID(idLearnClearBase+i, "Clear the saved "+bo.Label+" assignment and return it to Not assigned.")
		if bo.Key == "LT" {
			msg := "Optional manual Left Trigger mapping. Your analog controls already generate LT automatically when the selected control mode/brake behavior calls for braking or reverse. You do not need to assign this for normal braking; use it only if you also want a button/hat to send full LT on demand."
			a.tooltipForID(idLearnBase+i, msg)
			a.tooltipForID(idLearnBase+200+i, msg)
		}
		if bo.Key == "RT" {
			msg := "Optional manual Right Trigger mapping. Acceleration is already generated automatically by the analog Podracer Style or Traditional HOTAS controls. You do not need to assign this for normal thrust; use it only if you also want a button/hat to send full RT on demand."
			a.tooltipForID(idLearnBase+i, msg)
			a.tooltipForID(idLearnBase+200+i, msg)
		}
		if bo.Key == "PITCHUP" || bo.Key == "PITCHDOWN" {
			direction := "UP"
			if bo.Key == "PITCHDOWN" {
				direction = "DOWN"
			}
			msg := "Digital pitch mapping. While this learned button/hat is held, Xbox Left Stick Y is sent fully " + direction + ". Use Pedal mode Pitch instead when you want proportional analog pitch."
			a.tooltipForID(idLearnBase+i, msg)
			a.tooltipForID(idLearnBase+200+i, msg)
		}
	}
	for i, def := range wasdButtonDefs {
		a.tooltipForID(idWASDLearnBase+i, "Click Learn, then press the controller button/hat/switch that should generate "+def.Label+" for on-foot movement.")
		a.tooltipForID(idWASDClearBase+i, "Clear the saved "+def.Label+" on-foot assignment and return it to Not assigned.")
		a.tooltipForID(idWASDIndicatorBase+i, "Live indicator for the learned "+def.Label+" movement input.")
	}

	a.populateCombos()
	a.applyConfigToUI()
	a.updateBindingLabels()
	a.resizeCaptionButtons()
	a.resizeSidebar()
	a.uiReady = true
	a.updateContextVisibility()
	a.liveCardLast = a.overlaySnapshotNow()
	a.navigateToSection(idSectionSetup)
	procSetTimer.Call(a.hwnd, 1, 100, 0)
	if t, _, e := procSetTimer.Call(a.hwnd, liveCardTimerID, overlayRefreshMs, 0); t == 0 {
		a.postStatus(fmt.Sprintf("Live output unavailable: refresh timer failed: %v", e))
	}
}

func (a *appState) populateCombos() {
	a.cfgMu.RLock()
	backend := normalizeInputBackend(a.cfg.InputBackend)
	a.cfgMu.RUnlock()

	deviceIDs := []int{idLeftDevice, idRightDevice, idPedalDevice, idHotasThrottleDevice, idHotasStickDevice, idTwistLeftDevice, idTwistRightDevice}
	if backend == "WinMM" {
		a.devicesMu.RLock()
		devs := append([]deviceInfo(nil), a.devices...)
		a.devicesMu.RUnlock()
		for _, id := range deviceIDs {
			a.comboReset(id)
			ids := []int{}
			guids := []string{}
			if id == idPedalDevice || id == idHotasThrottleDevice || id == idHotasStickDevice || id == idTwistLeftDevice || id == idTwistRightDevice {
				a.comboAdd(id, "None")
				ids = append(ids, -1)
				guids = append(guids, "")
			}
			for _, d := range devs {
				a.comboAdd(id, fmt.Sprintf("%s [WinMM ID %d]", d.Name, d.ID))
				ids = append(ids, d.ID)
				guids = append(guids, "")
			}
			a.deviceComboIDs[id] = ids
			a.deviceComboGUIDs[id] = guids
		}
	} else {
		devs := a.di.DeviceList()
		for _, id := range deviceIDs {
			a.comboReset(id)
			ids := []int{-1}
			guids := []string{""}
			a.comboAdd(id, "None")
			for _, d := range devs {
				a.comboAdd(id, fmt.Sprintf("%s [DirectInput]", d.Name))
				ids = append(ids, a.matchWinMMIDByName(d.Name))
				guids = append(guids, d.GUID)
			}
			a.deviceComboIDs[id] = ids
			a.deviceComboGUIDs[id] = guids
		}
	}

	axisList := a.axisNames
	if backend != "WinMM" {
		axisList = diAxisNames
	}
	for _, id := range []int{idLeftAxis, idRightAxis, idPedalAxis, idHotasThrottleAxis, idHotasStickXAxis, idHotasStickYAxis, idTwistLeftAxis, idTwistRightAxis} {
		a.comboReset(id)
		for _, name := range axisList {
			a.comboAdd(id, name)
		}
	}

	a.comboReset(idInputBackend)
	for _, mode := range []string{"Auto (DirectInput first)", "DirectInput", "WinMM (compatibility)"} {
		a.comboAdd(idInputBackend, mode)
	}

	a.comboReset(idControlMode)
	for _, mode := range []string{"Podracer Style", "Traditional HOTAS"} {
		a.comboAdd(idControlMode, mode)
	}

	a.comboReset(idHotasBrakeBehavior)
	for _, mode := range []string{"Off", "Digital at Idle", "Analog Brake Zone"} {
		a.comboAdd(idHotasBrakeBehavior, mode)
	}

	a.comboReset(idPedalMode)
	for _, mode := range []string{"Off", "Fine steering", "Move Camera", "Pitch", "Keyboard keys"} {
		a.comboAdd(idPedalMode, mode)
	}

	for _, id := range []int{idTwistLeftAxisType, idTwistRightAxisType} {
		a.comboReset(id)
		for _, name := range []string{"Centered", "Lever"} {
			a.comboAdd(id, name)
		}
	}
	for _, id := range []int{idTwistLeftMode, idTwistRightMode} {
		a.comboReset(id)
		for _, mode := range []string{"Off", "Fine steering", "Move Camera", "Pitch", "Keyboard keys"} {
			a.comboAdd(id, mode)
		}
	}
	a.comboReset(idOnFootMode)
	for _, mode := range []string{"Off", "Thumbstick -> WASD", "Buttons -> WASD"} {
		a.comboAdd(idOnFootMode, mode)
	}

	a.comboReset(idPollingRate)
	for _, hz := range pollingRateOptions {
		a.comboAdd(idPollingRate, fmt.Sprintf("%d", hz))
	}
	for _, id := range []int{idPedalLeftKey, idPedalRightKey, idTwistLeftLeftKey, idTwistLeftRightKey, idTwistRightLeftKey, idTwistRightRightKey} {
		a.comboReset(id)
		for _, k := range keyboardKeyOptions {
			a.comboAdd(id, k.Name)
		}
	}
}

func (a *appState) applyConfigToUI() {
	a.applyingUI = true
	defer func() { a.applyingUI = false }()
	a.cfgMu.RLock()
	c := a.cfg
	a.cfgMu.RUnlock()
	backendIdx := 0
	switch normalizeInputBackend(c.InputBackend) {
	case "DirectInput":
		backendIdx = 1
	case "WinMM":
		backendIdx = 2
	}
	comboSetIndex(idInputBackend, backendIdx)
	controlIdx := 0
	if strings.EqualFold(normalizeControlMode(c.ControlMode), "HOTAS") {
		controlIdx = 1
	}
	comboSetIndex(idControlMode, controlIdx)
	if normalizeInputBackend(c.InputBackend) == "WinMM" {
		comboSetDevice(idHotasThrottleDevice, c.HotasThrottleID)
		comboSetDevice(idHotasStickDevice, c.HotasStickID)
		comboSetDevice(idLeftDevice, c.LeftID)
		comboSetDevice(idRightDevice, c.RightID)
		comboSetDevice(idPedalDevice, c.PedalID)
		comboSetDevice(idTwistLeftDevice, c.TwistLeft.DeviceID)
		comboSetDevice(idTwistRightDevice, c.TwistRight.DeviceID)
	} else {
		comboSetGUID(idHotasThrottleDevice, c.HotasThrottleDIGUID)
		comboSetGUID(idHotasStickDevice, c.HotasStickDIGUID)
		comboSetGUID(idLeftDevice, c.LeftDIGUID)
		comboSetGUID(idRightDevice, c.RightDIGUID)
		comboSetGUID(idPedalDevice, c.PedalDIGUID)
		comboSetGUID(idTwistLeftDevice, c.TwistLeft.DIGUID)
		comboSetGUID(idTwistRightDevice, c.TwistRight.DIGUID)
	}
	comboSetIndex(idHotasThrottleAxis, c.HotasThrottleAxis)
	setChecked(a.controls[idHotasThrottleInvert], c.HotasThrottleInvert)
	brakeModeIdx := 1
	switch normalizeHotasBrakeBehavior(c.HotasBrakeBehavior) {
	case "Off":
		brakeModeIdx = 0
	case "Analog Brake Zone":
		brakeModeIdx = 2
	}
	comboSetIndex(idHotasBrakeBehavior, brakeModeIdx)
	setText(a.controls[idHotasBrakeThreshold], strconv.Itoa(c.HotasBrakeThreshold))
	setText(a.controls[idHotasBrakeZone], strconv.Itoa(c.HotasBrakeZone))
	comboSetIndex(idHotasStickXAxis, c.HotasStickXAxis)
	comboSetIndex(idHotasStickYAxis, c.HotasStickYAxis)
	setChecked(a.controls[idHotasStickInvertX], c.HotasStickInvertX)
	setChecked(a.controls[idHotasStickInvertY], c.HotasStickInvertY)
	comboSetIndex(idLeftAxis, c.LeftAxis)
	comboSetIndex(idRightAxis, c.RightAxis)
	comboSetIndex(idPedalAxis, c.PedalAxis)
	a.applyAuxiliaryAxisToUI(c.TwistLeft, true)
	a.applyAuxiliaryAxisToUI(c.TwistRight, false)
	setChecked(a.controls[idLeftInvert], c.LeftInvert)
	setChecked(a.controls[idRightInvert], c.RightInvert)
	setChecked(a.controls[idLeftThrottleMode], c.LeftThrottleMode)
	setChecked(a.controls[idRightThrottleMode], c.RightThrottleMode)
	setChecked(a.controls[idPedalInvert], c.PedalInvert)
	setChecked(a.controls[idWASDInvertSide], c.ThumbWASDInvertSide)
	setChecked(a.controls[idWASDInvertForward], c.ThumbWASDInvertForward)
	modeIdx := 0
	if strings.EqualFold(c.PedalMode, "Fine steering") {
		modeIdx = 1
	} else if strings.EqualFold(c.PedalMode, "Move Camera") {
		modeIdx = 2
	} else if strings.EqualFold(c.PedalMode, "Pitch") {
		modeIdx = 3
	} else if strings.EqualFold(c.PedalMode, "Keyboard keys") {
		modeIdx = 4
	}
	comboSetIndex(idPedalMode, modeIdx)
	onFootIdx := 0
	switch normalizeOnFootMode(c.OnFootMode) {
	case "Thumbstick -> WASD":
		onFootIdx = 1
	case "Buttons -> WASD":
		onFootIdx = 2
	}
	comboSetIndex(idOnFootMode, onFootIdx)
	comboSetIndex(idPedalLeftKey, keyOptionIndex(c.PedalLeftKey))
	comboSetIndex(idPedalRightKey, keyOptionIndex(c.PedalRightKey))
	setText(a.controls[idDeadzone], strconv.Itoa(c.Deadzone))
	throttleScale := normalizeScalePercent(c.ThrottleScale)
	setText(a.controls[idThrottleScale], strconv.Itoa(throttleScale))
	a.setValueSlider(idThrottleScale, throttleScale)
	throttleExpo := normalizeExpoPercent(c.ThrottleExpo)
	setText(a.controls[idThrottleExpo], strconv.Itoa(throttleExpo))
	a.setValueSlider(idThrottleExpo, throttleExpo)
	steerScale := normalizeSteerScalePercent(c.SteerScale)
	setText(a.controls[idSteerScale], strconv.Itoa(steerScale))
	a.setValueSlider(idSteerScale, steerScale)
	steerExpo := normalizeExpoPercent(c.SteerExpo)
	setText(a.controls[idSteerExpo], strconv.Itoa(steerExpo))
	a.setValueSlider(idSteerExpo, steerExpo)
	steerExpo2 := normalizeExpoPercent(c.SteerExpo2)
	setText(a.controls[idSteerExpo2], strconv.Itoa(steerExpo2))
	a.setValueSlider(idSteerExpo2, steerExpo2)
	setText(a.controls[idPedalStrength], strconv.Itoa(c.PedalSteerStrength))
	setText(a.controls[idCameraSensitivity], strconv.Itoa(c.CameraSensitivity))
	setText(a.controls[idCameraDeadzone], strconv.Itoa(c.CameraDeadzone))
	setText(a.controls[idPedalKeyThreshold], strconv.Itoa(c.PedalKeyThreshold))
	setText(a.controls[idWASDThreshold], strconv.Itoa(c.ThumbWASDThreshold))
	pidx := 0
	for i, hz := range pollingRateOptions {
		if hz == c.PollingHz {
			pidx = i
			break
		}
	}
	comboSetIndex(idPollingRate, pidx)
	if a.uiReady {
		a.updateContextVisibility()
	}
}

func auxiliaryIDs(left bool) []int {
	if left {
		return []int{idTwistLeftDevice, idTwistLeftAxis, idTwistLeftInvert, idTwistLeftAxisType, idTwistLeftDeadzone, idTwistLeftMode, idTwistLeftStrength, idTwistLeftCameraSensitivity, idTwistLeftCameraDeadzone, idTwistLeftLeftKey, idTwistLeftRightKey, idTwistLeftKeyThreshold, idTwistLeftOutputNegative}
	}
	return []int{idTwistRightDevice, idTwistRightAxis, idTwistRightInvert, idTwistRightAxisType, idTwistRightDeadzone, idTwistRightMode, idTwistRightStrength, idTwistRightCameraSensitivity, idTwistRightCameraDeadzone, idTwistRightLeftKey, idTwistRightRightKey, idTwistRightKeyThreshold, idTwistRightOutputNegative}
}

func (a *appState) applyAuxiliaryAxisToUI(v auxiliaryAxisConfig, left bool) {
	ids := auxiliaryIDs(left)
	comboSetIndex(ids[1], v.Axis)
	setChecked(a.controls[ids[2]], v.Invert)
	setChecked(a.controls[ids[12]], v.OutputNegative)
	if normalizeAxisType(v.AxisType) == "Lever" {
		comboSetIndex(ids[3], 1)
	} else {
		comboSetIndex(ids[3], 0)
	}
	setText(a.controls[ids[4]], strconv.Itoa(v.Deadzone))
	modes := []string{"Off", "Fine steering", "Move Camera", "Pitch", "Keyboard keys"}
	for i, mode := range modes {
		if strings.EqualFold(mode, v.Mode) {
			comboSetIndex(ids[5], i)
			break
		}
	}
	setText(a.controls[ids[6]], strconv.Itoa(v.SteerStrength))
	setText(a.controls[ids[7]], strconv.Itoa(v.CameraSensitivity))
	setText(a.controls[ids[8]], strconv.Itoa(v.CameraDeadzone))
	comboSetIndex(ids[9], keyOptionIndex(v.LeftKey))
	comboSetIndex(ids[10], keyOptionIndex(v.RightKey))
	setText(a.controls[ids[11]], strconv.Itoa(v.KeyThreshold))
}

func readLimitedUIInt(h uintptr, fallback, minValue, maxValue int) int {
	v, err := strconv.Atoi(strings.TrimSpace(getText(h)))
	if err != nil {
		return fallback
	}
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func (a *appState) readAuxiliaryAxisFromUI(v auxiliaryAxisConfig, left bool) auxiliaryAxisConfig {
	ids := auxiliaryIDs(left)
	v.DeviceID = comboDevice(ids[0])
	v.DIGUID = comboGUID(ids[0])
	v.Axis = comboIndex(ids[1])
	v.Invert = isChecked(a.controls[ids[2]])
	v.OutputNegative = isChecked(a.controls[ids[12]])
	v.AxisType = []string{"Centered", "Lever"}[min(comboIndex(ids[3]), 1)]
	v.Deadzone = readLimitedUIInt(a.controls[ids[4]], v.Deadzone, 0, 40)
	modes := []string{"Off", "Fine steering", "Move Camera", "Pitch", "Keyboard keys"}
	v.Mode = modes[min(comboIndex(ids[5]), len(modes)-1)]
	v.SteerStrength = readLimitedUIInt(a.controls[ids[6]], v.SteerStrength, 0, 100)
	v.CameraSensitivity = readLimitedUIInt(a.controls[ids[7]], v.CameraSensitivity, 10, 250)
	v.CameraDeadzone = readLimitedUIInt(a.controls[ids[8]], v.CameraDeadzone, 0, 40)
	v.LeftKey = selectedKeyVK(ids[9])
	v.RightKey = selectedKeyVK(ids[10])
	v.KeyThreshold = readLimitedUIInt(a.controls[ids[11]], v.KeyThreshold, 5, 95)
	return v
}

func comboSetIndex(id, idx int) {
	opts := app.comboOptions[id]
	if len(opts) == 0 {
		app.comboSelected[id] = 0
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(opts) {
		idx = len(opts) - 1
	}
	app.comboSelected[id] = idx
	setText(app.controls[id], opts[idx]+"  v")
}
func comboSetDevice(id, deviceID int) {
	ids := app.deviceComboIDs[id]
	idx := 0
	for i, v := range ids {
		if v == deviceID {
			idx = i
			break
		}
	}
	comboSetIndex(id, idx)
}
func comboSetGUID(id int, guid string) {
	guids := app.deviceComboGUIDs[id]
	idx := 0
	for i, v := range guids {
		if strings.EqualFold(v, guid) {
			idx = i
			break
		}
	}
	comboSetIndex(id, idx)
}

func comboGUID(id int) string {
	idx := comboIndex(id)
	guids := app.deviceComboGUIDs[id]
	if idx >= 0 && idx < len(guids) {
		return guids[idx]
	}
	return ""
}

func setChecked(h uintptr, v bool) {
	n := uintptr(0)
	if v {
		n = BST_CHECKED
	}
	procSendMessageW.Call(h, BM_SETCHECK, n, 0)
}
func isChecked(h uintptr) bool {
	r, _, _ := procSendMessageW.Call(h, BM_GETCHECK, 0, 0)
	return r == BST_CHECKED
}
func setText(h uintptr, s string) { procSetWindowTextW.Call(h, uintptr(unsafe.Pointer(utf16ptr(s)))) }
func getText(h uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := procGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:int(n)])
}
func comboIndex(id int) int {
	idx, ok := app.comboSelected[id]
	if !ok || idx < 0 {
		return 0
	}
	return idx
}
func comboDevice(id int) int {
	idx := comboIndex(id)
	ids := app.deviceComboIDs[id]
	if idx >= 0 && idx < len(ids) {
		return ids[idx]
	}
	return -1
}

func (a *appState) syncConfigFromUI() {
	if !a.uiReady {
		return
	}
	a.cfgMu.Lock()
	c := a.cfg
	backendModes := []string{"Auto", "DirectInput", "WinMM"}
	bi := comboIndex(idInputBackend)
	if bi < 0 || bi >= len(backendModes) {
		bi = 0
	}
	c.InputBackend = backendModes[bi]
	controlModes := []string{"HOSAS", "HOTAS"}
	ci := comboIndex(idControlMode)
	if ci < 0 || ci >= len(controlModes) {
		ci = 0
	}
	c.ControlMode = controlModes[ci]
	if normalizeInputBackend(c.InputBackend) == "WinMM" {
		c.HotasThrottleID = comboDevice(idHotasThrottleDevice)
		c.HotasStickID = comboDevice(idHotasStickDevice)
		c.LeftID = comboDevice(idLeftDevice)
		c.RightID = comboDevice(idRightDevice)
		c.PedalID = comboDevice(idPedalDevice)
	} else {
		c.HotasThrottleDIGUID = comboGUID(idHotasThrottleDevice)
		c.HotasStickDIGUID = comboGUID(idHotasStickDevice)
		c.LeftDIGUID = comboGUID(idLeftDevice)
		c.RightDIGUID = comboGUID(idRightDevice)
		c.PedalDIGUID = comboGUID(idPedalDevice)
		// Keep a matching WinMM ID when names line up so Auto has a safe legacy fallback.
		c.HotasThrottleID = comboDevice(idHotasThrottleDevice)
		c.HotasStickID = comboDevice(idHotasStickDevice)
		c.LeftID = comboDevice(idLeftDevice)
		c.RightID = comboDevice(idRightDevice)
		c.PedalID = comboDevice(idPedalDevice)
	}
	c.HotasThrottleAxis = comboIndex(idHotasThrottleAxis)
	c.HotasThrottleInvert = isChecked(a.controls[idHotasThrottleInvert])
	brakeBehaviors := []string{"Off", "Digital at Idle", "Analog Brake Zone"}
	bbi := comboIndex(idHotasBrakeBehavior)
	if bbi < 0 || bbi >= len(brakeBehaviors) {
		bbi = 1
	}
	c.HotasBrakeBehavior = brakeBehaviors[bbi]
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idHotasBrakeThreshold]))); e == nil {
		if n < 0 {
			n = 0
		}
		if n > 50 {
			n = 50
		}
		c.HotasBrakeThreshold = n
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idHotasBrakeZone]))); e == nil {
		if n < 1 {
			n = 1
		}
		if n > 50 {
			n = 50
		}
		c.HotasBrakeZone = n
	}
	c.HotasStickXAxis = comboIndex(idHotasStickXAxis)
	c.HotasStickYAxis = comboIndex(idHotasStickYAxis)
	c.HotasStickInvertX = isChecked(a.controls[idHotasStickInvertX])
	c.HotasStickInvertY = isChecked(a.controls[idHotasStickInvertY])
	c.LeftAxis = comboIndex(idLeftAxis)
	c.RightAxis = comboIndex(idRightAxis)
	c.PedalAxis = comboIndex(idPedalAxis)
	c.TwistLeft = a.readAuxiliaryAxisFromUI(c.TwistLeft, true)
	c.TwistRight = a.readAuxiliaryAxisFromUI(c.TwistRight, false)
	pi := comboIndex(idPollingRate)
	if pi >= 0 && pi < len(pollingRateOptions) {
		c.PollingHz = pollingRateOptions[pi]
	}
	c.LeftInvert = isChecked(a.controls[idLeftInvert])
	c.RightInvert = isChecked(a.controls[idRightInvert])
	c.LeftThrottleMode = isChecked(a.controls[idLeftThrottleMode])
	c.RightThrottleMode = isChecked(a.controls[idRightThrottleMode])
	c.PedalInvert = isChecked(a.controls[idPedalInvert])
	c.ThumbWASDInvertSide = isChecked(a.controls[idWASDInvertSide])
	c.ThumbWASDInvertForward = isChecked(a.controls[idWASDInvertForward])
	modes := []string{"Off", "Fine steering", "Move Camera", "Pitch", "Keyboard keys"}
	mi := comboIndex(idPedalMode)
	if mi < 0 || mi >= len(modes) {
		mi = 0
	}
	c.PedalMode = modes[mi]
	onFootModes := []string{"Off", "Thumbstick -> WASD", "Buttons -> WASD"}
	oi := comboIndex(idOnFootMode)
	if oi < 0 || oi >= len(onFootModes) {
		oi = 0
	}
	c.OnFootMode = onFootModes[oi]
	// Maintain the old boolean field so older builds can still interpret a thumbstick-enabled config.
	c.ThumbWASDEnabled = strings.EqualFold(c.OnFootMode, "Thumbstick -> WASD")
	c.PedalLeftKey = selectedKeyVK(idPedalLeftKey)
	c.PedalRightKey = selectedKeyVK(idPedalRightKey)
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idWASDThreshold]))); e == nil {
		if n < 5 {
			n = 5
		}
		if n > 95 {
			n = 95
		}
		c.ThumbWASDThreshold = n
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idPedalKeyThreshold]))); e == nil {
		if n < 5 {
			n = 5
		}
		if n > 95 {
			n = 95
		}
		c.PedalKeyThreshold = n
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idThrottleScale]))); e == nil {
		c.ThrottleScale = normalizeScalePercent(n)
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idThrottleExpo]))); e == nil {
		c.ThrottleExpo = clampExpoPercent(n)
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idSteerScale]))); e == nil {
		c.SteerScale = normalizeSteerScalePercent(n)
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idSteerExpo]))); e == nil {
		c.SteerExpo = clampExpoPercent(n)
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idSteerExpo2]))); e == nil {
		c.SteerExpo2 = clampExpoPercent(n)
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idDeadzone]))); e == nil {
		if n < 0 {
			n = 0
		}
		if n > 40 {
			n = 40
		}
		c.Deadzone = n
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idPedalStrength]))); e == nil {
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		c.PedalSteerStrength = n
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idCameraSensitivity]))); e == nil {
		if n < 10 {
			n = 10
		}
		if n > 250 {
			n = 250
		}
		c.CameraSensitivity = n
	}
	if n, e := strconv.Atoi(strings.TrimSpace(getText(a.controls[idCameraDeadzone]))); e == nil {
		if n < 0 {
			n = 0
		}
		if n > 40 {
			n = 40
		}
		c.CameraDeadzone = n
	}
	a.cfg = c
	a.cfgMu.Unlock()
}

func (a *appState) handleValueEdit(spec valueSlider, notify int) {
	if notify == EN_CHANGE {
		if a.sliderChanging {
			return
		}
		n, err := strconv.Atoi(strings.TrimSpace(getText(a.controls[spec.editID])))
		if err == nil && n >= spec.min && n <= spec.max {
			procSendMessageW.Call(spec.hwnd, TBM_SETPOS, 1, uintptr(n))
			procRedrawWindow.Call(a.hwnd, 0, 0, RDW_INVALIDATE)
			if a.uiReady {
				a.syncConfigFromUI()
				saveConfig()
			}
		}
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(getText(a.controls[spec.editID])))
	if err != nil {
		position, _, _ := procSendMessageW.Call(spec.hwnd, TBM_GETPOS, 0, 0)
		n = int(int32(position))
		a.postStatus(fmt.Sprintf("Invalid %s; restored the last valid value of %d%%.", spec.name, n))
	}
	normalized := spec.normalize(n)
	if n != normalized {
		a.postStatus(fmt.Sprintf("%s adjusted to %d%% (supported range: %d-%d%%).", strings.ToUpper(spec.name[:1])+spec.name[1:], normalized, spec.min, spec.max))
	}
	a.sliderChanging = true
	setText(a.controls[spec.editID], strconv.Itoa(normalized))
	a.sliderChanging = false
	procSendMessageW.Call(spec.hwnd, TBM_SETPOS, 1, uintptr(normalized))
	if a.uiReady {
		a.syncConfigFromUI()
		saveConfig()
	}
	procRedrawWindow.Call(a.hwnd, 0, 0, RDW_INVALIDATE)
}

func (a *appState) onCommand(id, notify int) {
	if a.applyingUI && notify == EN_CHANGE {
		return
	}
	if spec, ok := a.valueSliderForEdit(id); ok && (notify == EN_CHANGE || notify == EN_KILLFOCUS) {
		a.handleValueEdit(spec, notify)
		return
	}
	if notify == BN_CLICKED {
		if state := a.controls[id]; state != 0 && a.checkboxVisible[id] != 0 {
			next := uintptr(0)
			if !isChecked(state) {
				next = BST_CHECKED
			}
			procSendMessageW.Call(state, BM_SETCHECK, next, 0)
			procRedrawWindow.Call(a.checkboxVisible[id], 0, 0, RDW_INVALIDATE|RDW_UPDATENOW)
		}
	}
	if notify == 0 || notify == BN_CLICKED {
		switch id {
		case idWindowMinimize:
			procShowWindow.Call(a.hwnd, SW_MINIMIZE)
			return
		case idWindowMaximize:
			if maximized, _, _ := procIsZoomed.Call(a.hwnd); maximized != 0 {
				procShowWindow.Call(a.hwnd, SW_RESTORE)
			} else {
				procShowWindow.Call(a.hwnd, SW_MAXIMIZE)
			}
			return
		case idWindowClose:
			procSendMessageW.Call(a.hwnd, WM_CLOSE, 0, 0)
			return
		case idSectionSetup:
			a.navigateToSection(idSectionSetup)
			return
		case idSectionTwist:
			a.navigateToSection(idSectionTwist)
			return
		case idSectionMonitor:
			a.navigateToSection(idSectionMonitor)
			return
		case idSectionBindings:
			a.navigateToSection(idSectionBindings)
			return
		case idSectionFoot:
			a.navigateToSection(idSectionFoot)
			return
		case idSectionDevices:
			a.navigateToSection(idSectionDevices)
			return
		case idSelectHOSAS:
			comboSetIndex(idControlMode, 0)
			a.updateContextVisibility()
			a.syncConfigFromUI()
			saveConfig()
			a.redrawAfterScroll()
			return
		case idSelectHOTAS:
			comboSetIndex(idControlMode, 1)
			a.updateContextVisibility()
			a.syncConfigFromUI()
			saveConfig()
			a.redrawAfterScroll()
			return
		}
	}
	if id == idComboList && (notify == LBN_SELCHANGE || notify == LBN_DBLCLK) {
		a.closeCombo(true)
		return
	}
	if id == idComboList && notify == LBN_KILLFOCUS {
		a.closeCombo(false)
		return
	}
	if a.isComboID(id) && notify == BN_CLICKED {
		a.toggleCombo(id)
		return
	}
	if a.comboActiveID != 0 {
		a.closeCombo(false)
	}
	switch {
	case id == idStart && notify == BN_CLICKED:
		if a.mappingStarting.Load() {
			a.postStatus("Virtual Xbox controller is still starting...")
			return
		}
		if a.mappingActive.Load() || a.xboxReady.Load() {
			a.stopMapping()
			return
		}
		a.syncConfigFromUI()
		saveConfig()
		a.mappingStarting.Store(true)
		setText(a.controls[idStart], "Initiating...")
		a.postStatus("Initiating virtual Xbox controller...")
		go a.startMappingAsync()
		return
	case id == idTestA && notify == BN_CLICKED:
		if !a.xboxReady.Load() {
			a.postStatus("Initiate Xbox Controller first; the virtual Xbox controller is intentionally disconnected while output is inactive.")
			return
		}
		a.testUntilMu.Lock()
		a.testUntil = time.Now().Add(350 * time.Millisecond)
		a.testUntilMu.Unlock()
		a.postStatus("Sent Xbox A test pulse.")
		return
	case id == idTestCamLeft && notify == BN_CLICKED:
		if !a.xboxReady.Load() {
			a.postStatus("Initiate Xbox Controller first; the virtual Xbox controller is intentionally disconnected while output is inactive.")
			return
		}
		a.cameraTestMu.Lock()
		a.cameraTestValue = -1.0
		a.cameraTestUntil = time.Now().Add(800 * time.Millisecond)
		a.cameraTestMu.Unlock()
		a.postStatus("Camera test: virtual Xbox Right Stick X = full LEFT for 0.8 seconds.")
		return
	case id == idShowOverlay && notify == BN_CLICKED:
		a.showOverlay()
		return
	case id == idTestCamRight && notify == BN_CLICKED:
		if !a.xboxReady.Load() {
			a.postStatus("Initiate Xbox Controller first; the virtual Xbox controller is intentionally disconnected while output is inactive.")
			return
		}
		a.cameraTestMu.Lock()
		a.cameraTestValue = 1.0
		a.cameraTestUntil = time.Now().Add(800 * time.Millisecond)
		a.cameraTestMu.Unlock()
		a.postStatus("Camera test: virtual Xbox Right Stick X = full RIGHT for 0.8 seconds.")
		return
	case id == idHidHideSetup && notify == BN_CLICKED:
		a.openHidHideSetup()
		return
	case id == idSave && notify == BN_CLICKED:
		a.syncConfigFromUI()
		saveConfig()
		a.postStatus("Settings saved.")
		return
	case id == idRefresh && notify == BN_CLICKED:
		a.syncConfigFromUI()
		a.cfgMu.RLock()
		backend := normalizeInputBackend(a.cfg.InputBackend)
		a.cfgMu.RUnlock()
		if backend == "WinMM" {
			a.devicesMu.Lock()
			a.devices = enumerateDevices()
			a.devicesMu.Unlock()
		} else {
			_ = a.di.Restart(a.hwnd)
		}
		a.populateCombos()
		a.applyConfigToUI()
		if backend == "WinMM" {
			a.postStatus("WinMM compatibility device list refreshed through the isolated helper.")
		} else {
			a.postStatus("DirectInput device list refreshed.")
		}
		return
	case id == idDetectLeft && notify == BN_CLICKED:
		a.startDetect("Left")
		return
	case id == idDetectRight && notify == BN_CLICKED:
		a.startDetect("Right")
		return
	case id == idDetectWASD && notify == BN_CLICKED:
		a.startDetectThumbstick()
		return
	case id >= idWASDClearBase && id < idWASDClearBase+len(wasdButtonDefs) && notify == BN_CLICKED:
		def := wasdButtonDefs[id-idWASDClearBase]
		a.clearBinding(def.Key, def.Label+" (on-foot)")
		return
	case id >= idLearnClearBase && id < idLearnClearBase+len(buttonOutputs) && notify == BN_CLICKED:
		def := buttonOutputs[id-idLearnClearBase]
		a.clearBinding(def.Key, def.Label)
		return
	case id >= idWASDLearnBase && id < idWASDLearnBase+len(wasdButtonDefs) && notify == BN_CLICKED:
		def := wasdButtonDefs[id-idWASDLearnBase]
		a.startLearn(def.Key, def.Label+" (on-foot)")
		return
	case id >= idLearnBase && id < idLearnBase+len(buttonOutputs) && notify == BN_CLICKED:
		a.startLearn(buttonOutputs[id-idLearnBase].Key, buttonOutputs[id-idLearnBase].Label)
		return
	}
	if !a.uiReady {
		return
	}
	if id == idInputBackend && notify == CBN_SELCHANGE {
		backends := []string{"Auto", "DirectInput", "WinMM"}
		idx := comboIndex(idInputBackend)
		if idx < 0 || idx >= len(backends) {
			idx = 0
		}
		a.cfgMu.Lock()
		a.cfg.InputBackend = backends[idx]
		a.cfgMu.Unlock()
		if backends[idx] == "WinMM" {
			a.devicesMu.Lock()
			a.devices = enumerateDevices()
			a.devicesMu.Unlock()
		}
		a.populateCombos()
		a.applyConfigToUI()
		a.updateContextVisibility()
		saveConfig()
		a.postStatus("Input API changed to " + backends[idx] + ". DirectInput exposes additional rotation/slider axes; WinMM remains available as the compatibility fallback.")
		return
	}
	if notify == CBN_SELCHANGE || notify == EN_CHANGE || notify == BN_CLICKED {
		a.syncConfigFromUI()
		saveConfig()
		if notify == CBN_SELCHANGE {
			a.updateContextVisibility()
		}
	}
}

func (a *appState) mappingStatus() string {
	if a.xboxReady.Load() {
		s := a.xboxSlot.Load()
		if s >= 0 {
			return fmt.Sprintf("Mapping ACTIVE. Windows sees Xbox controller in XInput slot %d.", s+1)
		}
		return "Mapping ACTIVE. Virtual Xbox controller is running."
	}
	return "Mapping ACTIVE, but Xbox output is not ready yet."
}

func (a *appState) refreshXboxUI() {
	if a.controls[idStart] == 0 {
		return
	}
	if a.mappingStarting.Load() {
		setText(a.controls[idStart], "Initiating...")
		return
	}
	if a.mappingActive.Load() && a.xboxReady.Load() {
		setText(a.controls[idStart], "Deactivate Xbox Controller")
		return
	}
	setText(a.controls[idStart], "Initiate Xbox Controller")
}

func (a *appState) refreshXInputReadback() {
	h := a.controls[idXInputReadback]
	if h == 0 {
		return
	}
	if !a.xboxReady.Load() {
		setText(h, "XInput readback: disconnected (virtual Xbox exists only during mapping)")
		return
	}
	slot := int(a.xboxSlot.Load())
	if slot < 0 {
		setText(h, "XInput readback: virtual Xbox connected; slot not identified yet")
		return
	}
	st, ok := xinputReadState(slot)
	if !ok {
		setText(h, fmt.Sprintf("XInput readback: slot %d not readable", slot+1))
		return
	}
	g := st.Gamepad
	setText(h, fmt.Sprintf(
		"XInput slot %d  LX %+.2f  LY %+.2f  RX %+.2f  RY %+.2f  LT %3d  RT %3d  Btn 0x%04X",
		slot+1,
		stickReadback(g.ThumbLX), stickReadback(g.ThumbLY),
		stickReadback(g.ThumbRX), stickReadback(g.ThumbRY),
		g.LeftTrigger, g.RightTrigger, g.Buttons,
	))
}

func (a *appState) setButtonTileStates(highlights uint32) {
	for i := range buttonOutputs {
		h := a.controls[idLearnBase+200+i]
		if h == 0 {
			continue
		}
		state := uintptr(0)
		if highlights&(uint32(1)<<uint(i)) != 0 {
			state = 1
		}
		procSendMessageW.Call(h, BM_SETSTATE, state, 0)
	}
}

func (a *appState) setOnFootButtonStates(highlights uint8) {
	for i := range wasdButtonDefs {
		h := a.controls[idWASDIndicatorBase+i]
		if h == 0 {
			continue
		}
		state := uintptr(0)
		if highlights&(uint8(1)<<uint(i)) != 0 {
			state = 1
		}
		procSendMessageW.Call(h, BM_SETSTATE, state, 0)
	}
}

const stickDotSize = 12

// stickDotOffset maps a -1..1 x/y pair (y positive = forward/up) to the dot's
// pixel offset from its centered position inside a square box. NaN -> center.
func stickDotOffset(boxSize int, x, y float64) (float64, float64) {
	if math.IsNaN(x) {
		x = 0
	}
	if math.IsNaN(y) {
		y = 0
	}
	rangePx := float64(boxSize-stickDotSize) / 2.0
	return clamp(x, -1, 1) * rangePx, -clamp(y, -1, 1) * rangePx
}

func (a *appState) moveStickDot(id, boxSize int, x, y float64) {
	h := a.controls[id]
	base, ok := a.baseRects[h]
	if h == 0 || !ok {
		return
	}
	dx, dy := stickDotOffset(boxSize, x, y)
	// Anchor movement to the dot's actual registered layout position.
	// The twist-axis insert shifts that position down by 210px, while the
	// original hardcoded 542px monitor origin was never updated in v1.98.
	// Using baseRects also keeps dots correctly aligned during scrolling.
	cx := float64(int(base.Left)-a.scrollX) + dx
	cy := float64(int(base.Top)-a.scrollY) + dy
	var old rect
	procGetWindowRect.Call(h, uintptr(unsafe.Pointer(&old)))
	procScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&old.Left)))
	procScreenToClient.Call(a.hwnd, uintptr(unsafe.Pointer(&old.Right)))
	clipChanged := a.applyChildClip(h, rect{Left: int32(cx), Top: int32(cy), Right: int32(cx) + stickDotSize, Bottom: int32(cy) + stickDotSize}, a.currentContentViewport())
	procSetWindowPos.Call(
		h,
		HWND_TOP,
		uintptr(int(cx)),
		uintptr(int(cy)),
		uintptr(stickDotSize),
		uintptr(stickDotSize),
		SWP_NOACTIVATE|SWP_NOCOPYBITS,
	)
	if clipChanged {
		// The region changed without redraw; repaint the area the dot left.
		procRedrawWindow.Call(a.hwnd, uintptr(unsafe.Pointer(&old)), 0, RDW_INVALIDATE|RDW_ERASE|RDW_ALLCHILDREN)
	}
	redrawLiveMonitorDot(h)
}

func (a *appState) raiseLiveMonitorDots() {
	for _, id := range []int{idLeftDot, idRightDot, idDriveSteerDot} {
		if h := a.controls[id]; h != 0 {
			procSetWindowPos.Call(h, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
			redrawLiveMonitorDot(h)
		}
	}
}

func redrawLiveMonitorDot(h uintptr) {
	procRedrawWindow.Call(h, 0, 0, RDW_INVALIDATE|RDW_NOERASE|RDW_UPDATENOW)
}

func (a *appState) refreshMonitor() {
	a.refreshXInputReadback()
	a.monitorMu.RLock()
	m := a.monitor
	a.monitorMu.RUnlock()
	a.cfgMu.RLock()
	c := a.cfg
	a.cfgMu.RUnlock()

	const boxSize = 126

	// The live visual layer is intentionally suspended while controller mapping is active.
	// This leaves the fast 100 Hz input/output loop focused on the game.
	if a.mappingActive.Load() {
		if !a.visualsPaused {
			setText(a.controls[idMonitor], "Live visual monitor PAUSED while game mapping is active.\r\nController polling/output remains active at the selected polling rate.")
			a.setButtonTileStates(0)
			a.setOnFootButtonStates(0)
			a.moveStickDot(idLeftDot, boxSize, 0, 0)
			a.moveStickDot(idRightDot, boxSize, 0, 0)
			a.moveStickDot(idDriveSteerDot, boxSize, 0, 0)
			setText(a.controls[idLeftVectorText], "Monitor paused")
			setText(a.controls[idRightVectorText], "Monitor paused")
			setText(a.controls[idDriveSteerVectorText], "Monitor paused")
			a.liveLeftThrust = 0
			a.liveRightThrust = 0
			a.redrawLiveThrustMeters()
			a.visualsPaused = true
		}
		return
	}
	a.visualsPaused = false
	leftThrust := clamp(m.Left, -1, 1)
	rightThrust := clamp(m.Right, -1, 1)
	if math.Abs(leftThrust-a.liveLeftThrust) >= 0.01 || math.Abs(rightThrust-a.liveRightThrust) >= 0.01 {
		a.liveLeftThrust = leftThrust
		a.liveRightThrust = rightThrust
		a.redrawLiveThrustMeters()
	}

	mode := fmt.Sprintf("Podracer Style: Max throttle %d%%  |  Throttle curve %d%%  |  Max steering %d%%  |  Steer curve %d/%d%%", normalizeScalePercent(c.ThrottleScale), normalizeExpoPercent(c.ThrottleExpo), normalizeSteerScalePercent(c.SteerScale), normalizeExpoPercent(c.SteerExpo), normalizeExpoPercent(c.SteerExpo2))
	if strings.EqualFold(normalizeControlMode(c.ControlMode), "HOTAS") {
		mode = "Traditional HOTAS"
	}
	pedal := "Off"
	if c.PedalID >= 0 {
		pedal = fmt.Sprintf("%+.2f", m.Pedal)
	}
	camera := "Off"
	if strings.EqualFold(c.PedalMode, "Move Camera") {
		camera = fmt.Sprintf("RX %+.2f", m.CameraX)
	} else if strings.EqualFold(c.PedalMode, "Pitch") {
		camera = fmt.Sprintf("Pitch LY %+.2f", m.Pedal)
	} else if strings.EqualFold(c.PedalMode, "Keyboard keys") {
		camera = fmt.Sprintf("Keys %s / %s @ %d%%", keyName(c.PedalLeftKey), keyName(c.PedalRightKey), c.PedalKeyThreshold)
	}
	var s string
	if strings.EqualFold(normalizeControlMode(c.ControlMode), "HOTAS") {
		brakeText := fmt.Sprintf("%.0f%%", m.HotasBrakeAmount*100.0)
		s = fmt.Sprintf(
			"Throttle %.2f  Brake %s  Stick X %+.2f  Stick Y %+.2f\r\nTwist L %+.2f  R %+.2f  Pedal %s  Output %s\r\n%s",
			m.Throttle, brakeText, m.LeftX, m.LeftY, m.TwistLeft, m.TwistRight, pedal, camera, mode,
		)
	} else {
		s = fmt.Sprintf(
			"Left %+.2f  Right %+.2f  Drive %+.2f  Steer %+.2f\r\nTwist L %+.2f  R %+.2f  Pedal %s  Output %s\r\n%s",
			m.Left, m.Right, m.Drive, m.Steer, m.TwistLeft, m.TwistRight, pedal, camera, mode,
		)
	}
	setText(a.controls[idMonitor], s)

	a.moveStickDot(idLeftDot, boxSize, m.LeftX, m.LeftY)
	a.moveStickDot(idRightDot, boxSize, m.RightX, m.RightY)
	a.moveStickDot(idDriveSteerDot, boxSize, m.Steer, m.Drive)
	setText(a.controls[idLeftVectorText], fmt.Sprintf("Side %+0.2f  F %+0.2f", m.LeftX, m.LeftY))
	setText(a.controls[idRightVectorText], fmt.Sprintf("Side %+0.2f  F %+0.2f", m.RightX, m.RightY))
	driveDir := "Neutral"
	if m.Drive > 0.01 {
		driveDir = "Forward"
	} else if m.Drive < -0.01 {
		driveDir = "Brake/Reverse"
	}
	steerDir := "Straight"
	if m.Steer > 0.01 {
		steerDir = "Right"
	} else if m.Steer < -0.01 {
		steerDir = "Left"
	}
	setText(a.controls[idDriveSteerVectorText], fmt.Sprintf("Drive %+0.2f  %s\r\nSteer %+0.2f  %s", m.Drive, driveDir, m.Steer, steerDir))
	a.setButtonTileStates(m.OutputHighlights)
	a.setOnFootButtonStates(m.OnFootHighlights)
}

func (a *appState) postStatus(s string) {
	a.pendingMu.Lock()
	a.statusQueue = append(a.statusQueue, s)
	a.pendingMu.Unlock()
	procPostMessageW.Call(a.hwnd, WM_APP_STATUS, 0, 0)
}
func (a *appState) drainStatus() {
	a.pendingMu.Lock()
	q := append([]string(nil), a.statusQueue...)
	a.statusQueue = nil
	a.pendingMu.Unlock()
	if len(q) > 0 {
		setText(a.controls[idStatus], q[len(q)-1])
	}
}

func (a *appState) startDetect(side string) {
	a.cfgMu.RLock()
	backend := normalizeInputBackend(a.cfg.InputBackend)
	a.cfgMu.RUnlock()
	if backend != "WinMM" {
		go func() {
			guid, name, axis, _, span, score, ok := a.detectDirectInputAxis(
				fmt.Sprintf("Detecting %s stick through DirectInput: PUSH THAT STICK FORWARD now...", side), "", -1,
			)
			if !ok {
				if backend == "Auto" {
					a.postStatus(fmt.Sprintf("DirectInput did not see a strong %s movement (score %.2f); Auto is retrying with WinMM...", side, score))
					a.startDetectWinMM(side)
					return
				}
				a.queueDetect(detectResult{Side: side, Message: fmt.Sprintf("DirectInput did not see a strong axis movement (score %.2f). Try Refresh devices or switch Input API to WinMM compatibility.", score)})
				return
			}
			a.queueDetect(detectResult{Side: side, DeviceID: a.matchWinMMIDByName(name), Axis: axis, Invert: span < 0, DeviceName: name, OK: true, UseDirectInput: true, DISideGUID: guid})
		}()
		return
	}
	a.startDetectWinMM(side)
}

func (a *appState) startDetectWinMM(side string) {
	a.postStatus(fmt.Sprintf("Detecting %s stick through WinMM: PUSH THAT STICK FORWARD now...", side))
	go func() {
		devs := enumerateDevices()
		if len(devs) == 0 {
			a.queueDetect(detectResult{Side: side, Message: "No joystick devices detected."})
			return
		}
		type baseKey struct{ dev, axis int }
		bases := map[baseKey]float64{}
		for _, d := range devs {
			if st, ok := readJoy(d.ID); ok {
				for ax := 0; ax < 6; ax++ {
					bases[baseKey{d.ID, ax}] = axisNormalized(d.ID, ax, false, st)
				}
			}
		}
		bestDelta := 0.0
		bestDev := -1
		bestAxis := 0
		bestVal := 0.0
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			for _, d := range devs {
				if st, ok := readJoy(d.ID); ok {
					for ax := 0; ax < 6; ax++ {
						v := axisNormalized(d.ID, ax, false, st)
						delta := math.Abs(v - bases[baseKey{d.ID, ax}])
						if delta > bestDelta {
							bestDelta = delta
							bestDev = d.ID
							bestAxis = ax
							bestVal = v - bases[baseKey{d.ID, ax}]
						}
					}
				}
			}
			if bestDelta > 0.65 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if bestDev < 0 || bestDelta < 0.30 {
			a.queueDetect(detectResult{Side: side, Message: "No strong axis movement detected. Click Detect and make one large forward movement."})
			return
		}
		name := fmt.Sprintf("Joystick %d", bestDev)
		for _, d := range devs {
			if d.ID == bestDev {
				name = d.Name
				break
			}
		}
		a.queueDetect(detectResult{Side: side, DeviceID: bestDev, Axis: bestAxis, Invert: bestVal < 0, DeviceName: name, OK: true})
	}()
}

func (a *appState) detectStrongAxis(devs []deviceInfo, restrictDev, excludeDev, excludeAxis int, prompt string, minDelta float64) (int, int, bool, float64, bool) {
	a.postStatus(prompt)
	type baseKey struct{ dev, axis int }
	bases := map[baseKey]float64{}
	for _, d := range devs {
		if restrictDev >= 0 && d.ID != restrictDev {
			continue
		}
		if st, ok := readJoy(d.ID); ok {
			for ax := 0; ax < 6; ax++ {
				if d.ID == excludeDev && ax == excludeAxis {
					continue
				}
				bases[baseKey{d.ID, ax}] = axisNormalized(d.ID, ax, false, st)
			}
		}
	}
	bestDelta, bestSigned := 0.0, 0.0
	bestDev, bestAxis := -1, -1
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		for _, d := range devs {
			if restrictDev >= 0 && d.ID != restrictDev {
				continue
			}
			st, ok := readJoy(d.ID)
			if !ok {
				continue
			}
			for ax := 0; ax < 6; ax++ {
				if d.ID == excludeDev && ax == excludeAxis {
					continue
				}
				base, ok := bases[baseKey{d.ID, ax}]
				if !ok {
					continue
				}
				v := axisNormalized(d.ID, ax, false, st)
				signed := v - base
				delta := math.Abs(signed)
				if delta > bestDelta {
					bestDelta, bestSigned, bestDev, bestAxis = delta, signed, d.ID, ax
				}
			}
		}
		if bestDelta > 0.65 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return bestDev, bestAxis, bestSigned < 0, bestDelta, bestDelta >= minDelta
}

func (a *appState) startDetectThumbstick() {
	go func() {
		sideGUID, sideName, sideAxis, sideCenter, sideSpan, sideScore, ok := a.detectDirectInputAxis(
			"Thumbstick detect 1/2 (DirectInput): push the small thumbstick RIGHT now...", "", -1,
		)
		if !ok {
			a.queueDetect(detectResult{Side: "Thumbstick", Message: fmt.Sprintf("DirectInput did not see a strong RIGHT movement (score %.2f). Move the thumbstick while watching the DirectInput axis monitor; X Rotation (Rx) or Y Rotation (Ry) should change.", sideScore)})
			return
		}
		a.postStatus(fmt.Sprintf("DirectInput found side axis: %s on %s. Return to center, then push FORWARD...", diAxisNames[sideAxis], sideName))
		deadline := time.Now().Add(3 * time.Second)
		tolerance := int32(math.Max(math.Abs(float64(sideSpan))*0.30, 256))
		for time.Now().Before(deadline) {
			if axes, ok := a.di.State(sideGUID); ok && int64Abs(int64(axes[sideAxis])-int64(sideCenter)) <= int64(tolerance) {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		time.Sleep(250 * time.Millisecond)
		forwardGUID, forwardName, forwardAxis, forwardCenter, forwardSpan, forwardScore, ok := a.detectDirectInputAxis(
			"Thumbstick detect 2/2 (DirectInput): push the small thumbstick FORWARD now...", sideGUID, sideAxis,
		)
		if !ok {
			a.queueDetect(detectResult{Side: "Thumbstick", Message: fmt.Sprintf("DirectInput found RIGHT on %s but did not see a strong FORWARD movement (score %.2f). Watch the DirectInput monitor and note which axis changes when you push forward.", diAxisNames[sideAxis], forwardScore)})
			return
		}
		name := sideName
		if !strings.EqualFold(sideGUID, forwardGUID) {
			name = fmt.Sprintf("%s + %s", sideName, forwardName)
		}
		a.queueDetect(detectResult{
			Side: "Thumbstick", OK: true, UseDirectInput: true, DeviceName: name,
			Axis: sideAxis, Axis2: forwardAxis,
			DISideGUID: sideGUID, DIForwardGUID: forwardGUID,
			DISideCenter: sideCenter, DIForwardCenter: forwardCenter,
			DISideSpan: sideSpan, DIForwardSpan: forwardSpan,
		})
	}()
}

func int64Abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (a *appState) detectThumbstickLegacy() {
	devs := enumerateDevices()
	if len(devs) == 0 {
		a.queueDetect(detectResult{Side: "Thumbstick", Message: "No joystick devices detected."})
		return
	}
	dev, sideAxis, sideInvert, sideDelta, ok := a.detectStrongAxis(devs, -1, -1, -1, "Legacy thumbstick detect 1/2: push the small thumbstick RIGHT now...", 0.12)
	if !ok {
		a.queueDetect(detectResult{Side: "Thumbstick", Message: fmt.Sprintf("Thumbstick side axis not detected (strongest movement %.2f).", sideDelta)})
		return
	}
	a.postStatus("Legacy side axis found. Return it to center, then push FORWARD...")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st, ok := readJoy(dev); ok && math.Abs(axisNormalized(dev, sideAxis, sideInvert, st)) < 0.25 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	dev2, forwardAxis, forwardInvert, forwardDelta, ok := a.detectStrongAxis(devs, -1, dev, sideAxis, "Legacy thumbstick detect 2/2: push the thumbstick FORWARD now...", 0.10)
	if !ok {
		a.queueDetect(detectResult{Side: "Thumbstick", Message: fmt.Sprintf("Legacy forward axis not detected either (strongest movement %.2f). Use the Raw HID monitor to verify Rx/Ry activity.", forwardDelta)})
		return
	}
	if dev2 != dev {
		a.queueDetect(detectResult{Side: "Thumbstick", Message: fmt.Sprintf("Legacy reader split the axes across joystick %d and %d. Raw HID should handle this; use the Raw HID monitor to inspect the two rotation axes.", dev, dev2)})
		return
	}
	name := fmt.Sprintf("Joystick %d", dev)
	for _, d := range devs {
		if d.ID == dev {
			name = d.Name
			break
		}
	}
	a.queueDetect(detectResult{Side: "Thumbstick", DeviceID: dev, Axis: sideAxis, Axis2: forwardAxis, Invert: sideInvert, Invert2: forwardInvert, DeviceName: name, OK: true})
}

func (a *appState) queueDetect(r detectResult) {
	a.pendingMu.Lock()
	a.detectQueue = append(a.detectQueue, r)
	a.pendingMu.Unlock()
	procPostMessageW.Call(a.hwnd, WM_APP_DETECT, 0, 0)
}
func (a *appState) drainDetect() {
	a.pendingMu.Lock()
	q := append([]detectResult(nil), a.detectQueue...)
	a.detectQueue = nil
	a.pendingMu.Unlock()
	for _, r := range q {
		if !r.OK {
			a.postStatus(r.Message)
			continue
		}
		a.cfgMu.Lock()
		if r.Side == "Left" {
			if r.UseDirectInput {
				a.cfg.LeftDIGUID = r.DISideGUID
				if r.DeviceID >= 0 {
					a.cfg.LeftID = r.DeviceID
				}
			} else {
				a.cfg.LeftID = r.DeviceID
			}
			a.cfg.LeftAxis = r.Axis
			a.cfg.LeftInvert = r.Invert
		} else if r.Side == "Right" {
			if r.UseDirectInput {
				a.cfg.RightDIGUID = r.DISideGUID
				if r.DeviceID >= 0 {
					a.cfg.RightID = r.DeviceID
				}
			} else {
				a.cfg.RightID = r.DeviceID
			}
			a.cfg.RightAxis = r.Axis
			a.cfg.RightInvert = r.Invert
		} else if r.Side == "Thumbstick" {
			if r.UseDirectInput {
				a.cfg.OnFootMode = "Thumbstick -> WASD"
				a.cfg.ThumbWASDEnabled = true
				a.cfg.ThumbWASDUseRawHID = false
				a.cfg.ThumbDISideGUID = r.DISideGUID
				a.cfg.ThumbDIForwardGUID = r.DIForwardGUID
				a.cfg.ThumbDIDeviceName = r.DeviceName
				a.cfg.ThumbDISideAxis = r.Axis
				a.cfg.ThumbDIForwardAxis = r.Axis2
				a.cfg.ThumbDISideCenter = r.DISideCenter
				a.cfg.ThumbDIForwardCenter = r.DIForwardCenter
				a.cfg.ThumbDISideSpan = r.DISideSpan
				a.cfg.ThumbDIForwardSpan = r.DIForwardSpan
				a.cfg.ThumbWASDInvertSide = false
				a.cfg.ThumbWASDInvertForward = false
			}
		}
		a.cfgMu.Unlock()
		a.applyConfigToUI()
		saveConfig()
		if r.Side == "Thumbstick" {
			if r.UseDirectInput {
				a.postStatus(fmt.Sprintf("Thumbstick detected through DirectInput: %s, side=%s, forward=%s. Right and forward are calibrated positive.", r.DeviceName, diAxisNames[r.Axis], diAxisNames[r.Axis2]))
			}
		} else {
			inv := "normal"
			if r.Invert {
				inv = "inverted"
			}
			if r.UseDirectInput {
				axisName := fmt.Sprintf("axis %d", r.Axis)
				if r.Axis >= 0 && r.Axis < len(diAxisNames) {
					axisName = diAxisNames[r.Axis]
				}
				a.postStatus(fmt.Sprintf("%s detected through DirectInput: %s, %s (%s). Forward is now positive.", r.Side, r.DeviceName, axisName, inv))
			} else {
				axisName := fmt.Sprintf("axis %d", r.Axis)
				if r.Axis >= 0 && r.Axis < len(a.axisNames) {
					axisName = a.axisNames[r.Axis]
				}
				a.postStatus(fmt.Sprintf("%s detected through WinMM: %s [ID %d], %s (%s). Forward is now positive.", r.Side, r.DeviceName, r.DeviceID, axisName, inv))
			}
		}
	}
}

func (a *appState) startLearn(key, label string) {
	a.cfgMu.RLock()
	backend := normalizeInputBackend(a.cfg.InputBackend)
	a.cfgMu.RUnlock()
	a.postStatus(fmt.Sprintf("Learning %s using %s: press the controller button, trigger, hat, or switch position you want...", label, backend))
	go func() {
		var b binding
		var ok bool
		switch backend {
		case "WinMM":
			b, ok = a.learnWinMM(5 * time.Second)
		case "DirectInput":
			b, ok = a.learnDirectInput(5 * time.Second)
		default:
			b, ok = a.learnAuto(5 * time.Second)
		}
		if ok {
			a.queueLearn(learnResult{Key: key, Binding: b, OK: true})
			return
		}
		a.queueLearn(learnResult{Key: key, Message: "Nothing detected. Try Learn again and press/move the control after clicking Learn. If the device is visible only through one API, select DirectInput or WinMM explicitly."})
	}()
}

func (a *appState) learnWinMM(timeout time.Duration) (binding, bool) {
	devs := enumerateDevices()
	type baseline struct {
		buttons uint32
		pov     uint32
	}
	base := map[int]baseline{}
	for _, d := range devs {
		if st, ok := readJoy(d.ID); ok {
			base[d.ID] = baseline{st.Buttons, st.POV}
		}
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, d := range devs {
			st, ok := readJoy(d.ID)
			if !ok {
				continue
			}
			bl := base[d.ID]
			pressed := st.Buttons &^ bl.buttons
			if pressed != 0 {
				for i := 0; i < 32; i++ {
					if pressed&(1<<uint(i)) != 0 {
						return binding{Kind: "button", Backend: "WinMM", DeviceID: d.ID, DeviceName: d.Name, Button: i}, true
					}
				}
			}
			if st.POV != JOY_POVCENTERED && st.POV != bl.pov {
				return binding{Kind: "pov", Backend: "WinMM", DeviceID: d.ID, DeviceName: d.Name, POVIndex: 0, POV: int(st.POV)}, true
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	return binding{}, false
}

func detectDirectInputLearn(base map[string]diJoyState, snaps []diFullSnapshot) (binding, bool) {
	for _, snap := range snaps {
		bl, exists := base[snap.GUID]
		if !exists {
			continue
		}
		for i := 0; i < len(snap.State.Buttons); i++ {
			now := snap.State.Buttons[i]&0x80 != 0
			was := bl.Buttons[i]&0x80 != 0
			if now && !was {
				return binding{Kind: "button", Backend: "DirectInput", DeviceID: -1, DIGUID: snap.GUID, DeviceName: snap.Name, Button: i}, true
			}
		}
		for i := 0; i < len(snap.State.POV); i++ {
			now := snap.State.POV[i]
			was := bl.POV[i]
			if now != 0xFFFFFFFF && now != was {
				return binding{Kind: "pov", Backend: "DirectInput", DeviceID: -1, DIGUID: snap.GUID, DeviceName: snap.Name, POVIndex: i, POV: int(now)}, true
			}
		}
	}
	return binding{}, false
}

func (a *appState) learnDirectInput(timeout time.Duration) (binding, bool) {
	base := make(map[string]diJoyState)
	for _, snap := range a.di.FullSnapshots() {
		base[snap.GUID] = snap.State
	}
	if len(base) == 0 {
		return binding{}, false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b, ok := detectDirectInputLearn(base, a.di.FullSnapshots()); ok {
			return b, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return binding{}, false
}

func (a *appState) learnAuto(timeout time.Duration) (binding, bool) {
	// Auto watches both APIs at the same time. DirectInput is checked first on every
	// cycle so a device visible through both APIs is saved as a DirectInput binding.
	diBase := make(map[string]diJoyState)
	for _, snap := range a.di.FullSnapshots() {
		diBase[snap.GUID] = snap.State
	}
	devs := enumerateDevices()
	type winBase struct {
		buttons uint32
		pov     uint32
	}
	wmBase := map[int]winBase{}
	for _, d := range devs {
		if st, ok := readJoy(d.ID); ok {
			wmBase[d.ID] = winBase{st.Buttons, st.POV}
		}
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(diBase) > 0 {
			if b, ok := detectDirectInputLearn(diBase, a.di.FullSnapshots()); ok {
				return b, true
			}
		}
		for _, d := range devs {
			st, ok := readJoy(d.ID)
			if !ok {
				continue
			}
			bl := wmBase[d.ID]
			pressed := st.Buttons &^ bl.buttons
			if pressed != 0 {
				for i := 0; i < 32; i++ {
					if pressed&(1<<uint(i)) != 0 {
						return binding{Kind: "button", Backend: "WinMM", DeviceID: d.ID, DeviceName: d.Name, Button: i}, true
					}
				}
			}
			if st.POV != JOY_POVCENTERED && st.POV != bl.pov {
				return binding{Kind: "pov", Backend: "WinMM", DeviceID: d.ID, DeviceName: d.Name, POVIndex: 0, POV: int(st.POV)}, true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return binding{}, false
}

func (a *appState) queueLearn(r learnResult) {
	a.pendingMu.Lock()
	a.learnQueue = append(a.learnQueue, r)
	a.pendingMu.Unlock()
	procPostMessageW.Call(a.hwnd, WM_APP_LEARN, 0, 0)
}
func (a *appState) clearBinding(key, label string) {
	a.cfgMu.Lock()
	if a.cfg.Bindings != nil {
		delete(a.cfg.Bindings, key)
	}
	a.cfgMu.Unlock()
	saveConfig()
	a.updateBindingLabels()
	a.postStatus(label + " cleared.")
}

func (a *appState) drainLearn() {
	a.pendingMu.Lock()
	q := append([]learnResult(nil), a.learnQueue...)
	a.learnQueue = nil
	a.pendingMu.Unlock()
	for _, r := range q {
		if !r.OK {
			a.postStatus(r.Message)
			continue
		}
		a.cfgMu.Lock()
		if a.cfg.Bindings == nil {
			a.cfg.Bindings = make(map[string]binding)
		}
		a.cfg.Bindings[r.Key] = r.Binding
		a.cfgMu.Unlock()
		saveConfig()
		a.updateBindingLabels()
		a.postStatus(fmt.Sprintf("%s assigned to %s.", outputLabel(r.Key), bindingDisplay(r.Binding)))
	}
}
func outputLabel(k string) string {
	for _, b := range buttonOutputs {
		if b.Key == k {
			return b.Label
		}
	}
	for _, b := range wasdButtonDefs {
		if b.Key == k {
			return b.Label + " (on-foot)"
		}
	}
	return k
}
func (a *appState) updateBindingLabels() {
	a.cfgMu.RLock()
	c := a.cfg
	a.cfgMu.RUnlock()
	for i, bo := range buttonOutputs {
		s := "Not assigned"
		if b, ok := c.Bindings[bo.Key]; ok {
			s = bindingDisplay(b)
		}
		setText(a.controls[idLearnBase+100+i], s)
		a.tooltipForID(idLearnBase+100+i, "Assigned to "+bo.Label+": "+s)
	}
	for i, def := range wasdButtonDefs {
		s := "Not assigned"
		if b, ok := c.Bindings[def.Key]; ok {
			s = bindingDisplay(b)
		}
		setText(a.controls[idWASDLabelBase+i], s)
	}
}
func bindingDisplay(b binding) string {
	backend := normalizeBindingBackend(b.Backend)
	if backend == "DirectInput" || b.DIGUID != "" {
		name := strings.TrimSpace(b.DeviceName)
		if name == "" {
			name = "DirectInput device"
		}
		if strings.EqualFold(b.Kind, "button") {
			return fmt.Sprintf("%s / Button %d [DI]", name, b.Button+1)
		}
		if strings.EqualFold(b.Kind, "pov") {
			return fmt.Sprintf("%s / POV %d %s [DI]", name, b.POVIndex+1, povName(b.POV))
		}
	}
	if strings.EqualFold(b.Kind, "button") {
		return fmt.Sprintf("Joystick %d / Button %d [WinMM]", b.DeviceID, b.Button+1)
	}
	if strings.EqualFold(b.Kind, "pov") {
		return fmt.Sprintf("Joystick %d / POV %s [WinMM]", b.DeviceID, povName(b.POV))
	}
	return "Not assigned"
}
func povName(v int) string {
	switch {
	case v < 2250 || v >= 33750:
		return "Up"
	case v < 6750:
		return "Up-Right"
	case v < 11250:
		return "Right"
	case v < 15750:
		return "Down-Right"
	case v < 20250:
		return "Down"
	case v < 24750:
		return "Down-Left"
	case v < 29250:
		return "Left"
	default:
		return "Up-Left"
	}
}
