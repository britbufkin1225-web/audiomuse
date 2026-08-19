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

function Get-DisplayLabel([string]$Value) {
    return (@($Value.Split('-') | Where-Object { $_.Length } |
        ForEach-Object { [char]::ToUpperInvariant($_[0]) + $_.Substring(1) }) -join ' ')
}

# Independent structural audit of the committed projection. This deliberately does NOT call the
# builder: every expected line is re-derived from canonical entries so a builder formatting or
# escaping defect cannot be masked by regenerating output with the same faulty logic.
function Assert-VocabularyIndexStructure([string]$Path, [object[]]$Entries) {
    $text = [IO.File]::ReadAllText($Path)
    foreach ($marker in @('System.Object[]', '$(')) {
        if ($text.Contains($marker)) { throw "Vocabulary index contains unexpanded template output ('$marker'); the builder emitted raw object text instead of markdown." }
    }
    $present = [Collections.Generic.HashSet[string]]::new([string[]]($text -split '\r?\n'), [StringComparer]::Ordinal)
    $expected = [Collections.Generic.List[string]]::new()
    foreach ($heading in @('# AudioMuse Vocabulary Index','## A-Z','## By Domain','## By Session','## By Canonical Node')) { $expected.Add($heading) }
    $expected.Add("Canonical vocabulary entries: $($Entries.Count)")
    foreach ($entry in $Entries) { $expected.Add(('- **{0}** (`{1}`) — {2}' -f $entry.term, $entry.id, $entry.definition)) }
    foreach ($domain in @($Entries.domain | Select-Object -Unique)) {
        $members = @($Entries | Where-Object domain -ceq $domain)
        $expected.Add(('### {0} — {1} terms' -f (Get-DisplayLabel $domain), $members.Count))
        foreach ($entry in $members) { $expected.Add(('- `{0}` — {1}' -f $entry.id, $entry.term)) }
    }
    foreach ($sessionId in @($Entries.session_refs | ForEach-Object { $_ } | Select-Object -Unique)) {
        $expected.Add(('### `{0}` — {1} terms' -f $sessionId, @($Entries | Where-Object { $sessionId -cin $_.session_refs }).Count))
    }
    foreach ($nodeId in @($Entries.node_refs | ForEach-Object { $_ } | Select-Object -Unique)) {
        $expected.Add(('### `{0}` — {1} terms' -f $nodeId, @($Entries | Where-Object { $nodeId -cin $_.node_refs }).Count))
    }
    foreach ($line in $expected) { if (-not $present.Contains($line)) { throw "Vocabulary index is missing expected line: $line" } }
    # A-Z ordering is asserted against an independently sorted term list, not against builder output.
    $azTerms = @([regex]::Matches($text, '(?m)^- \*\*(?<term>.+?)\*\* \(') | ForEach-Object { $_.Groups['term'].Value })
    if ($azTerms.Count -ne $Entries.Count) { throw "Vocabulary index A-Z section lists $($azTerms.Count) terms but $($Entries.Count) canonical entries exist." }
    $sortedTerms = [string[]]@($azTerms); [Array]::Sort($sortedTerms, [StringComparer]::Ordinal)
    for ($i = 0; $i -lt $sortedTerms.Count; $i++) { if ($azTerms[$i] -cne $sortedTerms[$i]) { throw "Vocabulary index A-Z section is not in ordinal term order at position $($i + 1): expected '$($sortedTerms[$i])', found '$($azTerms[$i])'." } }
    $known = [Collections.Generic.HashSet[string]]::new([string[]]@(@($Entries.session_refs | ForEach-Object { $_ }) + @($Entries.node_refs | ForEach-Object { $_ })), [StringComparer]::Ordinal)
    foreach ($match in [regex]::Matches($text, '(?m)^### `(?<id>[^`]*)` — ')) {
        if (-not $known.Contains($match.Groups['id'].Value)) { throw "Vocabulary index contains an unresolved grouping heading: $($match.Groups['id'].Value)" }
    }
}

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
$ids = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); $terms = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
foreach ($entry in $entries) {
    if (-not $ids.Add($entry.id)) { throw "Duplicate vocabulary id: $($entry.id)" }
    if (-not $terms.Add($entry.term)) { throw "Duplicate canonical vocabulary term: $($entry.term)" }
}
$domainText = [IO.File]::ReadAllText((Join-Path $repoRoot 'schemas/node.schema.yaml'))
$domainMatch = [regex]::Match($domainText, '(?ms)^  domain:\r?\n(?<body>.*?)^  status:')
if (-not $domainMatch.Success) { throw 'Could not parse canonical domain vocabulary.' }
$domainBlock = $domainMatch.Groups['body'].Value
$validDomains = @([regex]::Matches($domainBlock, '(?m)^      - (?<id>[a-z0-9-]+)\r?$') | ForEach-Object { $_.Groups['id'].Value })
$nodeIds = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); Get-ChildItem (Join-Path $repoRoot 'nodes') -Recurse -Filter '*.md' | Where-Object Name -ne 'README.md' | ForEach-Object { $m=[regex]::Match([IO.File]::ReadAllText($_.FullName),'(?m)^id: (?<id>[a-z0-9-]+)\r?$'); if($m.Success){[void]$nodeIds.Add($m.Groups['id'].Value)} }
# Session refs resolve by registered source TYPE, matching the Phase 5 rule that session lists may
# only name type: session sources. An id that merely starts with "session-" is not sufficient.
$sourceTypes = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::Ordinal); [regex]::Matches([IO.File]::ReadAllText((Join-Path $repoRoot 'sources/source-registry.yaml')), '(?m)^  - id: (?<id>[a-z0-9-]+)\r?\n    type: (?<type>[a-z0-9-]+)\r?$') | ForEach-Object { $sourceTypes[$_.Groups['id'].Value] = $_.Groups['type'].Value }
if (-not @($sourceTypes.Values | Where-Object { $_ -ceq 'session' }).Count) { throw 'Could not parse registered session sources.' }
$nodeRefs=0; $sessionRefs=0; $relatedRefs=0
foreach ($entry in $entries) {
    if ($entry.domain -cnotin $validDomains) { throw "Invalid vocabulary domain for $($entry.id): $($entry.domain)" }
    foreach ($field in @('technologies','node_refs','session_refs','related_terms','tags')) {
        $seen=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal); foreach($value in $entry.$field) { if($value -isnot [string] -or -not $value.Trim()){throw "Empty or non-string $field value for $($entry.id)"}; if(-not $seen.Add($value)){throw "Duplicate $field reference for $($entry.id): $value"} }
    }
    foreach ($ref in $entry.node_refs) { if (-not $nodeIds.Contains($ref)) { throw "Unresolved node reference for $($entry.id): $ref" }; $nodeRefs++ }
    foreach ($ref in $entry.session_refs) {
        if (-not $sourceTypes.ContainsKey($ref)) { throw "Unresolved session reference for $($entry.id): $ref" }
        if ($sourceTypes[$ref] -cne 'session') { throw "Vocabulary session_refs references a non-session source for $($entry.id): $ref (registered type: $($sourceTypes[$ref]))" }
        $sessionRefs++
    }
    foreach ($ref in $entry.related_terms) { if ($ref -ceq $entry.id) { throw "Self-related vocabulary term: $($entry.id)" }; if (-not $ids.Contains($ref)) { throw "Unresolved related term for $($entry.id): $ref" }; $relatedRefs++ }
}
if (-not (Test-Path -LiteralPath $IndexPath)) { throw 'Missing generated vocabulary index.' }
Assert-VocabularyIndexStructure $IndexPath @($entries)
$temp = Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-vocabulary-' + [guid]::NewGuid().ToString('N') + '.md')
try {
    & (Join-Path $PSScriptRoot 'build-vocabulary-index.ps1') -VocabularyDirectory $VocabularyDirectory -OutputPath $temp | Out-Null
    if ([Convert]::ToBase64String([IO.File]::ReadAllBytes($temp)) -cne [Convert]::ToBase64String([IO.File]::ReadAllBytes($IndexPath))) { throw 'Generated vocabulary index is stale.' }
} finally { if (Test-Path $temp) { Remove-Item -LiteralPath $temp -Force } }
Write-Output "vocabulary_entries: $($entries.Count)"
foreach ($domain in ($entries.domain | Sort-Object -Unique)) { Write-Output "  ${domain}: $(@($entries | Where-Object domain -ceq $domain).Count)" }
Write-Output "vocabulary_node_refs: $nodeRefs"
Write-Output "vocabulary_session_refs: $sessionRefs"
Write-Output "vocabulary_related_term_refs: $relatedRefs"
Write-Output 'unresolved_vocabulary_refs: 0'
Write-Output 'duplicate_vocabulary_ids: 0'
Write-Output 'duplicate_vocabulary_terms: 0'
Write-Output 'vocabulary_index_structure_verified: true'
Write-Output 'vocabulary_index_current: true'
