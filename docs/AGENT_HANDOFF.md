# WebTV Emulator Agent Handoff

Updated: 2026-10-07. This is the current continuation guide; older trace conclusions are not authority when they conflict with the verified results below.

## Outcome and remaining scope

The real boot ROM now receives Power through the emulated IR hardware and renders its authentic Connecting to WebTV page, update-check message, Preparing to call label, and progress bar. A fresh SDL submission capture was visually inspected, not just a RAM dump or build result. Modem dialing, PPP, service connectivity, application boot beyond the update check, complete keyboard support, and complete ASIC emulation are NOT verified.

The gold logo displayed immediately on launch is decoded from the ROM but presented by the host as a preview. It alone is not guest boot proof. Do not generate replacement Connecting pages, patch guest RAM/events, force the startup gate, or advertise a fake adapter to get visible output.

## Repository and publication

- Repository: MorenoLand/Moreno.WebTV; branch main.
- The existing origin URL names Denveous/Moreno.WebTV and GitHub redirects it to MorenoLand/Moreno.WebTV.
- Verified boot/viewer milestone: 778f8b0; latest UART source milestone: 58d7efb. The completed continuation pass and current response boundary are recorded below.
- Commits must be GPG-signed. Use the workstation signing runbook, never disable signing, amend, or invent empty progress commits.
- Do not publish ROMs, disk images, captures, decoded binaries, or other ROM-derived artifacts. They remain ignored.
- Do not modify CI/workflows unless explicitly requested. Preserve other agents' and the user's changes.

## Inputs, toolchain, and execution

The module requires Go 1.21 or later, with github.com/ebitengine/purego v0.10.1 for SDL2 bindings. Use go run during development, not a separately built executable. SDL2 is loaded through SDL2_DLL, the project's existing .tools package, or the system library path.

Required local ROM: roms/bootrom.brom, 2,097,152 bytes. Optional EEPROM: roms/ptnvram.iic. Supply the user's preserved disk/flash image as the command-line argument. The target is an 8 MB RAM, R4640/MIPS III, SOLO-based WebTV Plus-style machine.

From the repository directory, use PowerShell:

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
Remove-Item Env:WEBTV_STEPS -ErrorAction SilentlyContinue
go run . "firmware.img"
~~~

Replace firmware.img with the actual supplied image. Keep generated output in the project's existing ignored directories. Interactive execution is unbounded unless WEBTV_STEPS is supplied; headless mode defaults to a bounded run. F1 queues remote Power On. WEBTV_POWER_AT_STEP schedules one real hardware input using the main loop iteration counter, NOT cpu.Steps or ASIC cycles.

Tests:

~~~powershell
go test ./... -count=1
~~~

WEBTV_VIDEO=0 disables SDL. WEBTV_STEPS=180000000 reproduces the bounded verification. Do not mistake the last calibration diagnostic near iteration 5,009,835 for a halt: the next long phase is ROM decompression. A six-million-step check reached PC 0x9FC030E4 in the decompressor. Removing ten repeated environment reads per instruction materially reduced startup overhead; do not promise an exact runtime on another machine.

## File map and implemented changes

- main.go: boot/run loop, bounded headless versus unbounded interactive execution, scheduled hardware Power, IR state diagnostics, trace configuration caching. Nine hot-loop video environment reads now reuse bus.TraceVideoASIC; the gate-producer flag is cached once.
- bus.go: default expansion port is absent, not a fabricated built-in Plus device. Host remote input travels through a buffered channel and is admitted on the CPU thread after elapsed Tick time; it is retained until the IR receiver is enabled and sampled.
- ir_remote.go: genuine SIRCS-style remote waveform generation. Power command 0x2E uses the firmware's 20-bit value 0x79D2E, LSB first, three identical frames, and 45 ms start-to-start spacing.
- asic.go: timed IR transition FIFO/registers, device interrupt masking/acknowledgment/routing, corrected video DMA startup validity and active transaction state.
- cpu.go: WAIT chooses the next pending IR transition deadline rather than jumping across the waveform and overflowing its FIFO.
- video_sdl.go: F1 input, continuous-valid-frame qualification rather than requiring identical pixels, and periodic capture of data actually submitted to the renderer. The presented buffer is an independent copy of the splash; otherwise presentation overwrote the comparison reference and suppressed every capture.
- video_frame.go: existing active-DMA snapshot access; do not conflate current and queued next buffer registers.
- ir_remote_test.go, ir_test.go, video_dma_test.go, video_sdl_test.go, bus_test.go: waveform, deadline, FIFO, masking, asynchronous input, adapter absence, DMA control, animation, and splash-reference regression coverage.
- docs/BOOT_RENDERING_PROGRESS.md: detailed verified evidence and older historical trace notes.
- README.md: current run/input behavior and explicitly limited verification claims.

## Signed milestone history

| Commit | Actual change |
|---|---|
| 94a8d19 | Corrected the retained-timer and boot-event diagnosis |
| 431ae96 | Added firmware-verified remote frame encoding |
| 43ce4f8 | Added timed SOLO IR input, F1, absent-adapter default, and interactive lifetime |
| 23d099a | Retained Power until receiver initialization |
| 8e5c5ce | Admitted asynchronous input after elapsed WAIT intervals |
| dd54654 | Corrected enabled DEV IP2 routing, masked status, and video NV/NVF startup |
| 30bb38f | Presented changing scanout and captured submitted frames |
| 23a7de0 | Recorded authentic guest-memory Connecting evidence |
| 2c5b484 | Removed hot trace lookup overhead and fixed splash/capture aliasing |
| 778f8b0 | Recorded fresh live viewer submission verification |

## Disproved diagnoses: do not repeat them

Physical RAM 0x588 is NOT an external modem status waiting for a peer. It belongs to retained software timer state. Function 0x802C5B00 writes the 0x7661 signature and schedules two daily deadlines at object offsets 0x18C and 0x190, adding 86,400 seconds. Function 0x802C5E84 emits event code 0x2000 with selector 1 or 2; selector 2 is NOT boot event key 2.

The list rooted at 0x800017A0 is an external-adapter identification list, with nodes {next, type, probeAddress}. The seven-slot probe starts at 0xA4FFFFF0. Type 2 selects automatic startup and the printer-warning check. The old default byte 2/word 0 advertised a printer adapter and was not a coherent normal modem-presence protocol. The default now leaves it unmapped; old explicit diagnostics remain opt-in.

Earlier no-Power runs and synthetic PPP probes were made while the guest was in standby. A null packet queue from those runs is not evidence that PPP is required to show the boot page.

## Actual Power path and hardware details

- Remote command 0x2E maps through 0x8032BAF0 to 0x8020DEE8 and emits input code 0x8006 while off.
- Standby handler 0x802C6C04 accepts 0x8006 and calls 0x802C7464(0). Guest instruction 0x802C74BC sets 0x8000224E to 1.
- The guest initializes IR sample timing at 0xA4004020, reject timing at 0xA4004024, and control at 0xA400402C=0x3083. The measured run used sample interval 0x4DD (1,245 ASIC cycles); count units are 15 microseconds.
- The observed firmware ABI uses duration bits 10:0, level bit 11, and availability/count bits 15:12 in 0xA4004028. The archived SOLO1 manual instead places level at bit 15 and count at 14:11. That revision/document discrepancy remains unresolved; do not claim independent hardware verification of the observed layout.
- Device IRQ 0x1D uses bit 4 (numeric mask 0x04). 0x0074 reads enabled DEV status, 0x0078 raw status/set, 0x0174 clears status, 0x007C enables, and 0x017C disables.
- The firmware enables this DEV source while BusIntEnable remains 8. Enabled DEV status must reach CPU IP2 through its source path, like existing RIO/VID routing; requiring parent mask 0x10 blocked the first runtime test.
- IR FIFO depth is 16; interrupts qualify at the programmed threshold of eight. Status remains sticky until acknowledgment. Scheduled transitions are released over emulated time, not dumped into the FIFO.
- A two-frame 85-word signal leaves a five-word tail below the threshold. Three proper repeated frames deliver both required records by word 88 without lowering the threshold or inventing idle-overflow interrupts. Tests assert that exact point.
- Decoder-enable flags are at 0x80001308. Raw ring indices are 0x13B8/13BC, decoded indices 0x13A8/13AC. 0x224E is startup/on; 0x2240 is busy and can consume an input before ordinary dispatch.

## Video details and remaining hardware gaps

SOLO1 Table 5-73 and printed pages 5-73 through 5-77 establish DMAEN bit 2, NV bit 1, NVF bit 0, interlace bit 3, and VSYNCRESET bit 4. Idle DMA starts immediately when enabled and next-valid, consuming NV unless NVF holds it. Clearing NV does not stop an active transfer; clearing DMAEN flushes it. Active state no longer depends on a nonzero address or an uncleared NV.

Transfer clocks, CCNT advancement, transaction completion/rollover, POT-driven VSYNC completion, graphics acceleration, and portions of other ASIC units remain unmodeled. Do not claim full video DMA parity.

CityRoad.cel's six-frame repeating portion uses PERIOD=3 on a 60 Hz guest tick clock (nominal 50 ms). Waiting for 150 ms of unchanged bytes could indefinitely hide animation. The viewer now qualifies continuously presentable scanout while allowing byte changes. Its brightness/chroma filtering remains a heuristic, not a guest repaint-completion marker.

## Verified artifacts and observations

- .tmp/calibration-check-20261007.log: run advances beyond calibration to six million iterations.
- .tmp/viewer-fix-20261007.log: completed 180M verification, physical Power input, startup flag 1, and Captured presented video frame VID_CSTART=0x8000B740.
- captures/live_video_frame_yuy2.png: fresh renderer-submission capture of the real Connecting page, visually inspected.
- .tmp/guest-cstart-20261007.png: earlier read-only live RAM capture of the same authentic page; it was not an OS screenshot.
- .tmp/ir-power-ip2-20261007.log: earlier 75M run proved Power and selected the framebuffer, but its final image was still black. More execution was required for page painting.
- The active and next addresses were both 0x8000B740, sizes both 470,400 bytes (560x420 YUY2), control 0x1F, active DMA true.
- CPU, FIFO, interrupt, input, video, and renderer tests passed. No production executable was built.
- cpu.Steps includes WAIT fast-forward cycles; main step counts calls. Large ASIC/CPU trace numbers do not mean WEBTV_STEPS was ignored.

## Reverse-engineering assets and Ghidra traps

The raw boot ROM is already big-endian. Its compressed browser starts at file offset 0x32EC and expands to 0x1C0000 bytes at 0x80200000. Decoded image: .tmp/dumps/bootrom-browser.bin, SHA-256 9dd56fc13893d57dde53e84a19bfc50b1dc4f6619e1069b7995ceabb26597505. It matched all 9,472 bytes of a saved runtime code dump.

Initialized data decompresses from browser address 0x8034ACB0 into 0x80001000..0x80003BAF; BSS is 0x80003BB0..0x8000B19F. Data image: .tmp/dumps/bootrom-browser-data.bin.

For Ghidra, import the decoded browser as raw binary with MIPS:BE:64:64-32addr, compiler o32, base 0x80200000 BEFORE analysis. Importing at zero and rebasing after analysis shifted old J/JAL references incorrectly to 0x804...; verify targets against actual instruction bits. The raw bootloader belongs at 0x9FC00000.

The bridge's run_script endpoint returned 404, and some endpoints ignored program arguments in favor of the active program. Another concurrent project caused ARM disassembly to be returned for the requested MIPS program. Do not mutate a program without confirming identity. Prefer read-only subagent/offline work; no computer use unless newly authorized.

Capstone was installed locally under .tmp/python-deps and used with py. Existing pure LZSS decoder is .tmp/rommy-ref/lib/lzss.py; avoid full extraction routines that use Windows temp. The SOLO PDF is .tmp/SOLO1_ASIC_Spec.pdf; the text OCR omits several bit tables, so visually inspect relevant pages with the PDF skill. Poppler is available.

## Current next pass

Trace the actual post-Power Preparing to call state and first outbound Hayes transaction. Compare the guest's proven expected UART sequence with the current stage-4 modem model before changing it. The current default model's carrier assertion and early no-carrier diagnostic are not proof of a real dial sequence. WEBTV_MODEM_TCP forwards serial traffic, not a PPP implementation. Preserve UART/RIO sticky acknowledgment behavior.

Do not revive the obsolete timer signature or adapter-type theories. Do not force an ATD, guest event, packet queue pointer, clock, or page. Implement one original-firmware/spec-backed device correction at a time; verify through go run and real guest pixels, then sign and publish meaningful milestones when authorized.

## Completed UART continuation pass (2026-10-07)

Signed source milestone: 58d7efb, following published handoff d1c2658.

The first post-Power modem command now reaches the hardware TX register through the original guest ISR, not an injected string. The 400M-step run recorded exactly two downloader LF bytes followed by:

~~~text
ATS38=0S30=180S95=36S11=60&D2V1E0L3&Q5&K3\r
~~~

The reconstructed trace was asserted byte-for-byte against the original ROM command. Evidence is .tmp/uart-ready-long-20261007.log; final counters were 44 transmitted data bytes, MCR 0x0B, and MSR 0xB0. The earlier 220M run stopped during the legitimate first DTR-low delay, not a frozen clock.

Changes: bus.go now reads MCR back at offset 0x10, reports both THRE and TEMT when the existing instantaneous transmitter is empty, latches transmit-empty interrupts, identifies TX as IIR 0x02 behind RX and ahead of modem status, acknowledges that UART source on IIR read, and preserves sticky SOLO RIO status until its separate acknowledgment. IER enable and subsequent completed writes rearm TX. Tests in modem_uart_test.go cover DTR read-modify-write, CTS restoration, TX acknowledgment/priority/masking, and divisor-latch isolation. The old idle-LSR test now expects 0x60. Timing and a real TX FIFO remain outside this correction.

[TI's PC16550D register specification](https://www.mouser.com/datasheet/2/405/pc16550d-443503.pdf), sections 8.6.3 through 8.6.7, supplies the UART status and interrupt semantics; this manufacturer document is hosted by its distributor.

Original firmware detail: native builtin_winkdtr is 0x8024C46C. Its wrapper reaches 0x8020C450, which reads BE000010 and changes only DTR; intended control sequence is 0B -> 0A -> 0B. Zero readback previously lost RTS/OUT2. The low and high portions each wait 30 guest ticks through 0x8024BBC0. Tick accessor 0x8030AD78 reads 0x80002730; callback 0x8022C2CC increments it. At the 220M cutoff, only about 24 tick periods had elapsed since lowering DTR.

Queued sendstr reaches 0x8020C590: TX buffer 0x800010C4, producer 0x10D4, consumer 0x10D8. IIR 0x02 selects 0x802216F4 via table slot 0x80221844. That handler requires MSR CTS bit 0x10 and writes bytes at 0x80221750, disabling TX interrupts when its software ring empties.

Remaining boundary: ConfigureModem waits for OK after the first configuration, then a phone-setting-dependent ATV0 command and numeric result 0 before dial-prefix/number submission. A later successful dial result leads to ATD and Waiting for answer. The local model does not yet interpret normal post-download Hayes commands or supply genuine call-state responses; no OK/CONNECT result was fabricated in this pass. Premature DCD assertion from MCR readiness and the old no-carrier diagnostic remain unresolved model limitations. A future pass must implement or connect an evidenced modem command/call lifecycle, not revive missing-Power theories or inject guest events.
