# Compare both renderers on the same live native-player scenes, including
# the identical DirectWrite atlas texels, UI frames and in-memory video.
param([string]$Revision = "HEAD")
$ErrorActionPreference = "Stop"
$here = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$base = "443592d9e4df5b52859aef11cbce30f3593c2787"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
$old = Join-Path $tmp "base"
$actual = Join-Path $tmp "candidate"
New-Item -ItemType Directory $tmp | Out-Null

try {
    git -C $here worktree add --detach $old $base
    if ($LASTEXITCODE) { throw "Cannot check out the reference" }
    git -C $here worktree add --detach $actual $Revision
    if ($LASTEXITCODE) { throw "Cannot check out $Revision" }

    # Compile the reference builder and D3D11 renderer as a second internal
    # package. Both take scene.Scene from the exact same frame of drawPlayer.
    $gpu = Join-Path $actual "dependencies/mygo/internal/gpu"
    $ref = Join-Path $actual "dependencies/mygo/internal/danmakuref"
    Copy-Item -LiteralPath (Join-Path $old "dependencies/mygo/internal/gpu") -Destination $ref -Recurse
    foreach ($file in Get-ChildItem -LiteralPath $ref -Recurse -Filter "*.go") {
        $src = [IO.File]::ReadAllText($file.FullName)
        $src2 = $src.Replace("github.com/egoist/mygo/internal/gpu", "github.com/egoist/mygo/internal/danmakuref")
        if ($src -cne $src2) { [IO.File]::WriteAllText($file.FullName, $src2) }
    }
    foreach ($dir in @($gpu, $ref)) {
        Copy-Item (Join-Path $PSScriptRoot "d3d11_windows.go") (Join-Path $dir "d3d11/z_danmaku_readback_windows.go")
    }
    Copy-Item (Join-Path $PSScriptRoot "recorder_windows.go") (Join-Path $actual "dependencies/mygo/ui/z_danmaku_readback_windows.go")
    Copy-Item (Join-Path $PSScriptRoot "frames_windows_test.go") (Join-Path $actual "z_compare_danmaku_windows_test.go")
    Push-Location $actual
    try {
        $out = & go test -mod=readonly -count=1 -run '^TestCompareD3D11Danmaku$' -v .
        $rc = $LASTEXITCODE
        $out | Write-Output
        if ($rc) { throw "D3D11 comparison for $Revision failed" }
        if (@($out | Where-Object { $_ -match ': FRAME ' }).Count -ne 54) { throw "Expected 54 readbacks" }
        $summary = "54 same-scene D3D11 images and instance counts match $base ($Revision)."
        Write-Output $summary
        if ($env:GITHUB_STEP_SUMMARY) { Add-Content -LiteralPath $env:GITHUB_STEP_SUMMARY -Value $summary }
    } finally { Pop-Location }
} finally {
    foreach ($dir in @($actual,$old)) {
        if (Test-Path (Join-Path $dir ".git")) { git -C $here worktree remove --force $dir }
    }
    Remove-Item -LiteralPath $tmp -Recurse -Force
}
