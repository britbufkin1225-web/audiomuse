[CmdletBinding()]
param([string]$ExperimentDirectory, [string]$IndexPath)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not $ExperimentDirectory) { $ExperimentDirectory = Join-Path $repoRoot 'experiments/records' }
if (-not $IndexPath) { $IndexPath = Join-Path $repoRoot 'experiments/index.md' }
$required = @('id','title','status','type','difficulty','purpose','node_refs','vocabulary_refs','session_refs','source_refs','required_equipment','optional_equipment','safety','setup','procedure','observations','measurements','expected_behavior','interpretation','limitations','repeatability','related_experiments','project_connections')
$arrayFields = $required | Where-Object { $_ -notin @('id','title','status','type','difficulty','purpose') }
$records = [Collections.Generic.List[object]]::new()
foreach ($file in Get-ChildItem -LiteralPath $ExperimentDirectory -Filter '*.yaml' -File | Sort-Object Name) {
    $values = [ordered]@{}
    foreach ($line in [IO.File]::ReadAllLines($file.FullName)) {
        if ($line -notmatch '^(?<key>[a-z_]+): (?<value>.+)$') { throw "Malformed experiment line in $($file.Name): $line" }
        $key=$Matches.key
        if ($key -notin $required -or $values.Contains($key)) { throw "Unsupported or duplicate experiment field '$key' in $($file.Name)" }
        try { $values[$key]=$Matches.value | ConvertFrom-Json -NoEnumerate } catch { throw "Experiment field '$key' must use a JSON-compatible YAML value in $($file.Name)" }
    }
    if ($values.Count -ne $required.Count -or @($required | Where-Object { -not $values.Contains($_) }).Count) { throw "Missing required experiment field in $($file.Name)" }
    foreach ($field in @('id','title','status','type','difficulty','purpose')) { if ($values[$field] -isnot [string] -or -not $values[$field].Trim()) { throw "Experiment field '$field' must be a non-empty string in $($file.Name)" } }
    foreach ($field in $arrayFields) { if ($values[$field] -isnot [array]) { throw "Experiment field '$field' must be an array in $($file.Name)" } }
    if ($values.id -notmatch '^[a-z0-9]+(?:-[a-z0-9]+)*$') { throw "Invalid experiment id: $($values.id)" }
    $records.Add([pscustomobject]$values)
}
if (-not $records.Count) { throw 'No canonical experiment records found.' }
$ids=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); foreach($record in $records){ if(-not $ids.Add($record.id)){throw "Duplicate experiment id: $($record.id)"} }
$validStatus=@('proof','established'); $validType=@('listening','visualization','hybrid'); $validDifficulty=@('introductory','intermediate')
$nodeIds=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); Get-ChildItem (Join-Path $repoRoot 'nodes') -Recurse -Filter '*.md' | Where-Object Name -ne 'README.md' | ForEach-Object { $m=[regex]::Match([IO.File]::ReadAllText($_.FullName),'(?m)^id: (?<id>[a-z0-9-]+)\r?$'); if($m.Success){[void]$nodeIds.Add($m.Groups['id'].Value)} }
$vocabIds=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); Get-ChildItem (Join-Path $repoRoot 'vocabulary/entries') -Filter '*.yaml' -File | ForEach-Object { [regex]::Matches([IO.File]::ReadAllText($_.FullName),'(?m)^id: "(?<id>[a-z0-9-]+)"\r?$') | ForEach-Object {[void]$vocabIds.Add($_.Groups['id'].Value)} }
$sourceTypes=[Collections.Generic.Dictionary[string,string]]::new([StringComparer]::Ordinal); [regex]::Matches([IO.File]::ReadAllText((Join-Path $repoRoot 'sources/source-registry.yaml')),'(?m)^  - id: (?<id>[a-z0-9-]+)\r?\n    type: (?<type>[a-z0-9-]+)\r?$') | ForEach-Object {$sourceTypes[$_.Groups['id'].Value]=$_.Groups['type'].Value}
$nodeRefs=0;$vocabRefs=0;$sessionRefs=0;$sourceRefs=0;$relatedRefs=0
foreach($record in $records){
    if($record.status -cnotin $validStatus){throw "Invalid experiment status for $($record.id): $($record.status)"}; if($record.type -cnotin $validType){throw "Invalid experiment type for $($record.id): $($record.type)"}; if($record.difficulty -cnotin $validDifficulty){throw "Invalid experiment difficulty for $($record.id): $($record.difficulty)"}
    foreach($field in $arrayFields){$seen=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); foreach($value in $record.$field){if($value -isnot [string] -or -not $value.Trim()){throw "Empty or non-string $field value for $($record.id)"};if(-not $seen.Add($value)){throw "Duplicate $field value for $($record.id): $value"}}}
    foreach($ref in $record.node_refs){if(-not $nodeIds.Contains($ref)){throw "Unresolved node reference for $($record.id): $ref"};$nodeRefs++}
    foreach($ref in $record.vocabulary_refs){if(-not $vocabIds.Contains($ref)){throw "Unresolved vocabulary reference for $($record.id): $ref"};$vocabRefs++}
    foreach($ref in $record.session_refs){if(-not $sourceTypes.ContainsKey($ref) -or $sourceTypes[$ref] -cne 'session'){throw "Invalid session reference for $($record.id): $ref"};$sessionRefs++}
    foreach($ref in $record.source_refs){if(-not $sourceTypes.ContainsKey($ref)){throw "Unresolved source reference for $($record.id): $ref"};$sourceRefs++}
    foreach($ref in $record.related_experiments){if($ref -ceq $record.id){throw "Self-related experiment: $($record.id)"};if(-not $ids.Contains($ref)){throw "Unresolved related experiment for $($record.id): $ref"};$relatedRefs++}
}
if(-not (Test-Path -LiteralPath $IndexPath)){throw 'Missing generated experiment index.'}
# Independent reconciliation reads committed Markdown without calling the builder. Every expected
# line is re-derived from canonical records so a builder formatting, ordering, or escaping defect
# cannot be masked by regenerating the projection with the same faulty logic.
$text=[IO.File]::ReadAllText($IndexPath); $listed=@([regex]::Matches($text,'(?m)^- \*\*.+?\*\* \(`(?<id>[a-z0-9-]+)`\)') | ForEach-Object {$_.Groups['id'].Value})
if($listed.Count -ne $records.Count){throw "Experiment index lists $($listed.Count) A-Z records but $($records.Count) canonical records exist."}
foreach($record in $records){if($record.id -cnotin $listed){throw "Canonical experiment missing from index: $($record.id)"}}
foreach($marker in @('System.Object[]','$(')){if($text.Contains($marker)){throw "Experiment index contains unexpanded template output ('$marker'); the builder emitted raw object text instead of markdown."}}
$present=[Collections.Generic.HashSet[string]]::new([string[]]($text -split '\r?\n'),[StringComparer]::Ordinal)
$expected=[Collections.Generic.List[string]]::new()
foreach($heading in @('# AudioMuse Experiment Index','## A-Z','## By Type','## By Canonical Node','## By Vocabulary Term','## By Session')){$expected.Add($heading)}
$expected.Add("Canonical experiments: $($records.Count)")
foreach($record in $records){$expected.Add(('- **{0}** (`{1}`) — {2}' -f $record.title,$record.id,$record.purpose))}
foreach($line in $expected){if(-not $present.Contains($line)){throw "Experiment index is missing expected line: $line"}}
# A-Z ordering is asserted against an independently sorted title list, not against builder output.
$azTitles=@([regex]::Matches($text,'(?m)^- \*\*(?<title>.+?)\*\* \(') | ForEach-Object {$_.Groups['title'].Value})
$sortedTitles=[string[]]@($azTitles); [Array]::Sort($sortedTitles,[StringComparer]::Ordinal)
for($i=0;$i -lt $sortedTitles.Count;$i++){if($azTitles[$i] -cne $sortedTitles[$i]){throw "Experiment index A-Z section is not in ordinal title order at position $($i+1): expected '$($sortedTitles[$i])', found '$($azTitles[$i])'."}}
$knownKeys=[Collections.Generic.HashSet[string]]::new([string[]]@(@($records.type)+@($records.node_refs | ForEach-Object {$_})+@($records.vocabulary_refs | ForEach-Object {$_})+@($records.session_refs | ForEach-Object {$_})),[StringComparer]::Ordinal)
foreach($match in [regex]::Matches($text,'(?m)^### `(?<id>[^`]*)` — ')){if(-not $knownKeys.Contains($match.Groups['id'].Value)){throw "Experiment index contains an unresolved grouping heading: $($match.Groups['id'].Value)"}}
foreach($view in @('type','node_refs','vocabulary_refs','session_refs')){
    foreach($key in @($records.$view | ForEach-Object {$_} | Sort-Object -Unique)){
        $members=@($records | Where-Object {$key -cin @($_.$view)})
        $noun=if($members.Count -eq 1){'experiment'}else{'experiments'}
        $heading="### ``$key`` — $($members.Count) $noun"
        $start=$text.IndexOf($heading,[StringComparison]::Ordinal);if($start -lt 0){throw "Experiment index is missing or miscounts grouping for $key"}
        $next=$text.IndexOf("`n### ",$start+$heading.Length,[StringComparison]::Ordinal);$nextSection=$text.IndexOf("`n## ",$start+$heading.Length,[StringComparison]::Ordinal)
        if($next -lt 0 -or ($nextSection -ge 0 -and $nextSection -lt $next)){$next=$nextSection};if($next -lt 0){$next=$text.Length}
        $body=$text.Substring($start,$next-$start)
        foreach($member in $members){$line="- ``$($member.id)`` — $($member.title)";if(-not $body.Contains($line)){throw "Experiment index grouping '$key' omits $($member.id)"}}
        $listedInGroup=[regex]::Matches($body,'(?m)^- `(?<id>[a-z0-9-]+)` — ').Count;if($listedInGroup -ne $members.Count){throw "Experiment index grouping '$key' lists $listedInGroup records; expected $($members.Count)"}
    }
}
$temp=Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-experiments-'+[guid]::NewGuid().ToString('N')+'.md')
try { & (Join-Path $PSScriptRoot 'build-experiment-index.ps1') -ExperimentDirectory $ExperimentDirectory -OutputPath $temp | Out-Null; if([Convert]::ToBase64String([IO.File]::ReadAllBytes($temp)) -cne [Convert]::ToBase64String([IO.File]::ReadAllBytes($IndexPath))){throw 'Generated experiment index is stale.'} } finally {if(Test-Path $temp){Remove-Item -LiteralPath $temp -Force}}
Write-Output "experiments: $($records.Count)"; foreach($type in $validType){Write-Output "  ${type}: $(@($records | Where-Object type -ceq $type).Count)"}
Write-Output "experiment_node_refs: $nodeRefs";Write-Output "experiment_vocabulary_refs: $vocabRefs";Write-Output "experiment_session_refs: $sessionRefs";Write-Output "experiment_source_refs: $sourceRefs";Write-Output "related_experiment_refs: $relatedRefs"
Write-Output 'unresolved_experiment_refs: 0';Write-Output 'duplicate_experiment_ids: 0';Write-Output 'experiment_index_reconciled: true';Write-Output 'experiment_index_structure_verified: true';Write-Output 'experiment_index_current: true'
