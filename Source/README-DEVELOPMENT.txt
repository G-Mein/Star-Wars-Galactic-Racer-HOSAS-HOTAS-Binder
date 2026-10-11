GALACTIC RACER HOSAS / HOTAS BRIDGE - LOCAL DEVELOPMENT (v1.96)

The application is written in Go and uses native Windows APIs directly.

QUICK BUILD
1. Install Go for Windows.
2. Edit main.go and save it.
3. Close any running GalacticRacerHOSAS.exe.
4. Double-click Build-Local.cmd.
5. Run the newly built GalacticRacerHOSAS.exe in this Source folder.

Build-Local.cmd searches PATH and common Go install locations, builds a Windows GUI executable,
and copies ViGEmClient.dll beside the dev EXE when it can find the installed/release copy.

v1.96 INPUT ARCHITECTURE
Physical flight axes can use:
  Auto -> DirectInput first, with WinMM compatibility available.
  DirectInput -> broader X/Y/Z/Rx/Ry/Rz/Slider 1/Slider 2 path.
  WinMM -> legacy six-axis compatibility path.

Learned buttons/hats are backend-aware:
  DirectInput -> device instance GUID + up to 128 buttons + four POV hats.
  WinMM -> original joystick ID + 32-button/POV path.
  Auto -> watches both and prefers DirectInput if the same press is visible to both.
Existing bindings without a backend field are treated as WinMM so old configs keep working.

HOTAS BRAKE THRESHOLD
HOTAS mode maps the throttle to RT above the configured threshold. At/below the threshold, RT is
released and LT/brake is held. Default is 5%; config range is 0-50%.

HidHide integration is optional. The app can register this executable on the allowlist and open
HidHide Configuration Client, but it deliberately does not auto-hide devices.

UI
The Win32 layout remains fixed-size inside a resizable/scrollable main window. v1.96 renders hover
help and selector option lists as children of the main window so they remain visible in window-only
screen capture instead of appearing as separate popup windows.
