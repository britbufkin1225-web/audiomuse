[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$nodeFiles = Get-ChildItem (Join-Path $repoRoot 'nodes') -Recurse -Filter '*.md' |
    Where-Object Name -ne 'README.md' | Sort-Object FullName
$vocabularyPath = Join-Path $repoRoot 'schemas/relationship-types.yaml'
$requiredFields = @(
    'id', 'title', 'domain', 'status', 'session_origin', 'definition',
    'core_concepts', 'relationships', 'sources', 'experiments',
    'practical_applications', 'project_connections', 'future_questions'
)

$vocabularyText = Get-Content -Raw $vocabularyPath
$validTypes = [regex]::Matches($vocabularyText, '(?m)^  - id: ([a-z0-9_]+)\r?$') |
    ForEach-Object { $_.Groups[1].Value }
if (-not $validTypes) { throw 'No relationship types found in canonical vocabulary.' }

$nodes = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::Ordinal)
$parsed = @()
foreach ($file in $nodeFiles) {
    $text = Get-Content -Raw $file.FullName
    if ($text -notmatch '(?s)\A---\r?\n(?<yaml>.*?)\r?\n---(?:\r?\n|\z)') {
        throw "Invalid YAML front matter delimiters: $($file.FullName)"
    }
    $yaml = $Matches.yaml
    $topLevelFields = [regex]::Matches($yaml, '(?m)^(?<key>[a-z_]+):') |
        ForEach-Object { $_.Groups['key'].Value }
    foreach ($field in $requiredFields) {
        if ($field -notin $topLevelFields) { throw "Missing required field '$field': $($file.FullName)" }
    }
    foreach ($field in $topLevelFields) {
        if ($field -notin $requiredFields) { throw "Unknown top-level field '$field': $($file.FullName)" }
    }
    if (($topLevelFields | Select-Object -Unique).Count -ne $topLevelFields.Count) {
        throw "Duplicate top-level field: $($file.FullName)"
    }
    if ($yaml -match '(?m)^related_nodes:') { throw "Legacy related_nodes field remains: $($file.FullName)" }
    $idMatch = [regex]::Match($yaml, '(?m)^id: ([a-z0-9]+(?:-[a-z0-9]+)*)\r?$')
    if (-not $idMatch.Success) { throw "Missing or invalid node id: $($file.FullName)" }
    $id = $idMatch.Groups[1].Value
    if ($nodes.ContainsKey($id)) { throw "Duplicate node id: $id" }
    $nodes[$id] = $file.FullName

    if ($yaml -match '(?m)^relationships: \[\]\r?$') {
        $edges = @()
    } else {
        $relationshipBlock = [regex]::Match($yaml, '(?m)^relationships:\r?\n(?<body>(?:  [^\r\n]*\r?\n?)*)')
        if (-not $relationshipBlock.Success) { throw "Missing relationships array: $id" }
        $body = $relationshipBlock.Groups['body'].Value
        $edges = [regex]::Matches($body,
            '(?m)^  - target: (?<target>[a-z0-9]+(?:-[a-z0-9]+)*)\r?\n    type: (?<type>[a-z0-9_]+)\r?$')
        $edgeLines = [regex]::Matches($body, '(?m)^  - target:').Count
        if ($edgeLines -eq 0) { throw "Empty relationship list must be written as 'relationships: []': $id" }
        if ($edges.Count -ne $edgeLines) { throw "Malformed relationship entry in node: $id" }
    }
    $parsed += [pscustomobject]@{ Id = $id; Edges = $edges; File = $file.FullName }
}

$violations = @()
$unresolved = 0
$invalidTypes = 0
$selfLinks = 0
$duplicates = 0
$edgeCount = 0
$counts = [Collections.Generic.Dictionary[string,int]]::new([StringComparer]::Ordinal)
foreach ($node in $parsed) {
    $seen = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($edge in $node.Edges) {
        $edgeCount++
        $target = $edge.Groups['target'].Value
        $type = $edge.Groups['type'].Value
        if (-not $nodes.ContainsKey($target)) { $unresolved++; $violations += "$($node.Id): unresolved target $target" }
        if ($type -cnotin $validTypes) { $invalidTypes++; $violations += "$($node.Id): invalid type $type" }
        if ($target -ceq $node.Id) { $selfLinks++; $violations += "$($node.Id): self-link" }
        $key = "$type|$target"
        if (-not $seen.Add($key)) { $duplicates++; $violations += "$($node.Id): duplicate edge $key" }
        if (-not $counts.ContainsKey($type)) { $counts[$type] = 0 }
        $counts[$type]++
    }
}

foreach ($violation in $violations) { Write-Output "violation: $violation" }

Write-Output "nodes: $($nodes.Count)"
Write-Output "typed_relationships: $edgeCount"
foreach ($type in ($counts.Keys | Sort-Object)) { Write-Output "  ${type}: $($counts[$type])" }
Write-Output "unresolved_targets: $unresolved"
Write-Output "invalid_relationship_types: $invalidTypes"
Write-Output "duplicate_edges: $duplicates"
Write-Output "self_links: $selfLinks"

if ($unresolved + $invalidTypes + $selfLinks + $duplicates -ne 0) { exit 1 }
