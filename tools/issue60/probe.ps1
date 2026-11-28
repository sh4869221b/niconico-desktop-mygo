param(
    [Parameter(Mandatory=$true)][int]$ProcessId,
    [Parameter(Mandatory=$true)][string]$Log,
    [int]$Count = 40,
    [int]$IntervalMs = 100
)
# Foreground-only synthetic input to this measurement window. Never activates
# another app or changes Windows settings. Stop if the target loses focus.
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class Issue60Foreground {
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint process);
}
'@
Add-Type -AssemblyName System.Windows.Forms
if (Test-Path $Log) { throw 'Log already exists; select a new path.' }
$target = Get-Process -Id $ProcessId -ErrorAction Stop
if ($target.ProcessName -notlike '*test*') { throw 'Target must be the measurement test process.' }
Write-Host 'Focus the Issue 60 window within 5 seconds, then click Save during the probe.'
Start-Sleep -Seconds 5
for ($i=0; $i -lt $Count; $i++) {
    [uint32]$foregroundProcess = 0
    $hwnd = [Issue60Foreground]::GetForegroundWindow()
    [void][Issue60Foreground]::GetWindowThreadProcessId($hwnd, [ref]$foregroundProcess)
    if ($foregroundProcess -ne $ProcessId) { throw 'Measurement window lost foreground; stopped.' }
    $start = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    [System.Windows.Forms.SendKeys]::SendWait('{F8}')
    @{ event='probe-sent'; ordinal=$i; start_utc_ms=$start; return_utc_ms=[DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds() } | ConvertTo-Json -Compress | Add-Content $Log
    Start-Sleep -Milliseconds $IntervalMs
}
