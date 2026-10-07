# WebTV Emulator: Complete Agent Handoff

Updated: 2026-10-07. This is the consolidated published handoff; workstation-specific paths are omitted. A replacement agent should be able to continue using this file alone, without the preceding chat. Older research documents and repository trace notes contain superseded conclusions; the corrections here take precedence.

## 1. Read this first

The original boot ROM now powers on through emulated infrared hardware and renders its authentic **Connecting to WebTV** page, update-check text, **Preparing to call**, and progress bar. The SDL frontend submitted that page; its fresh capture was visually inspected. The latest UART correction also let the original guest transmit its entire first Hayes configuration command.

**The current remaining boundary is modem command/response handling.** The guest expects an `OK` response to the first configuration, then a numeric `0` response to its next configuration before dialing. The local stage-4 modem model does not interpret normal Hayes commands or implement a real call lifecycle. Dialing, PPP, service login, and application operation beyond the boot update check are not verified.

Do not start over at the old logo/standby diagnosis. Do not fabricate a page, force guest RAM/events, inject a packet-queue pointer, spoof a printer adapter, or send an invented successful carrier response.

## 2. Repository, remote, and required local inputs

| Item | Location / value |
|---|---|
| Checkout | Repository root |
| Published handoff | `docs/AGENT_HANDOFF.md` |
| Branch | `main` |
| Canonical GitHub repository | `https://github.com/MorenoLand/Moreno.WebTV.git` |
| Existing origin | `https://github.com/Denveous/Moreno.WebTV.git`; GitHub redirects it |
| Published HEAD, checked for this handoff | `18e6aeb1246f15d9ac0e94c8b391fc851ada7b0d` |
| Latest source-changing milestone | `58d7efb` |
| Boot ROM | `roms/bootrom.brom`, 2,097,152 bytes |
| Optional EEPROM input | `roms/ptnvram.iic` |
| Preserved disk / flash input | User-supplied preserved disk/flash image, local and ignored |
| Signing instructions | the workstation signing runbook |
| Go toolchain | Go 1.21 or later on PATH |

Published HEAD and local HEAD matched when checked. All listed current milestones are GPG-signed and were pushed. ROMs, decoded binaries, captures, logs, and executables are intentionally ignored; they are not included in a fresh Git clone. Do not publish them.

The old checkout name `webtv-emulator-work` is obsolete. The local canonical research handoff and this published copy contain the same technical state; only workstation-specific paths differ.

## 3. Scope and working rules

- Use the named checkout and inputs. Do not search other drives, change supplied firmware, or broaden external access without permission.
- Use `go run` in development; no production executable was built during these passes. Do not blame a user-reported run on a stale separately compiled EXE.
- Keep generated output in the checkout's existing ignored `.tmp` and `captures` directories. Do not write Windows temp or the user's home.
- Use native tooling; Docker is not allowed on this workstation.
- Preserve user edits, comments, and existing style. Prefer the smallest complete, original-firmware/spec-backed change.
- Source edits and passing tests are not screen proof. Verify actual runtime behavior and pixels.
- Every commit must be factual and GPG-signed. Read the workstation signing runbook before committing; its wrapper fixes the Git-for-Windows/MSYS2 path problem. Do not disable signing, amend, add coauthors, or create empty commits.
- Do not modify CI/workflows or push unless currently authorized.
- Use subagents for bounded independent work when available. Avoid Ghidra in the main work when offline analysis suffices.
- Computer use was explicitly stopped during this investigation; obtain new authorization before using it.
- Do not use shell SSH/SFTP; remote work requires configured MCP tools.
- Keep communication terse and evidence-based. Implement authorized fixes directly; do not infer emotions or substitute generic troubleshooting.
- This handoff authorizes no new implementation, launch, publication, or external coordination by itself; the next agent must follow the user's current request.

## 4. Run and test commands

Go module requirement: Go 1.21 or later. Existing dependency: `github.com/ebitengine/purego v0.10.1`. SDL2 is resolved through `SDL2_DLL`, the existing project `.tools` package, then the system library path.

Interactive run, with automatic hardware Power input and experimental shortcuts disabled:

~~~powershell
$env:GOTMPDIR=(Resolve-Path '.tmp').Path
$env:GOCACHE=Join-Path $env:GOTMPDIR 'go-cache'
$env:WEBTV_VIDEO='1'
$env:WEBTV_POWER_AT_STEP='60000000'
$env:WEBTV_WTV_PORT_RESPONSE='none'
$env:WEBTV_COMPAT_SHIMS='0'
$env:WEBTV_SYNTH_CLOCK='0'
$env:WEBTV_SYNTHETIC_PPP='0'
$env:WEBTV_MODEM_STATUS=''
$env:WEBTV_MODEM_TCP=''
$env:WEBTV_MODEM_NO_CARRIER='0'
$env:WEBTV_CAPTURE_VIDEO='1'
foreach ($taskFlag in @('WEBTV_FORCE_MODEM_STATUS','WEBTV_PERSIST_MODEM_STATUS','WEBTV_MODEM_DEVICE_FIELDS','WEBTV_MODEM_QUEUE_PROBE','WEBTV_SKIP_NULL_CLEANUP')) { Remove-Item -LiteralPath "Env:$taskFlag" -ErrorAction SilentlyContinue }
Remove-Item Env:WEBTV_STEPS -ErrorAction SilentlyContinue
go run . "firmware.img"
~~~

Run these commands from the repository root and replace firmware.img with the actual preserved image path. F1 queues remote Power On. Automatic Power is one actual IR waveform, not a RAM patch. Its scheduling value uses the **main loop iteration count**, not `cpu.Steps` or ASIC cycles.

Without an explicit `WEBTV_STEPS`, interactive execution is unbounded. Headless execution is bounded by default. An explicitly bounded viewer run can leave the window visible after the CPU finishes; later keyboard input cannot progress a finished CPU loop.

Tests, from the same directory with the cache/temp settings above:

~~~powershell
go test ./... -count=1
~~~

Bounded viewer reproduction: set `WEBTV_STEPS='180000000'`. The latest outbound-command reproduction used `WEBTV_VIDEO='0'`, `WEBTV_STEPS='400000000'`, `WEBTV_POWER_AT_STEP='60000000'`, and `WEBTV_TRACE_MODEM_TX='1'`, keeping shortcuts disabled.

The calibration diagnostic at main iteration 5,009,835 is not a halt. Decompression runs afterward with little console output. A six-million-iteration check reached `PC=0x9FC030E4`. Ten environment lookups per instruction were removed from the hot loop; startup became substantially faster. Runtime remains machine-dependent.

## 5. Current verified state and evidence limits

Verified:

1. Original boot ROM executes, decompresses its browser, initializes resources, and enters standby.
2. Remote Power `0x2E` is decoded through the emulated IR FIFO and real guest interrupt/dispatch path.
3. Guest code sets startup flag `0x8000224E=1`; no forced gate write is required.
4. Guest selects the YUY2 framebuffer and paints the original Connecting page.
5. SDL submitted that page and produced a fresh, visually inspected capture after renderer presentation.
6. The original registration script performs a DTR wink and transmits its first complete Hayes command through its interrupt-driven software queue.
7. CPU, bus, IR, UART, video, and renderer regression tests pass.

Not verified / incomplete:

- Normal modem command interpreter, call setup/results, busy/no-dial-tone/no-carrier lifecycle, and correct carrier timing.
- PPP/WTVP peer, actual service connectivity/login, and complete application boot after the update check.
- Full keyboard/remote controls beyond the implemented Power input.
- Cycle-accurate UART transmission and a real transmit FIFO.
- Full video DMA clocks, CCNT advancement, queued-transfer rollover, POT-driven VSYNC completion, and graphics acceleration.
- Several audio, display/input, device, protection, and other ASIC functions.
- Physical ASIC revision behind the documented versus observed IR packing discrepancy.
- Renderer brightness/chroma qualification is a heuristic, not a guest repaint-completion marker.

The initial gold logo is decoded from `/ROM/Images/SplashLogo1.gif` but presented as a **host preview**. It alone is not guest boot proof. A RAM image is weaker evidence than a renderer submission; neither should be misrepresented as an OS screenshot of a physical display.

## 6. Architecture and source map

Target: WebTV Plus-style SOLO machine, R4640/MIPS III CPU, 8 MB RAM, boot ROM at `0xBFC00000`, physical firmware flash at `0x1F000000`, IDE image, EEPROM, SDL2 frontend, eventual WTVP connectivity.

| File | Role / changes |
|---|---|
| `main.go` | Boot/run loop, diagnostics, automatic Power, bounded versus interactive lifetime, cached trace configuration |
| `cpu.go` | MIPS/COP0/TLB execution; WAIT now includes the earliest IR deadline |
| `bus.go` | RAM/ROM/device mapping, absent external-adapter default, CPU-owned input queue, modem upload/UART/TCP bridge |
| `asic.go` | SOLO registers, IR FIFO/timing/IRQ, masked DEV status and IP2 routing, corrected video startup validity |
| `ir_remote.go` | Firmware-compatible three-frame SIRCS-style remote command encoding |
| `video_sdl.go` | SDL setup, F1, scanout qualification, immutable splash reference, submitted-frame captures |
| `video_frame.go` | Active DMA snapshot access; current and next buffers are distinct concepts |
| `ide.go` | Disk-image access and IDE device |
| `romfs_file.go` | ROM resource extraction/access used by preview and analysis |
| `modem_test.go` | Validated S-record firmware upload and SRAM/backdoor tests |
| `modem_uart_test.go` | MCR readback, DTR wink, CTS restoration, THRE priority/ack/mask and divisor tests |
| `modem_tcp.go`, `modem_tcp_test.go` | Optional serial TCP bridge, not PPP |
| `ppp.go`, `ppp_test.go` | Framing/probe helpers, not a complete peer |
| `ir_remote_test.go`, `ir_test.go` | Waveform, FIFO, interrupt, host-input race, and WAIT regression tests |
| `video_dma_test.go`, `video_sdl_test.go`, `bus_test.go`, `asic_test.go` | DMA, display selection/capture, adapter absence, and bus/interrupt regression coverage |
| `docs/BOOT_RENDERING_PROGRESS.md` | Evidence chronology; older parts are historical |
| `docs/WEBTV_HARDWARE_REFERENCE.md` | Hardware notes |
| `docs/AGENT_HANDOFF.md` | Published, workstation-path-free continuation snapshot |

Earlier core corrections remain important: overflow-safe RAM/ROM bounds, ERL KSEG2 physical mapping, aborting faulting TLB-miss loads/stores instead of writing bad values, delay-slot EPC/BD preservation, and missing REGIMM branch-likely/immediate-trap support that fixed image-manager initialization order.

The preserved app header reports load `0x804E0000` and ROMFS `0x80800000`. The build-length field excludes its first 12 bytes; the loader must copy `romLen+0x0C`. Data at `0x804E0000` is an executable header, not another framebuffer. Loaded image/header presence is not full application-boot proof.

## 7. Signed, published milestone history

All of the following current milestones are reachable from the checked published HEAD:

| Commit | Actual change |
|---|---|
| `4b07bcb` | Earlier real printer-adapter warning documentation, not normal Connecting proof |
| `94a8d19` | Corrected retained-timer and boot-event diagnosis |
| `431ae96` | Added verified remote frame encoding |
| `43ce4f8` | Timed IR input, F1, absent-adapter default, interactive lifetime |
| `23d099a` | Retained Power until IR receiver initialization |
| `8e5c5ce` | Admitted asynchronous input after elapsed WAIT intervals |
| `dd54654` | Enabled DEV IP2 routing, masked status, video NV/NVF startup |
| `30bb38f` | Changing scanout presentation and submitted-frame capture |
| `23a7de0` | Recorded authentic guest-memory Connecting evidence |
| `2c5b484` | Removed hot trace lookup overhead; preserved splash/capture identity |
| `778f8b0` | Verified fresh live viewer capture |
| `d1c2658` | Published continuation guide |
| `58d7efb` | MCR readback, idle THRE/TEMT, transmit-empty interrupt support |
| `706dfb6` | Recorded complete queued Hayes transmission and remaining response boundary |
| `18e6aeb` | Updated the published guide's current UART milestone |

Older signed passes include `c20b3ca` (initial video handoff), `9e48fd6` / `eff9c88` (no-carrier diagnostics), and earlier CPU/device fixes. Those are history, not proof that their original diagnoses were correct. The consolidation changes documentation only, not emulator code or firmware.

## 8. The diagnoses that were wrong

**Do not revive these as current blockers.**

- Physical RAM `0x588` is not an external modem status waiting for a peer. It is retained software timer state. `0x802C5B00` writes the `0x7661` tag and schedules two daily deadlines at object offsets `0x18C/0x190`, advancing expired values by 86,400 seconds. `0x802C5E84` emits event `0x2000` with selector 1 or 2. Selector 2 is not a missing startup event key 2.
- The list at `0x800017A0` is an external-adapter identification list: `{next,type,probeAddress}`. The seven-slot probe begins at `0xA4FFFFF0`. Type 2 requests automatic power-on and is also tested by the printer-warning branch. The old default byte 2/word 0 falsely advertised this adapter, not the built-in modem.
- The bootloader deliberately preserves physical `0x400..0x7FF` during its sentinel fill. Its lack of writes to `0x588` in standby does not imply missing external hardware data.
- Earlier synthetic PPP/no-Power tests observed null receive queue `0x80001A3C` and only the first `0x7E`. They do not establish that PPP or a peer is needed to show the boot page.
- The old calibration log, black 75M image, or an intermediate PC inside `0x802BA40C` did not prove a permanent stall. That bitmap search is bounded; later execution progressed into copying and painting.
- Missing live captures initially resulted from splash/presented slice aliasing. It suppressed the capture comparison, not necessarily SDL rendering itself.

## 9. Actual Power and infrared path

- Command `0x2E` maps through `0x8032BAF0` to `0x8020DEE8`, producing key `0x8006` while off.
- `0x802C6C04` dispatches that key to `0x802C7464(0)`; guest store `0x802C74BC` sets startup flag `0x8000224E`.
- Input object/vtable path: `0x800025D4 -> 0x80341484 -> 0x802C6BCC -> 0x802B4930 -> action 0x1D -> 0x802C6D48 -> 0x802C6C04`.
- `0x2240` is busy; its path can consume input instead of normal dispatch. `0x224E` is startup/on.
- The encoder produces `0x79D2E`, LSB first, with 2,400 us header, 600 us spaces/zero pulses, 1,200 us one pulses, and three identical 45 ms-period frames. Power's inter-frame gap is 11,400 us.
- Firmware requires two matching 20-bit records. Three repeated frames produce 127 transition words and allow the threshold-eight IRQ to deliver both records at word 88. Tests assert this exact point without a fake tail IRQ or lowered threshold.
- Guest register setup: sample `A4004020`, reject `A4004024`, control `A400402C=0x3083`. Measured sample interval was `0x4DD=1245` cycles; decoded duration units are 15 us.
- Observed `A4004028` packing: duration bits 10:0, level bit 11, availability/count bits 15:12. Archived SOLO1 page 5-82 instead puts level at 15 and count at 14:11. Preserve this discrepancy explicitly; the firmware ABI is verified, independent hardware revision is not.
- IRQ ID `0x1D` uses numeric mask `0x04` (bit 2, not bit 4). `0074` reads enabled DEV status, `0078` raw status/set, `0174` clears, `007C` enables, `017C` disables.
- Firmware enables DEV mask 4 while `BusIntEnable=8`. Requiring parent mask `0x10` prevented IP2; corrected source routing matches the firmware and existing RIO/VID approach.
- FIFO depth 16, threshold eight, timed arrivals, sticky status. Reset is modeled as a write pulse; reserved control bits are masked.
- CPU WAIT chooses the earliest pending IR arrival. Host input is admitted after an elapsed Tick and retained until receiver readiness, avoiding both startup loss and a WAIT/FIFO overflow race.
- Decoder flags: `0x80001308`. Raw ring indices: `0x13B8/13BC`. Decoded indices: `0x13A8/13AC`.

SIRCS timing reference: [maintained Arduino-IRremote implementation](https://github.com/Arduino-IRremote/Arduino-IRremote/blob/master/src/ir_Sony.hpp); original firmware remains authority for the WebTV identifier/command.

## 10. Video and SDL behavior

The actual observed frame was `CSTART=NSTART=0x8000B740`, `CSIZE=NSIZE=470400=0x72D80`, YUY2 `560x420`, DMA control `0x1F`, active transfer true.

SOLO1 Table 5-73, printed pages 5-73..5-77, defines:
- DMAEN bit 2, NV bit 1, NVF bit 0.
- Interlace bit 3; VSYNCRESET bit 4.
- Idle enabled DMA starts when NV is set, without waiting for VSYNC/write-back.
- Start consumes NV unless NVF holds it. Clearing NV does not stop the current transfer.
- DMAEN falling edge flushes/resets the current transaction.

The model now tracks active state separately from address/NV and continues snapshots after consumed NV. Do not claim implemented transfer clocks, CCNT advancement, rollover, or VSYNC completion.

`CityRoad.cel` repeats six frames at `PERIOD=3` on the guest 60 Hz tick clock: nominal 50 ms. The first frame uses `PERIOD=20`; `ConnectingTo.cel` uses periods 10 and 100. A 150 ms unchanged-pixel requirement could hide animation. The viewer now qualifies continuously presentable scanout while allowing pixel changes.

The initial presented slice must be independent of the immutable splash. Sharing it overwrote the reference and made the capture's comparison permanently equal. Captures now occur after renderer presentation and update once a second with submitted data. The brightness/chroma filter can still reject frames; it is not an authoritative paint-completion signal.

## 11. Modem / UART: current pass and next boundary

The modem is the external Device-0 aperture at `BE000000` / physical `1E000000`, four-byte-spaced registers. Do not apply the internal SOLO sucUnit UART's CTS-inversion erratum to it.

Offsets: data/divisor-low `00`, IER/divisor-high `04`, IIR/FCR `08`, LCR `0C`, MCR `10`, LSR `14`, MSR `18`, scratch `1C`. Full scratch/FIFO/UART timing parity is not claimed.

The downloader recognizes `AT**`, consumes valid S3 records, validates the S7 terminator, stores 21 bytes from three records at modem addresses `0x200..0x214` inclusive, and acknowledges completion. SRAM/backdoor support uses the normal Device-0 aperture. Stage 4 means firmware-download completion in this model; it is not independent online/carrier proof.

The native `builtin_winkdtr` at `0x8024C46C` reaches `0x8020C450`, which reads MCR and modifies only DTR. Intended sequence: `0B -> 0A -> 0B`. Previously, MCR readback returned zero, losing RTS/OUT2 and preventing CTS restoration.

UART changes in `58d7efb`:
- MCR offset `10` reads back its programmed value.
- Empty instantaneous transmission reports LSR THRE+TEMT, `0x60`, plus RX-ready as applicable.
- TX-empty pending state reaches RIO when IER bit 1 is enabled.
- IIR identifies RX first, then TX `02`, then modem-status `00`.
- Reading TX IIR acknowledges that UART source without clearing sticky SOLO RIO.
- A subsequent completed data write or enable transition rearms TX; divisor writes remain isolated.
- No fabricated AT response or guest memory/event change was added.

The low and high DTR portions each wait 30 ticks through `0x8024BBC0`. Tick accessor `0x8030AD78` reads `0x80002730`; callback `0x8022C2CC` increments it and acknowledges `A40090A8=0x30` and `A4000118=1`. The 220M cutoff was only about 24 tick periods after DTR low, so absence of DTR high there was normal.

Actual queued transmit:
- `sendstr -> 0x8024C3F8 -> 0x80238CAC -> 0x8020C590`.
- Buffer pointer `0x800010C4`, producer `0x10D4`, consumer `0x10D8`; ring size 4096.
- It enables TX interrupts through `BE000004 | 2`.
- IIR dispatcher `0x8022168C`, table `0x8022183C`, IIR 02 slot `0x80221844 -> 0x802216F4`.
- TX drain requires MSR CTS `0x10`; otherwise it refuses to advance.
- Data writes occur at `0x80221750`, up to 16 bytes per pass, and empty software queue disables TX IRQ.

The 400M run transmitted exactly two downloader LF bytes plus:
~~~text
ATS38=0S30=180S95=36S11=60&D2V1E0L3&Q5&K3\r
~~~
The full reconstructed 44-byte stream was asserted against the original ROM literal.

Registration script ordering:
1. `enablemodem()`, DTR wink, `setflowcontrol(3)`, `setbaud(57600)`.
2. First configuration above, waiting for `OK`.
3. Second configuration beginning `ATV0`, with phone-dependent M/tone/timing options, waiting for numeric `0`.
4. Only after those succeed: `Dialing toll free`; send `ATDP` or `ATDT`, prefixes/number, then `;\r`.
5. A successful first dial result leads to `ATD\r` and `Waiting for answer`.

Relevant raw boot-ROM offsets: first literal `D0E80`; Preparing to call `D0FA9`; enablemodem `D1583`; wink `D1595`; first ConfigureModem `D15D5`; second `D1679`; dial status `D171C`; dial-string sends `D183B..D1871`; later ATD `D1A62`. Native `read32` is `0x8020C2C0`, `write32` is `0x8020C2C8`; `0x8020C5E8` is inside enqueue, not a TX function entry.

Remaining modem issues:
- Stage-4 normal Hayes lines are accumulated/cleared without local interpretation or `OK/ERROR` production.
- The TCP bridge forwards serial bytes, not PPP or a complete modem command backend.
- MCR readiness still schedules CTS/DSR/DCD together; carrier before dialing is an unresolved modeling problem.
- Diagnostic no-carrier still follows MCR readiness rather than a proven dial attempt.
- TX is instantaneous, send failure/link loss handling and real occupancy/timing/FIFO behavior are incomplete.
- Do not send canned successful carrier text merely to advance the screen.

UART reference: [TI PC16550D manufacturer datasheet, hosted by distributor](https://www.mouser.com/datasheet/2/405/pc16550d-443503.pdf), sections 8.6.3..8.6.7.

## 12. Artifacts a replacement agent can inspect

All paths below are relative to the checkout; files were checked to exist during consolidation.

| Artifact | What it establishes |
|---|---|
| `captures/live_video_frame_yuy2.png` | Fresh, visually inspected renderer-submission Connecting page |
| `.tmp/viewer-fix-20261007.log` | 180M run, physical Power, capture log with CSTART B740 |
| `.tmp/calibration-check-20261007.log` | Execution beyond the calibration diagnostic at 6M |
| `.tmp/uart-ready-long-20261007.log` | 400M run, full Hayes TX; final reads 599, writes 257, TX bytes 44, MCR 0B, MSR B0 |
| `.tmp/uart-ready-pass-20261007.log` | 220M cutoff during first DTR-low wait, not a frozen clock |
| `.tmp/ir-power-ip2-20261007.log` | Earlier 75M Power/framebuffer selection; final pixels still black |
| `.tmp/guest-cstart-20261007.png` | Earlier live RAM image of authentic page, not OS screen capture |
| `.tmp/dumps/bootrom-browser.bin` | Decoded browser executable image |
| `.tmp/dumps/bootrom-browser-data.bin` | Decoded initialized data |

Do not assume a fresh clone includes these ignored files. Do not confuse a stale September capture with the fresh current evidence. Runtime process IDs are transient; identify the current instance by its exact command/input before any diagnostic. Never terminate unrelated processes.

`cpu.Steps` includes WAIT fast-forward cycles; the main `step` counter counts execution calls. Large CPU/ASIC numbers do not imply a bounded run exceeded its iteration limit.

Historical synthetic PPP tests observed null queue globals `0x1A3C/1A40`, helper `0x80229F3C`, parser `0x80243480`, UART handler `0x802217D8`, and dormant consumer `0x80243F50`. Preserve those facts for later network work, but redo them after actual Power/normal modem setup rather than treating the old standby inference as a current blocker.

## 13. Reverse engineering: exact binary mapping and traps

Raw boot ROM is already big-endian. Raw bootloader runtime base is `0x9FC00000`; CPU reset view is `0xBFC00000`.

Compressed browser source: ROM file offset `0x32EC`. Expanded output: `0x1C0000` bytes at `0x80200000`. Decoded file SHA-256:
~~~text
9dd56fc13893d57dde53e84a19bfc50b1dc4f6619e1069b7995ceabb26597505
~~~
It matched all 9,472 bytes of a saved runtime code dump. Raw boot ROM import alone cannot directly expose these compressed runtime functions.

Initialized data decompresses from `0x8034ACB0` into `0x80001000..0x80003BAF` (11,184 bytes). BSS is `0x80003BB0..0x8000B19F`. The browser executable file is 1,835,008 bytes.

If decoded artifacts are missing, reproduce the ROM's LZSS expansion rather than importing random byte ranges: read flag bits LSB first, bit 1 means a literal byte, and bit 0 means a two-byte backreference. For bytes b0/b1, distance is `1 + (b0 | ((b1 & 0xF0) << 4))` and length is `3 + (b1 & 0x0F)`; copy from already-produced output, including overlap. Start at raw-ROM offset `0x32EC`, stop at `0x1C0000` output bytes, and verify the SHA-256 above. The existing pure decoder uses `flags_start=0`; only its pure expansion method was used, not temp-writing extraction helpers.

Ghidra settings: raw binary, `MIPS:BE:64:64-32addr`, compiler `o32`, base `0x80200000` **before analysis**. Zero-based analysis followed by rebasing shifted old call references to wrong `0x804...` targets; verify J/JAL from actual opcode bits, not those cached references.

Bridge caveats: `run_script` returned 404; some endpoints ignored the requested program and used the active one. Concurrent projects caused ARM bytes to be returned for a MIPS request. Confirm identity and bytes before any inference/mutation. Use offline/subagent analysis when practical; do not operate the GUI without current authorization.

Tools/reference files:
- `py` with `sys.path.insert(0,'.tmp/python-deps')`: locally installed Capstone, MIPS64 big-endian mode.
- `.tmp/rommy-ref/lib/lzss.py`: pure decoder. Full extraction helpers can use Windows temp; avoid those paths.
- `.tmp/SOLO1_ASIC_Spec.pdf`: primary hardware manual.
- `.tmp/SOLO1_ASIC_Spec_djvu.txt`: OCR, incomplete tables.
- `.tmp/solo-rio-text.txt`: Device-0 modem/aperture reference extract.
- Poppler is available. Inspect PDF bit tables visually; do not infer missing bit positions from OCR.

## 14. Recommended next work, in order

1. Reproduce current Connecting page and the complete first configuration TX on the current checkout with shortcuts disabled.
2. Implement or connect an evidenced normal modem command/call backend. Preserve the actual configuration order and the expected `OK`, then `0` responses. Establish supported syntax/status from original modem/firmware references; do not special-case this one executable/string.
3. Derive DCD and failure results from real call state instead of DTR/RTS readiness; verify no-phone-line behavior after a real dial attempt.
4. Recheck incoming parser/queue creation only after normal post-Power configuration completes. Do not patch the old null queue.
5. Advance into PPP/WTVP only when original guest behavior and actual wire bytes establish the boundary.
6. Independently close hardware gaps, including DMA transaction timing/rollover/VSYNC, as evidenced; do not degrade correctness merely to pass a test or link/build.
7. For each authorized milestone: edit narrowly, run regression tests, exercise `go run`, inspect runtime artifacts, sign a factual commit, and publish only when authorized.

Keep this document current by replacing obsolete state, not appending contradictory current-blocker sections. No private conversation context or another handoff is required to understand the present boundary.
