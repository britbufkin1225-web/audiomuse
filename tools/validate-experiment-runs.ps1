[CmdletBinding()]
param([string]$RunDirectory, [string]$IndexPath)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not $RunDirectory) { $RunDirectory = Join-Path $repoRoot 'experiment-runs/records' }
if (-not $IndexPath) { $IndexPath = Join-Path $repoRoot 'experiment-runs/index.md' }
$required = @('id','experiment_id','run_date','status','environment_notes','equipment','software','procedure_deviations','control_settings','observations','measurements','limitations','safety_notes','interpretation','follow_up_questions','source_refs')
$stringArrays = @('environment_notes','equipment','software','procedure_deviations','limitations','safety_notes','interpretation','follow_up_questions','source_refs')
$objectArrays = @('control_settings','observations','measurements')
$validStatus = @('planned','completed','incomplete','invalid')
$validCalibration = @('known','unknown','not-applicable')

function Assert-ExactObjectFields($Object, [string[]]$Fields, [string]$Context) {
    if ($Object -isnot [pscustomobject]) { throw "$Context must be an object." }
    $names = @($Object.psobject.Properties.Name)
    foreach ($name in $names) { if ($name -notin $Fields) { throw "Unsupported field '$name' in $Context." } }
    foreach ($field in $Fields) { if ($field -notin $names) { throw "Missing field '$field' in $Context." } }
}
function Assert-NonEmptyString($Value, [string]$Context) {
    if ($Value -isnot [string] -or -not $Value.Trim()) { throw "$Context must be a non-empty string." }
}
function Assert-UniqueStrings($Values, [string]$Context) {
    $seen = @{}
    foreach ($value in $Values) { Assert-NonEmptyString $value $Context; if ($seen.ContainsKey($value)) { throw "Duplicate value in ${Context}: $value" }; $seen[$value] = $true }
}
function Assert-UniqueObjects($Values, [string]$Context) {
    $seen=@{}; foreach($value in $Values){$key=$value | ConvertTo-Json -Compress -Depth 5; if($seen.ContainsKey($key)){throw "Duplicate object in $Context."}; $seen[$key]=$true}
}
function Get-Ordinal([object[]]$Values) { $items=[string[]]@($Values | ForEach-Object {[string]$_}); [Array]::Sort($items,[StringComparer]::Ordinal); return $items }

$records = [Collections.Generic.List[object]]::new()
foreach ($file in Get-ChildItem -LiteralPath $RunDirectory -Filter '*.yaml' -File) {
    $values = [ordered]@{}
    foreach ($line in [IO.File]::ReadAllLines($file.FullName)) {
        if ($line -notmatch '^(?<key>[a-z_]+): (?<value>.+)$') { throw "Malformed experiment-run line in $($file.Name): $line" }
        $key = $Matches.key
        if ($key -notin $required -or $values.Contains($key)) { throw "Unsupported or duplicate experiment-run field '$key' in $($file.Name)" }
        try { $values[$key] = $Matches.value | ConvertFrom-Json -NoEnumerate } catch { throw "Experiment-run field '$key' must use a JSON-compatible YAML value in $($file.Name)" }
    }
    if ($values.Count -ne $required.Count -or @($required | Where-Object {-not $values.Contains($_)}).Count) { throw "Missing required experiment-run field in $($file.Name)" }
    Assert-NonEmptyString $values.id "id in $($file.Name)"; Assert-NonEmptyString $values.experiment_id "experiment_id in $($file.Name)"; Assert-NonEmptyString $values.status "status in $($file.Name)"
    if ($values.id -notmatch '^[a-z0-9]+(?:-[a-z0-9]+)*$') { throw "Invalid experiment-run id: $($values.id)" }
    foreach ($field in @($stringArrays + $objectArrays)) { if ($values[$field] -isnot [array]) { throw "Experiment-run field '$field' must be an array in $($file.Name)" } }
    foreach ($field in $stringArrays) { Assert-UniqueStrings $values[$field] "$field for $($values.id)" }
    foreach ($field in $objectArrays) { Assert-UniqueObjects $values[$field] "$field for $($values.id)" }
    $records.Add([pscustomobject]$values)
}
if (-not $records.Count) { throw 'No experiment-run records found.' }

# Reference sets compare with StringComparer::Ordinal. A default hashtable resolves case-insensitively,
# which would accept a record reference that does not literally match the registered canonical id.
$experimentIds = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); Get-ChildItem (Join-Path $repoRoot 'experiments/records') -Filter '*.yaml' -File | ForEach-Object { $m=[regex]::Match([IO.File]::ReadAllText($_.FullName),'(?m)^id: "(?<id>[a-z0-9-]+)"\r?$'); if($m.Success){[void]$experimentIds.Add($m.Groups['id'].Value)} }
$sourceIds = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); [regex]::Matches([IO.File]::ReadAllText((Join-Path $repoRoot 'sources/source-registry.yaml')),'(?m)^  - id: (?<id>[a-z0-9-]+)\r?$') | ForEach-Object {[void]$sourceIds.Add($_.Groups['id'].Value)}
$ids=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); $observationCount=0; $measurementCount=0
foreach ($record in $records) {
    if (-not $ids.Add($record.id)) { throw "Duplicate experiment-run id: $($record.id)" }
    if (-not $experimentIds.Contains($record.experiment_id)) { throw "Unresolved experiment id for $($record.id): $($record.experiment_id)" }
    if (-not ($validStatus -ccontains $record.status)) { throw "Invalid experiment-run status for $($record.id): $($record.status)" }
    if ($null -ne $record.run_date -and ($record.run_date -isnot [string] -or $record.run_date -notmatch '^\d{4}-(0[1-9]|1[0-2])-([012]\d|3[01])$')) { throw "Invalid run_date for $($record.id); use ISO YYYY-MM-DD or null." }
    if ($null -ne $record.run_date) { $parsed=[datetime]::MinValue; if(-not [datetime]::TryParseExact($record.run_date,'yyyy-MM-dd',[Globalization.CultureInfo]::InvariantCulture,[Globalization.DateTimeStyles]::None,[ref]$parsed)){throw "Impossible run_date for $($record.id): $($record.run_date)"}
        # One day of tolerance covers author time zones ahead of UTC without admitting a date that cannot have happened.
        if($parsed.Date -gt [datetime]::UtcNow.Date.AddDays(1)){throw "Future run_date for $($record.id): $($record.run_date); a run cannot be recorded before it is performed."} }
    $hasEvidence = $record.observations.Count -gt 0 -or $record.measurements.Count -gt 0
    if ($record.status -ceq 'planned' -and ($null -ne $record.run_date -or $hasEvidence -or $record.interpretation.Count -gt 0 -or $record.procedure_deviations.Count -gt 0)) { throw "Planned run $($record.id) cannot contain a run date, evidence, interpretation, or procedure deviations." }
    if ($record.status -cne 'planned' -and $null -eq $record.run_date) { throw "Performed run $($record.id) requires run_date." }
    if ($record.status -ceq 'completed' -and -not $hasEvidence) { throw "Completed run $($record.id) requires observation or measurement evidence." }
    if ($record.status -ceq 'invalid' -and $record.interpretation.Count -gt 0) { throw "Invalid run $($record.id) cannot contain interpretation." }
    foreach ($setting in $record.control_settings) {
        Assert-ExactObjectFields $setting @('quantity','value','unit','context') "control setting for $($record.id)"
        Assert-NonEmptyString $setting.quantity "control-setting quantity for $($record.id)"; Assert-NonEmptyString $setting.context "control-setting context for $($record.id)"
        if (($setting.value -isnot [string] -and $setting.value -isnot [ValueType]) -or $setting.value -is [bool]) { throw "Control-setting value for $($record.id) must be a string or number." }
        if ($setting.value -is [string] -and -not $setting.value.Trim()) { throw "Control-setting value for $($record.id) cannot be empty." }
        if ($null -ne $setting.unit) { Assert-NonEmptyString $setting.unit "control-setting unit for $($record.id)" }
    }
    foreach ($observation in $record.observations) {
        Assert-ExactObjectFields $observation @('statement','context') "observation for $($record.id)"
        Assert-NonEmptyString $observation.statement "observation statement for $($record.id)"; Assert-NonEmptyString $observation.context "observation context for $($record.id)"; $observationCount++
    }
    foreach ($measurement in $record.measurements) {
        Assert-ExactObjectFields $measurement @('quantity','value','unit','method','tool','calibration','uncertainty','limitations') "measurement for $($record.id)"
        foreach ($field in @('quantity','unit','method','tool','calibration')) { Assert-NonEmptyString $measurement.$field "measurement $field for $($record.id)" }
        if ($measurement.value -isnot [ValueType] -or $measurement.value -is [bool]) { throw "Measurement value for $($record.id) must be numeric." }
        if (-not ($validCalibration -ccontains $measurement.calibration)) { throw "Invalid measurement calibration for $($record.id): $($measurement.calibration)" }
        if ($null -ne $measurement.uncertainty) { Assert-NonEmptyString $measurement.uncertainty "measurement uncertainty for $($record.id)" }
        if ($measurement.limitations -isnot [array]) { throw "Measurement limitations for $($record.id) must be an array." }; Assert-UniqueStrings $measurement.limitations "measurement limitations for $($record.id)"
        if ($measurement.calibration -ceq 'unknown' -and $measurement.limitations.Count -eq 0) { throw "Measurement with unknown calibration for $($record.id) requires a limitation." }; $measurementCount++
    }
    foreach ($ref in $record.source_refs) { if (-not $sourceIds.Contains($ref)) { throw "Unresolved source reference for $($record.id): $ref" } }
}

if (-not (Test-Path -LiteralPath $IndexPath)) { throw 'Missing generated experiment-run index.' }
$text=[IO.File]::ReadAllText($IndexPath)
foreach($marker in @('System.Object[]','$(')){if($text.Contains($marker)){throw "Experiment-run index contains raw or unexpanded template output: $marker"}}
$lines=[Collections.Generic.HashSet[string]]::new([string[]]($text -split '\r?\n'),[StringComparer]::Ordinal)
foreach($heading in @('# AudioMuse Experiment Run Index','## By Experiment','## By Status')){if(-not $lines.Contains($heading)){throw "Experiment-run index missing section: $heading"}}
if(-not $lines.Contains("Experiment runs: $($records.Count)")){throw 'Experiment-run index has an incorrect declared count.'}
$listed=@([regex]::Matches($text,'(?m)^- `(?<id>[a-z0-9-]+)` — `(?<value>[a-z0-9-]+)`(?: — .+)?$') | ForEach-Object {$_.Groups['id'].Value})
foreach($id in $listed){if(-not $ids.Contains($id)){throw "Experiment-run index contains invented id: $id"}}
foreach($record in $records){if(@($listed | Where-Object {$_ -ceq $record.id}).Count -ne 2){throw "Experiment-run index omits or duplicates record: $($record.id)"}}
# Every projected line is re-derived in full from the canonical records, not just its run id. Verifying
# only the id would leave the projected status, experiment mapping, and run date detectable solely by the
# regeneration byte-comparison below, so a builder that mislabelled a run would be confirmed by a validator
# sharing the same assumption. That is the defect class already hardened in validate-experiments.ps1.
foreach($view in @(@{Field='experiment_id';Heading='By Experiment'},@{Field='status';Heading='By Status'})){
    foreach($key in Get-Ordinal @($records.($view.Field) | Select-Object -Unique)){
        $members=@($records | Where-Object {$_.($view.Field) -ceq $key}); $heading="### ``$key`` — $($members.Count) runs"
        $start=$text.IndexOf($heading,[StringComparison]::Ordinal); if($start -lt 0){throw "Experiment-run index missing or miscounts grouping: $key"}
        $next=$text.IndexOf("`n### ",$start+$heading.Length,[StringComparison]::Ordinal); $section=$text.IndexOf("`n## ",$start+$heading.Length,[StringComparison]::Ordinal); if($next -lt 0 -or ($section -ge 0 -and $section -lt $next)){$next=$section}; if($next -lt 0){$next=$text.Length}
        $body=$text.Substring($start,$next-$start); $actual=@([regex]::Matches($body,'(?m)^-.*$') | ForEach-Object {$_.Value})
        $expected=@(foreach($id in Get-Ordinal @($members.id)){
            $member=@($members | Where-Object {$_.id -ceq $id})[0]
            if($view.Field -eq 'experiment_id'){ $shown=if($null -eq $member.run_date){'not performed'}else{$member.run_date}; "- ``$($member.id)`` — ``$($member.status)`` — $shown" } else { "- ``$($member.id)`` — ``$($member.experiment_id)``" } })
        if($actual.Count -ne $expected.Count){throw "Experiment-run grouping '$key' lists $($actual.Count) runs; expected $($expected.Count)."}
        for($i=0;$i -lt $expected.Count;$i++){if($actual[$i] -cne $expected[$i]){throw "Experiment-run grouping '$key' line $($i+1) does not match canonical record data or ordinal run-id order: expected '$($expected[$i])', found '$($actual[$i])'."}}
    }
}
$temp=Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-runs-'+[guid]::NewGuid().ToString('N')+'.md')
try { & (Join-Path $PSScriptRoot 'build-experiment-run-index.ps1') -RunDirectory $RunDirectory -OutputPath $temp | Out-Null; if([Convert]::ToBase64String([IO.File]::ReadAllBytes($temp)) -cne [Convert]::ToBase64String([IO.File]::ReadAllBytes($IndexPath))){throw 'Generated experiment-run index is stale.'} } finally {if(Test-Path $temp){Remove-Item -LiteralPath $temp -Force}}
Write-Output "experiment_runs: $($records.Count)"; foreach($status in $validStatus){Write-Output "  ${status}: $(@($records | Where-Object status -eq $status).Count)"}; Write-Output "run_observations: $observationCount"; Write-Output "run_measurements: $measurementCount"; Write-Output 'unresolved_run_experiment_refs: 0'; Write-Output 'duplicate_run_ids: 0'; Write-Output 'experiment_run_index_reconciled: true'; Write-Output 'experiment_run_index_structure_verified: true'; Write-Output 'experiment_run_index_current: true'
