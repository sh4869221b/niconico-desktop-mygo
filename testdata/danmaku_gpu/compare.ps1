# Run in a candidate checkout with the fixed baseline commit available locally.
$ErrorActionPreference = "Stop"
$here = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$base = "443592d9e4df5b52859aef11cbce30f3593c2787"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
$old = Join-Path $tmp "base"
New-Item -ItemType Directory $tmp | Out-Null

function Read-Frames([string]$root, [string]$label) {
    $map = @{}
    $map[(Join-Path $root "z_compare_danmaku_windows_test.go")] = Join-Path $PSScriptRoot "frames_windows_test.go"
    $map[(Join-Path $root "dependencies/mygo/ui/z_danmaku_readback_windows.go")] = Join-Path $PSScriptRoot "recorder_windows.go"
    $map[(Join-Path $root "dependencies/mygo/internal/gpu/d3d11/z_danmaku_readback_windows.go")] = Join-Path $PSScriptRoot "d3d11_windows.go"
    $overlay = Join-Path $tmp "$label.json"
    @{Replace = $map} | ConvertTo-Json -Depth 3 | Set-Content -Encoding utf8NoBOM $overlay
    Push-Location $root
    try {
        $log = & go test -mod=readonly "-overlay=$overlay" -count=1 -run '^TestCompareD3D11Danmaku$' -v .
        if ($LASTEXITCODE) { throw ($log -join [Environment]::NewLine) }
        foreach ($row in $log | Where-Object { $_ -match ': WORK ' }) {
            $line = "$label " + ($row -split ': WORK ',2)[1]
            Write-Host $line
            if ($env:GITHUB_STEP_SUMMARY) { Add-Content -LiteralPath $env:GITHUB_STEP_SUMMARY -Value "- $line" }
        }
        $rows = @($log | Where-Object { $_ -match ': FRAME ' } | ForEach-Object { ($_ -split ': FRAME ',2)[1] })
        if ($rows.Count -ne 54) { throw "Expected 54 readbacks for $label, found $($rows.Count)" }
        return $rows
    } finally { Pop-Location }
}

try {
    git -C $here worktree add --detach $old $base
    if ($LASTEXITCODE) { throw "Cannot check out the baseline" }
    $a = @(Read-Frames $old "baseline")
    $b = @(Read-Frames $here "candidate")
    $diff = @()
    for ($i=0; $i -lt $a.Count; $i++) {
        if ($a[$i] -cne $b[$i]) { $diff += "Frame $i differs. Baseline $($a[$i]). Candidate $($b[$i])." }
    }
    if ($diff.Count) {
        $repeat = @(Read-Frames $old "baseline-repeat")
        for ($i=0; $i -lt $a.Count; $i++) {
            if ($a[$i] -cne $repeat[$i]) { throw "Baseline is not reproducible at frame $i." }
        }
        $diff | Select-Object -First 3 | Write-Host
        throw "$($diff.Count) of $($a.Count) frames differ from the repeatable baseline."
    }
    Write-Output "54 DirectWrite/D3D11 readbacks, active counts and glyph counts match $base."
} finally {
    if (Test-Path (Join-Path $old ".git")) { git -C $here worktree remove $old }
    Remove-Item -LiteralPath $tmp -Recurse -Force
}
