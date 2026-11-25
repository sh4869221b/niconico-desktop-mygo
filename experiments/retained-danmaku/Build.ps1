[CmdletBinding()]
param([string]$OutputPath = (Join-Path (Get-Location) 'mygo-retained-experiment.exe'))
$ErrorActionPreference = 'Stop'
function Invoke-Checked([string]$Program,[string[]]$ArgsList) {
  & $Program @ArgsList
  if ($LASTEXITCODE -ne 0) { throw "$Program failed (exit $LASTEXITCODE)." }
}
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$OutputPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputPath)
$work = Join-Path ([IO.Path]::GetTempPath()) ('mygo-retained-' + [guid]::NewGuid().ToString('N'))
$app = Join-Path $work 'app'
$lib = Join-Path $work 'mygo'
$archive = Join-Path $work 'tracked.tar'
$null = New-Item -ItemType Directory -Path $app
Invoke-Checked git @('-C',$repo,'archive','--format=tar',"--output=$archive",'HEAD')
Invoke-Checked tar @('-xf',$archive,'-C',$app)
Push-Location $app
try {
  $raw = Invoke-Checked go @('mod','download','-json','github.com/egoist/mygo@v0.10.0')
  $info = ($raw -join "`n") | ConvertFrom-Json
  if (-not $info.Dir -or $info.Version -ne 'v0.10.0') { throw 'MyGo v0.10.0 download did not return a module directory.' }
  Copy-Item -Recurse -LiteralPath $info.Dir -Destination $lib
  Get-ChildItem -LiteralPath $lib -Recurse -File -Force | ForEach-Object { $_.IsReadOnly = $false }
  Invoke-Checked git @('apply','--check',(Join-Path $PSScriptRoot 'app.patch'))
  Invoke-Checked git @('apply',(Join-Path $PSScriptRoot 'app.patch'))
  Push-Location $lib
  try {
    Invoke-Checked git @('apply','--check',(Join-Path $PSScriptRoot 'mygo-v0.10.0.patch'))
    Invoke-Checked git @('apply',(Join-Path $PSScriptRoot 'mygo-v0.10.0.patch'))
  } finally { Pop-Location }
  Invoke-Checked go @('mod','edit',('-replace=github.com/egoist/mygo=' + ($lib -replace '\\','/')))
  Invoke-Checked go @('test','./...')
  Invoke-Checked go @('test','-c','-o',$OutputPath)
} finally { Pop-Location }
Write-Output "Experiment executable: $OutputPath"
Write-Output "Isolated source workspace: $work"

