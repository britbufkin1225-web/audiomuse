[CmdletBinding()]
param(
    [string]$VocabularyDirectory,
    [string]$IndexPath
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not $VocabularyDirectory) { $VocabularyDirectory = Join-Path $repoRoot 'vocabulary/entries' }
if (-not $IndexPath) { $IndexPath = Join-Path $repoRoot 'vocabulary/index.md' }
$required = @('id','term','domain','definition','digital_relationship','best_use','technologies','node_refs','session_refs','related_terms','tags')
$entries = [Collections.Generic.List[object]]::new()
foreach ($file in @(Get-ChildItem -LiteralPath $VocabularyDirectory -Filter '*.yaml' -File | Sort-Object Name)) {
    $documents = [regex]::Split([IO.File]::ReadAllText($file.FullName), '(?m)^---\r?\n') | Where-Object { $_.Trim() }
    foreach ($document in $documents) {
        $values = [ordered]@{}
        foreach ($line in ($document.Trim() -split '\r?\n')) {
            if ($line -notmatch '^(?<key>[a-z_]+): (?<value>.+)$') { throw "Malformed vocabulary line in $($file.Name): $line" }
            $key = $Matches.key
            if ($key -notin $required -or $values.Contains($key)) { throw "Unsupported or duplicate vocabulary field '$key' in $($file.Name)" }
            try { $values[$key] = $Matches.value | ConvertFrom-Json -NoEnumerate } catch { throw "Vocabulary field '$key' must use a JSON-compatible YAML value in $($file.Name)" }
        }
        if (@($values.Keys).Count -ne $required.Count -or @($required | Where-Object { -not $values.Contains($_) }).Count) { throw "Missing required vocabulary field in $($file.Name)" }
        foreach ($field in @('id','term','domain','definition','digital_relationship','best_use')) { if ($values[$field] -isnot [string] -or -not $values[$field].Trim()) { throw "Vocabulary field '$field' must be a non-empty string in $($file.Name)" } }
        foreach ($field in @('technologies','node_refs','session_refs','related_terms','tags')) { if ($values[$field] -isnot [array]) { throw "Vocabulary field '$field' must be an array in $($file.Name)" } }
        if ($values.id -notmatch '^[a-z0-9]+(?:-[a-z0-9]+)*$') { throw "Invalid vocabulary id: $($values.id)" }
        $entries.Add([pscustomobject]$values)
    }
}
if ($entries.Count -eq 0) { throw 'No canonical vocabulary entries found.' }
$ids = @{}; $terms = @{}
foreach ($entry in $entries) {
    if ($ids.ContainsKey($entry.id)) { throw "Duplicate vocabulary id: $($entry.id)" }; $ids[$entry.id] = $true
    $termKey = $entry.term.ToLowerInvariant(); if ($terms.ContainsKey($termKey)) { throw "Duplicate canonical vocabulary term: $($entry.term)" }; $terms[$termKey] = $true
}
$domainText = [IO.File]::ReadAllText((Join-Path $repoRoot 'schemas/node.schema.yaml'))
$domainMatch = [regex]::Match($domainText, '(?ms)^  domain:\r?\n(?<body>.*?)^  status:')
if (-not $domainMatch.Success) { throw 'Could not parse canonical domain vocabulary.' }
$domainBlock = $domainMatch.Groups['body'].Value
$validDomains = @([regex]::Matches($domainBlock, '(?m)^      - (?<id>[a-z0-9-]+)\r?$') | ForEach-Object { $_.Groups['id'].Value })
$nodeIds = @{}; Get-ChildItem (Join-Path $repoRoot 'nodes') -Recurse -Filter '*.md' | Where-Object Name -ne 'README.md' | ForEach-Object { $m=[regex]::Match([IO.File]::ReadAllText($_.FullName),'(?m)^id: (?<id>[a-z0-9-]+)\r?$'); if($m.Success){$nodeIds[$m.Groups['id'].Value]=$true} }
$sessionIds = @{}; [regex]::Matches([IO.File]::ReadAllText((Join-Path $repoRoot 'sources/source-registry.yaml')), '(?m)^  - id: (?<id>session-[a-z0-9-]+)\r?$') | ForEach-Object { $sessionIds[$_.Groups['id'].Value]=$true }
$nodeRefs=0; $sessionRefs=0; $relatedRefs=0
foreach ($entry in $entries) {
    if ($entry.domain -notin $validDomains) { throw "Invalid vocabulary domain for $($entry.id): $($entry.domain)" }
    foreach ($field in @('technologies','node_refs','session_refs','related_terms','tags')) {
        $seen=@{}; foreach($value in $entry.$field) { if($value -isnot [string] -or -not $value.Trim()){throw "Empty or non-string $field value for $($entry.id)"}; if($seen.ContainsKey($value)){throw "Duplicate $field reference for $($entry.id): $value"}; $seen[$value]=$true }
    }
    foreach ($ref in $entry.node_refs) { if (-not $nodeIds.ContainsKey($ref)) { throw "Unresolved node reference for $($entry.id): $ref" }; $nodeRefs++ }
    foreach ($ref in $entry.session_refs) { if (-not $sessionIds.ContainsKey($ref)) { throw "Unresolved session reference for $($entry.id): $ref" }; $sessionRefs++ }
    foreach ($ref in $entry.related_terms) { if ($ref -eq $entry.id) { throw "Self-related vocabulary term: $($entry.id)" }; if (-not $ids.ContainsKey($ref)) { throw "Unresolved related term for $($entry.id): $ref" }; $relatedRefs++ }
}
$temp = Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-vocabulary-' + [guid]::NewGuid().ToString('N') + '.md')
try {
    & (Join-Path $PSScriptRoot 'build-vocabulary-index.ps1') -VocabularyDirectory $VocabularyDirectory -OutputPath $temp | Out-Null
    if (-not (Test-Path -LiteralPath $IndexPath)) { throw 'Missing generated vocabulary index.' }
    if ([Convert]::ToBase64String([IO.File]::ReadAllBytes($temp)) -cne [Convert]::ToBase64String([IO.File]::ReadAllBytes($IndexPath))) { throw 'Generated vocabulary index is stale.' }
} finally { if (Test-Path $temp) { Remove-Item -LiteralPath $temp -Force } }
Write-Output "vocabulary_entries: $($entries.Count)"
foreach ($domain in ($entries.domain | Sort-Object -Unique)) { Write-Output "  ${domain}: $(@($entries | Where-Object domain -eq $domain).Count)" }
Write-Output "vocabulary_node_refs: $nodeRefs"
Write-Output "vocabulary_session_refs: $sessionRefs"
Write-Output "vocabulary_related_term_refs: $relatedRefs"
Write-Output 'unresolved_vocabulary_refs: 0'
Write-Output 'duplicate_vocabulary_ids: 0'
Write-Output 'duplicate_vocabulary_terms: 0'
Write-Output 'vocabulary_index_current: true'
