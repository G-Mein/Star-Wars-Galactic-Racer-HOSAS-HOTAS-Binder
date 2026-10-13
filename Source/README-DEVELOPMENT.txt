GALACTIC RACER HOSAS / HOTAS BRIDGE - LOCAL DEVELOPMENT (v2.0)
================================================================

The release includes matching Go source for both executables.

MAIN APP
--------
Source/main.go (window, settings, input polling, mapping)
Source/native_ui.go, scrollbar.go, live_card.go (custom frame, status bar, scrollbar, cards)
Source/curve_plot.go, throttle_curve.go (HOSAS driving response and curve plots)
Source/axis_config.go, binding_keys.go (config helpers)
Source/overlay.go (always-on-top Live Monitor overlay)
Source/*_test.go (unit tests: go test ./...)

WINMM HELPER
------------
Source/WinMMHelper/main.go

Build-Local.cmd builds both executables on Windows. The helper exists only to isolate legacy WinMM
device-capability enumeration from the main process. Users do not launch it directly.

v2.0 INPUT / OUTPUT NOTES
--------------------------
- Auto and DirectInput use the DirectInput device list during normal startup.
- WinMM capability enumeration runs through GalacticRacerWinMMHelper.exe when required.
- DirectInput exposes X/Y/Z, Rx/Ry/Rz, Slider 1/2 plus buttons/POVs.
- Pedal mode Pitch maps the normalized pedal axis to virtual Xbox Left Stick Y.
- Learned PITCHUP / PITCHDOWN bindings generate full-deflection Left Stick Y outputs.
- Digital pitch bindings override analog Left Stick Y while held; both together cancel to neutral.
- Pedal Keyboard Keys mode supports modifier keys including Shift, left/right Control and Alt.
- Twist Lever Output Negative multiplies the deadzone-corrected output by -1;
  Invert still reverses the one-way travel before deadzone. Settings persist
  as twist_left.output_negative and twist_right.output_negative (default false).
- axis_math_test.go checks the invert/polarity combinations and older JSON.

PODRACER THRUST RESPONSE
------------------------
- HOSAS mode has one shared "Max throttle" percentage (10-200, default 100), shown in the Driving Response card below Deadzone.
- Drive = clamp((left + right) x max throttle / 100, -1, 1), applied after the deadzone.
  Examples at 100: one stick full + other centered = 100%; both at 50% = 100%;
  full forward + full back = 0. At 80: one stick full + other centered = 80%.
- Linear, no threshold jump; reverse is symmetric. Steering stays left - right (unscaled).
- Old per-stick full-throttle and hybrid settings are ignored; existing configs start at 100%.
- Learned W/A/S/D bindings retain their canonical keys when existing settings load.

The native Win32 UI follows the accepted dark-card mockup: a persistent left navigation,
profile selector, paired HOSAS stick cards, HOTAS throttle/flight-stick cards, and a visible
thrust-response curve. The resizable, scrollable window groups all existing device, twist,
pedal, on-foot, monitor, and mapping controls into native panels.

DEVELOPMENT SETUP
-----------------
Run Setup-Go.cmd on Windows x64 to install or upgrade Go through winget to the version required by go.mod.
Then run Build-Local.cmd to build both executables.
This setup does not install or change ViGEmBus or ViGEmClient.dll.
