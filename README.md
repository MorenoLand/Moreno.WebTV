# Moreno.WebTV

Standalone Go emulator workbench for WebTV Plus preservation.

The current target is a WebTV Plus-style machine with a SOLO ASIC, R4640 MIPS III CPU, 8 MB RAM, boot ROM mapped at `0xBFC00000`, firmware flash at `0x1F000000`, and an eventual SDL2 video/input frontend plus WTVP server connectivity.

This repo is still bring-up/debug code. It currently contains the CPU interpreter, memory bus, SOLO ASIC stub, IDE controller, I2C EEPROM model, tests, and firmware boot instrumentation.

## Current State

The emulator boots real boot ROM code and reaches firmware code paths. Recent work fixed several core correctness issues:

- ROM bounds checks no longer overflow on bad physical addresses.
- ERL-mode KSEG2 direct physical mapping is implemented.
- Load/store TLB misses now abort the faulting instruction instead of writing `0xFFFFFFFF` garbage.
- Delay-slot exceptions preserve branch EPC/BD behavior.
- Initial SOLO ASIC timer/RIO/scheduler experiments are instrumented.

The normal boot path now passes the external-printer-adapter check and completes the boot-ROM modem download. The current blocker is the post-carrier network transition: the firmware waits for event key `2` at RAM gate byte `0x8000224E`, but the emulator has no in-process PPP/WTVP peer to supply the event or received bytes, so IDE reads and live firmware video have not started. The optional TCP modem bridge forwards serial traffic to an external peer but does not implement PPP itself.

The SDL2 frame viewer opens by default after the firmware run stops. Set `WEBTV_VIDEO=0` to disable it. It loads `SDL2.dll` from `SDL2_DLL`, then the local `.tools` package, then the system path. The current presentation path uses YUY2, based on the firmware-programmed display buffer.

Set `WEBTV_MODEM_TCP` to a TCP serial-modem peer such as `127.0.0.1:57388` to forward post-download UART bytes and receive peer data. The default synthetic modem remains active when this option is unset.

## Repository Layout

- `main.go` - boot harness and current trace/debug run loop.
- `cpu.go` - R4640/MIPS III interpreter and COP0/TLB handling.
- `bus.go` - physical memory/bus mapping.
- `asic.go` - SOLO ASIC register model and interrupt/timer stubs.
- `ide.go` - IDE disk image access.
- `*_test.go` - focused CPU/bus/ASIC/IDE regression tests.
- `roms/` - local-only firmware inputs. See `roms/README.md`.

## Local Files Not Committed

Firmware, ROM images, generated traces, framebuffer dumps, preview images, and built executables are intentionally ignored.

Expected local inputs include:

- `roms/bootrom.brom`
- `roms/ptnvram.iic` if available
- a WebTV Plus disk/flash image, passed on the command line

## Build And Test

This repo is plain Go and currently uses the local Go 1.21 toolchain.

```powershell
$env:GOMODCACHE='G:\Development\Go\.gomodcache'
$env:GOCACHE='G:\Development\Go\.gocache'
$env:GOTOOLCHAIN='local'
go test ./...
go build -buildvcs=false -o webtv_trace.exe .
```

Run with a local firmware image:

```powershell
.\webtv_trace.exe "G:\Development\Working\WebTV Preservation\HackTV-7352-ndb-For_Old_WebTV_Plus-Themes-8-16-2025.img"
```

## Notes

This is preservation/debug work. Do not commit proprietary ROMs, disk images, generated memory dumps, or local preview artifacts.
