# Build the public glyph shader source with the system Direct3D compiler.
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$overlay = [IO.Path]::GetTempFileName()
try {
    $map = @{}
    $map[(Join-Path $root "dependencies/mygo/internal/gpu/d3d11/z_emit_danmaku_windows_test.go")] = Join-Path $PSScriptRoot "shaders_windows_test.go"
    @{Replace = $map} | ConvertTo-Json -Depth 3 | Set-Content -Encoding utf8NoBOM $overlay
    $env:DANMAKU_GLYPH_HLSL = Join-Path $PSScriptRoot "glyph_coverage.hlsl"
    go test -mod=readonly "-overlay=$overlay" -count=1 -run '^TestEmitDanmakuShaders$' -v github.com/egoist/mygo/internal/gpu/d3d11
    if ($LASTEXITCODE) { throw "Unable to compile the glyph class shaders" }
} finally {
    Remove-Item -LiteralPath $overlay
}
