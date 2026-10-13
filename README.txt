GALACTIC RACER HOSAS / HOTAS BRIDGE — v2.0
by JaffaQuake and Mars
Modified version with additions by G-Mein (https://github.com/G-Mein)
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

5. PODRACER THRUST RESPONSE — Set left/right stick travel independently for
   reaching full forward thrust. 100% retains linear legacy behavior; lower
   values provide full thrust earlier without changing steering or reverse.
   Overall forward thrust follows the stronger scaled stick, so one stick can
   provide full thrust while the other remains neutral.

6. NATIVE UI — The complete Go/Win32 application follows the accepted dark-card mockup:
   persistent left navigation with separate Steuerung, Buttons, Twist/Pedals, Zu Fuss,
   device/HidHide, and Live Monitor views; fixed profile selector; paired HOSAS stick cards;
   HOTAS throttle/flight-stick cards; and a visible thrust-response curve. Rudder-pedal and
   on-foot controls use separate cards. Existing settings, mappings, monitors, and actions
   remain available; no browser or WebView is used.

MODIFICATIONS BY G-MEIN (2026-10, https://github.com/G-Mein)
------------------------------------------------------------
This is a modified version of the original work (GPL-3.0-or-later, section 5a notice).
Changes by G-Mein:
- Native Windows build of the new dark-card GUI with a custom, mockup-style window frame
  (no standard Windows title bar) and sidebar pages.
- Live Monitor with stick maps for Left Stick, Right Stick and Drive/Steer, also shown on
  the Steuerung page.
- Always-on-top live overlay (top-right corner, close with the red X).
- HOSAS Driving Response:
  - Max throttle 10-200% (above 100% the stick sum is amplified, so full thrust stays
    while the other stick pulls back for steering).
  - Throttle curve from -100 to +100 (positive = softer center,
    negative = more aggressive center).
  - Steer curve 1 (one stick: center -> max steering) and Steer curve 2
    (counter-stick: max steering -> 100%), each -100 to +100 and set separately.
  - Max steering 50-100% for one stick fully deflected; full counter-steer
    (left forward + right back) always gives 100% steering.
- THRUST/STEERING curve plots with a live dot for the current stick state.
- Start size 1380x616, dark-styled scrollbar; scrolled content stays below the title
  and profile bar.
- Status bar at the bottom of the window with the copyright notice of the original
  authors (JaffaQuake and Mars) and "Modified by G-Mein (github.com/G-Mein)".
- Includes the upstream v1.98.3 change: Lever-only Output Negative checkbox for both
  twist inputs (Centered-mode behavior unchanged).

INPUT / OUTPUT NOTES
--------------------
- Use the selected backend (Auto, DirectInput, WinMM) to choose each twist device
  and axis. DirectInput exposes up to eight axes, WinMM up to six.
- Lever mode normally outputs 0% to +100%. The Lever-only Output Negative
  checkbox changes the range to 0% to -100%, enabling negative steering, pitch,
  camera, or keyboard-key output. Invert independently reverses lever travel
  (0% ↔ 100%) without changing polarity.
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
To install the Go build prerequisite on Windows x64, run Source\Setup-Go.cmd.
It uses winget to install or upgrade Go as needed; it does not install ViGEm components.

Native UI improvements: dark cards, six separate navigation views, profile-aware HOSAS/HOTAS
controls, separate rudder-pedal/on-foot cards, shared HOSAS max-throttle, max-steering and steer-curve (expo) settings, and a live
bidirectional forward/reverse thrust meter and response preview. Live Monitor updates avoid
redrawing the full window when stick axes move. THRUST/STEERING curve plots show the
current stick state as a live dot. Max steering applies to one stick fully deflected; full
counter-steer (left forward + right back) always gives 100% steering.

Live overlay: Live Monitor -> "Show overlay" minimizes the app and shows Left Stick, Right Stick
and Drive/Steer always on top in the top-right corner of the monitor. Click the red X to close it
and restore the app. Works over windowed/borderless games, not exclusive fullscreen.
