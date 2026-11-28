param(
    [Parameter(Mandatory=$true)][int]$ProcessId,
    [Parameter(Mandatory=$true)][string]$Label,
    [Parameter(Mandatory=$true)][string]$Output,
    [int]$MaxNodes = 2000
)
# Read-only UIA snapshot of this process's top-level windows, with a bounded tree.
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
$target = Get-Process -Id $ProcessId -ErrorAction Stop
if ($MaxNodes -lt 1 -or $MaxNodes -gt 10000) { throw 'MaxNodes must be 1..10000.' }
if (Test-Path $Output) { throw 'Output exists; select a new file.' }
$condition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $ProcessId)
$roots = [System.Windows.Automation.AutomationElement]::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $condition)
$walker = [System.Windows.Automation.TreeWalker]::ControlViewWalker
$rows = New-Object System.Collections.Generic.List[object]
foreach ($root in $roots) {
    $queue = New-Object System.Collections.Generic.Queue[object]
    $queue.Enqueue(@{element=$root; depth=0})
    $count = 0
    while ($queue.Count -gt 0 -and $count -lt $MaxNodes) {
        $item = $queue.Dequeue()
        $count++
        try {
            $element = $item.element
            $c = $element.Current
            $r = $c.BoundingRectangle
            $rows.Add([pscustomobject]@{
                window = $root.Current.Name; depth = $item.depth;
                name = $c.Name; role = $c.ControlType.ProgrammaticName;
                automation_id = $c.AutomationId; process_id = $c.ProcessId;
                focused = $c.HasKeyboardFocus; focusable = $c.IsKeyboardFocusable;
                offscreen = $c.IsOffscreen; enabled = $c.IsEnabled;
                bounds = @{x=$r.X; y=$r.Y; width=$r.Width; height=$r.Height}
            })
            $child = $walker.GetFirstChild($element)
            while ($null -ne $child -and $queue.Count -lt $MaxNodes) {
                $queue.Enqueue(@{element=$child; depth=$item.depth+1})
                $child = $walker.GetNextSibling($child)
            }
        } catch {
            $rows.Add([pscustomobject]@{window='unavailable'; depth=$item.depth; error=$_.Exception.Message})
        }
    }
    $rows.Add([pscustomobject]@{window=$root.Current.Name; snapshot_nodes=$count; truncated=($queue.Count -gt 0)})
}
$globalFocusPid = $null
$globalFocus = $null
try {
    $currentFocus = [System.Windows.Automation.AutomationElement]::FocusedElement.Current
    $globalFocusPid = $currentFocus.ProcessId
    if ($globalFocusPid -eq $ProcessId) {
        $globalFocus = @{
            name=$currentFocus.Name; role=$currentFocus.ControlType.ProgrammaticName;
            automation_id=$currentFocus.AutomationId; focused=$currentFocus.HasKeyboardFocus
        }
    }
} catch {}
[pscustomobject]@{
    label=$Label; utc=[DateTimeOffset]::UtcNow.ToString('o'); process_id=$ProcessId;
    process_name=$target.ProcessName; global_focus_process_id=$globalFocusPid;
    global_focus=$globalFocus;
    window_count=$roots.Count; nodes=$rows
} | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 $Output
