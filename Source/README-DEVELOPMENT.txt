GALACTIC RACER HOSAS / HOTAS BRIDGE - LOCAL DEVELOPMENT (v1.98.2)
================================================================

The release includes matching Go source for both executables.

MAIN APP
--------
Source/main.go

WINMM HELPER
------------
Source/WinMMHelper/main.go

Build-Local.cmd builds both executables on Windows. The helper exists only to isolate legacy WinMM
device-capability enumeration from the main process. Users do not launch it directly.

v1.98.2 INPUT / OUTPUT NOTES
--------------------------
- Auto and DirectInput use the DirectInput device list during normal startup.
- WinMM capability enumeration runs through GalacticRacerWinMMHelper.exe when required.
- DirectInput exposes X/Y/Z, Rx/Ry/Rz, Slider 1/2 plus buttons/POVs.
- Pedal mode Pitch maps the normalized pedal axis to virtual Xbox Left Stick Y.
- Learned PITCHUP / PITCHDOWN bindings generate full-deflection Left Stick Y outputs.
- Digital pitch bindings override analog Left Stick Y while held; both together cancel to neutral.
- Pedal Keyboard Keys mode supports modifier keys including Shift, left/right Control and Alt.

The Win32 UI remains a fixed-size content canvas inside a resizable/scrollable main window.
