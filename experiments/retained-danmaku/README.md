# Retained DirectComposition danmaku experiment (Windows)

> **Windows security notice (2026-11-25):** Microsoft Defender blocked one local unsigned experiment binary with the detection name `Trojan:Win32/Bearfoos.A!ml`. The cause has not been determined. Do not run that binary, restore it, or create an antivirus exclusion. This branch contains source only and is not a verified distribution.

This branch preserves the NucBox prototype as two source patches. The regular source tree and release workflow remain unchanged. The opt-in build copies the checked-in source and the pinned MyGo v0.10.0 dependency to a temporary directory, applies the patches, and builds the same test entry point used for the measurements. It does not edit the standard checkout or the Go module cache. No downloaded video, session data, binaries, or run logs are included.

Prerequisites: Windows, Go 1.27.3, Git, and tar. From the repository root:

```powershell
.\experiments\retained-danmaku\Build.ps1
```

To run an isolated 22-second native benchmark, choose a public video and use a fresh PowerShell process. The harness writes startup/final metadata and exits on its own:

```powershell
$run = Join-Path $env:TEMP ('mygo-retained-run-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $run | Out-Null
$env:APPDATA = $run
$env:MYGO_BENCH_OUT = $run
$env:MYGO_BENCH_SESSION = Split-Path -Leaf $run
$env:MYGO_BENCH_MODE = 'native'
$env:MYGO_BENCH_VID = 'sm9'
$env:MYGO_PROBE_COMPOSE = '1'
$env:MYGO_PROBE_RETAIN = 'D' # Retained layers. Use C for conventional drawing in an equivalent window.
.\mygo-retained-experiment.exe
```

Start a new process and use a new output/profile directory for each comparison. `MYGO_BENCH_CONTROL=1` enables the separate pause/seek/resume check and should not be set for the performance trial. `MYGO_PROBE_DIAG=1` and `MYGO_PROBE_COMPOSITION_STATS=1` enable diagnostics. Do not read the compositor rate estimator as the actual on-screen cadence of individual comments.

NucBox measurements against the conventional native parity renderer (two runs of ~22 s per mode):

| Video | App + desktop-compositor GPU | CPU | Peak on-screen candidates |
|---|---:|---:|---:|
| sm9 (dense) | 40.9% → 31.4% | 7.0% → 4.3% | 61 → 61 |
| sm44301514 (720p) | 35.3% → 24.9% | 9.5% → 5.4% | 25 → 25 |

The dense clip included a 4,474 px wide bitmap, showed no fallback, and used about 24 MiB more private working set. Pause, seek, and resume were visually checked. Actual displayed comment cadence, buffering, other DPI settings and production failure recovery are not qualified. The patches retain benchmark instrumentation and are not intended to land as a production dependency fork.
