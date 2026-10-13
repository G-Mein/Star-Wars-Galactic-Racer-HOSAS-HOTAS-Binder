# Changelog

## v2.0 - 2026-10-13
- Release v2.0 (modified by G-Mein, https://github.com/G-Mein): version shown in title and status bar from one appVersion constant; release files BUILD_VALIDATION/INSTALLER_AUDIT/RELEASE_NOTES/SHA256 v2.0; Build-Local.cmd added to Source; installer copies LICENSE and CHANGELOG.md.
- Port upstream v1.98.3: Lever-only Output Negative checkbox per twist (0..-100%), config key output_negative (default false), axis_math_test.go.
- Hide sidebar navigation entries that would overlap the status bar in short windows.
- Remove stale WSL build and UI mockup references from the docs (files are not shipped).

## Pre-2.0 development
- Add dark native frame: draggable title area, resize edges, caption controls; move profile selector above content.
- Preserve device/profile/HOTAS/twist/on-foot settings during legacy migration when axes are serialized as names.
- Remove Go vet DirectInput `unsafe.Pointer` warnings; retain native COM/input behavior.
- Restyle native UI with full dark cards, persistent sidebar, profile-aware HOSAS/HOTAS controls, independent per-stick thresholds/thrust curves; preserve existing functions.
- Add native sidebar page navigation; separate rudder/pedals and on-foot cards.
- Assign every control to one sidebar page/section, incl. overlapping cards; show signed forward/reverse thrust; redraw only affected visible Live Monitor meter regions (no erase, child redraw, or whole-window repaint).
- Keep device/twist/pedal/on-foot cards clear of fixed chrome and titles; fit twist, pedal, and on-foot controls inside their cards; size button-mappings card/scroll range to reach all controls; keep custom chrome above content; use explicit page ownership without geometry-based ownership inference. Add layout, page-ownership, scroll, and meter regression tests.
- Add independent HOSAS left/right forward-thrust response settings; add UI concept mockup.
- Preserve learned on-foot WASD assignments when normalizing configuration binding keys.
- Add a double-click Go setup script for Windows x64 development.
- Document: ViGEm runtime components are not part of Go setup.
- Add WSL setup and cross-build script for Windows x64 builds.
- Show a native in-client vertical scrollbar on overflowing pages, including Buttons in a short window; keep it below fixed profile chrome and synchronized with wheel and thumb scrolling.
- Route each full-throttle slider to its own setting; keep moving Live Monitor dots above axis-line controls and redraw after movement/visibility changes.
- Protect Live Monitor stick markers from axis-line repaints with sibling clipping and centralized redraw; in forward-only HOSAS input, use the strongest scaled left/right stick for total forward thrust independently of steering; label the checkbox Hybrid reverse scrubbing and retain its mixed forward/reverse behavior.
- Replace per-stick full-throttle thresholds, strongest-stick mixing, and the Hybrid reverse scrubbing checkbox with one shared HOSAS "Max throttle" slider (10-100, default 100) in the Driving Response card below Deadzone: drive = (left + right) x max throttle, linear and symmetric; old settings start at 100.
- Add Live Monitor "Show overlay": always-on-top, non-activating overlay at the top right of the monitor with Left Stick, Right Stick and Drive/Steer (also during mapping); X closes it and restores the app.
- Add HOSAS "Max steering" (10-100, default 100) and "Steer curve" (expo 0-100, default 0 = linear) edit+slider settings in Driving Response: left full + right centered steers at max steering; curve softens the center, full deflection unchanged. Preview shows separate THRUST/STEERING plots; overlay header shows T/S/C values.
- HOSAS steering counter-steer: Max steering applies to one fully deflected stick; opposing sticks raise steering up to 100% at full counter-steer; Steer curve spans the whole travel through the Max steering point; Max steering range is now 50-100 (lower values are raised to 50). README credits G-Mein (https://github.com/G-Mein) with a GPL modification notice listing the additions; main.go carries a modifications copyright line. Throttle curve and Steer curve now range -100 to 100 (negative = more aggressive center). Max throttle now goes up to 200: above 100 the stick sum is amplified so full thrust stays while steering with the other stick back. Steer curve split into Steer curve 1 (one-stick segment up to Max steering) and Steer curve 2 (counter-stick segment up to 100%), each -100..100; new config key steer_expo2 (default 0); overlay/mode text shows S<max>/<curve1>/<curve2>.
- Start window at 1380x616; replace the system scrollbar with a dark owner-drawn scrollbar (thumb drag, track paging, wheel); clip scrolled controls and cards to the content area so they never overlap the title bar or profile bar.
- Show the current stick state as a live dot on the THRUST/STEERING curve plots (steering plot covers counter-steer range); plots redraw flicker-free only when input changes.
- Fix scrolled Buttons page painting over title/profile bar: BUTTON/STATIC paint DCs (CS_PARENTDC) ignore window regions -> clip paint DC to cached child clip; clip before move, move with SWP_NOCOPYBITS; set regions without synchronous redraw and keep clip cache consistent on SetWindowRgn failure; owner-draw restores the WM_DRAWITEM DC (SaveDC/RestoreDC). Guard tiny-client scrollbar bounds and NaN curve plot points; add scrollbar/clip/plot regression tests; remove dead code.
- Add bottom status bar (full width, app dark design) with copyright notice "© 2026 JaffaQuake and Mars · Modified by G-Mein (github.com/G-Mein) · GPL-3.0-or-later" and version; content, scrollbars and sidebar end above it; bottom resize edge still works.
