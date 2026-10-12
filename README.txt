GALACTIC RACER HOSAS / HOTAS BRIDGE — v1.98.2
by JaffaQuake and Mars
=========================================

WHAT'S NEW
----------
1. TWIST AXIS — Independent Left Twist Axis and Right Twist Axis sections immediately
   above Rudder Pedals. Each has Device, Axis, Invert, Deadzone, Axis Type, and Mode.
   Centered is the default (-100% to +100%). Lever ranges 0% to 100%.
   Centered and Lever are mutually exclusive selections.

2. TWIST MODES — Each twist supports Off, Fine Steering, Move Camera, Pitch,
   and Keyboard Keys. Fine-steering strength, camera sensitivity/deadzone, and
   keyboard key/threshold settings are independent for each twist.
   Fine Steering affects Xbox Left Stick X, Move Camera affects Right Stick X,
   and Pitch affects Left Stick Y. Outputs from multiple analog sources are
   combined and clamped to the controller range.

3. REORGANIZED BUTTON MAPPING — Standard Xbox mappings first in two columns,
   followed by 'Galactic Racer Specific Mappings' with:
     Pitch Up / Left Stick ↑       Pitch Down / Left Stick ↓
     Look Left / Right Stick ←     Look Right / Right Stick →
     Look Back / Right Stick ↓
   All entries retain Learn and Clear. Look commands STILL send their established
   native keyboard keys (Delete, Page Down, End); Xbox direction labels are
   explanatory only. Learned digital Pitch Up/Down still send Xbox Left Stick Y.
   Mapping columns have more separation and room for direction labels.

4. COMPATIBILITY — Existing v1.97 settings and button bindings are preserved.
   New twist inputs default to 'None', mode Off, axis type Centered. The
   isolated WinMM helper and input backend selection are unchanged.

INPUT / OUTPUT NOTES
--------------------
- Use the selected backend (Auto, DirectInput, WinMM) to choose each twist device
  and axis. DirectInput exposes up to eight axes, WinMM up to six.
- Lever mode cannot produce negative axis values; when keyboard keys are used,
  a lever can fire the positive key only. Invert reverses travel for lever
  mode (0% ↔ 100%), not the sign.
- Centered axis Invert reverses positive and negative directions.
- Only modes that need extra settings show a second row for that twist.
- Existing pedal mode Pitch and learned digital Pitch commands still work.
- Live visual monitor is paused during active game mapping to reduce overhead.

INSTALL
-------
1. Extract the ZIP to a folder.
2. Double-click Install.cmd on Windows x64 and follow the installer.
3. Select devices and axes, choose the mode, then click Save settings.
4. Initiate Xbox Controller only after ViGEmBus and ViGEmClient are installed.

CONFIG (preserved by installer)
-------------------------------
%LOCALAPPDATA%\GalacticRacerHOSAS\hosas_config.json

SOURCE
------
Matching Go source, WinMMHelper source and Build-Local.cmd are in Source.

UI improvements in v1.98.2: full-height assignment fields, clickable two-line button labels, and live monitor dots that remain in their grids when scrolling.

v1.98.2 layout: Twist Axis shifted closer to Rudder Pedals; redundant axis-type note removed.
