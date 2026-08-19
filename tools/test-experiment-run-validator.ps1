[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$sourceRuns = Join-Path $repoRoot 'experiment-runs/records'
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-run-tests-' + [guid]::NewGuid().ToString('N'))
$tempRuns = Join-Path $tempRoot 'records'; $tempIndex = Join-Path $tempRoot 'index.md'
$validator = Join-Path $PSScriptRoot 'validate-experiment-runs.ps1'; $builder = Join-Path $PSScriptRoot 'build-experiment-run-index.ps1'
$passed = 0

function Reset-Fixture {
    if (Test-Path $tempRuns) { Remove-Item -LiteralPath $tempRuns -Recurse -Force }
    New-Item -ItemType Directory -Path $tempRuns -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $sourceRuns 'near-frequency-beating-planned-a.yaml') -Destination $tempRuns
    Copy-Item -LiteralPath (Join-Path $sourceRuns 'waveform-timbre-comparison-planned-a.yaml') -Destination $tempRuns
    & $builder -RunDirectory $tempRuns -OutputPath $tempIndex | Out-Null
}
function Replace-Text([string]$Path, [string]$Old, [string]$New) {
    $text=[IO.File]::ReadAllText($Path); if(-not $text.Contains($Old)){throw "Test fixture did not contain expected text: $Old"}; [IO.File]::WriteAllText($Path,$text.Replace($Old,$New),[Text.UTF8Encoding]::new($false))
}
function Expect-Failure([string]$Name, [scriptblock]$Sabotage) {
    Reset-Fixture; & $Sabotage
    try { & $validator -RunDirectory $tempRuns -IndexPath $tempIndex *> $null; throw "Adversarial test unexpectedly passed: $Name" } catch { if($_.Exception.Message -like 'Adversarial test unexpectedly passed:*'){throw}; $script:passed++; Write-Output "PASS: $Name" }
}

try {
    Expect-Failure 'duplicate run id' { Replace-Text (Join-Path $tempRuns 'waveform-timbre-comparison-planned-a.yaml') 'id: "waveform-timbre-comparison-planned-a"' 'id: "near-frequency-beating-planned-a"' }
    Expect-Failure 'nonexistent experiment reference' { Replace-Text (Join-Path $tempRuns 'near-frequency-beating-planned-a.yaml') 'experiment_id: "near-frequency-beating"' 'experiment_id: "missing-experiment"' }
    Expect-Failure 'unknown field' { $path=Join-Path $tempRuns 'near-frequency-beating-planned-a.yaml'; [IO.File]::AppendAllText($path,'unknown_field: "no"' + "`n",[Text.UTF8Encoding]::new($false)) }
    Expect-Failure 'invalid status' { Replace-Text (Join-Path $tempRuns 'near-frequency-beating-planned-a.yaml') 'status: "planned"' 'status: "published"' }
    Expect-Failure 'completed run without evidence' { $path=Join-Path $tempRuns 'near-frequency-beating-planned-a.yaml'; Replace-Text $path 'run_date: null' 'run_date: "2026-08-18"'; Replace-Text $path 'status: "planned"' 'status: "completed"' }
    Expect-Failure 'malformed measurement object' { $path=Join-Path $tempRuns 'near-frequency-beating-planned-a.yaml'; Replace-Text $path 'measurements: []' 'measurements: [{"quantity":"frequency"}]' }
    Expect-Failure 'invented index entry' { $text=[IO.File]::ReadAllText($tempIndex); [IO.File]::WriteAllText($tempIndex,$text + '- `invented-run` — `planned`' + "`n",[Text.UTF8Encoding]::new($false)) }
    Expect-Failure 'omitted index entry' { $text=[IO.File]::ReadAllText($tempIndex); $text=[regex]::Replace($text,'(?m)^- `near-frequency-beating-planned-a`[^\r\n]*\r?\n','',1); [IO.File]::WriteAllText($tempIndex,$text,[Text.UTF8Encoding]::new($false)) }
    Expect-Failure 'wrong index total' { Replace-Text $tempIndex 'Experiment runs: 2' 'Experiment runs: 99' }
    Expect-Failure 'reordered index members' { $a='- `near-frequency-beating-planned-a` — `near-frequency-beating`'; $b='- `waveform-timbre-comparison-planned-a` — `waveform-timbre-comparison`'; $text=[IO.File]::ReadAllText($tempIndex); $text=$text.Replace($a,"__ORDER_SENTINEL__").Replace($b,$a).Replace('__ORDER_SENTINEL__',$b); [IO.File]::WriteAllText($tempIndex,$text,[Text.UTF8Encoding]::new($false)) }
    Expect-Failure 'stale generated output' { [IO.File]::AppendAllText($tempIndex,"`n",[Text.UTF8Encoding]::new($false)) }
    Expect-Failure 'unexpanded template artifact' { [IO.File]::AppendAllText($tempIndex,'$(' + 'unexpanded)' + "`n",[Text.UTF8Encoding]::new($false)) }
    Write-Output "adversarial_tests_passed: $passed"
} finally { if(Test-Path $tempRoot){Remove-Item -LiteralPath $tempRoot -Recurse -Force} }
