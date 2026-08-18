[CmdletBinding()]
param(
    [string]$OutputDirectory
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $repoRoot 'indexes' }

function Get-OrdinalSortedStrings($Values) {
    $items = [Collections.Generic.List[string]]::new()
    foreach ($value in $Values) { $items.Add([string]$value) }
    $result = $items.ToArray()
    [Array]::Sort($result, [StringComparer]::Ordinal)
    return $result
}

function Get-YamlScalar([string]$Yaml, [string]$Name, [string]$Pattern = '[^\r\n]+') {
    $match = [regex]::Match($Yaml, "(?m)^${Name}: (?<value>${Pattern})\r?$")
    if (-not $match.Success) { throw "Missing or invalid '$Name' field." }
    return $match.Groups['value'].Value.Trim().Trim('"')
}

function Get-YamlList([string]$Yaml, [string]$Name) {
    if ($Yaml -match "(?m)^${Name}: \[\]\r?$") { return @() }
    $match = [regex]::Match($Yaml, "(?m)^${Name}:\r?\n(?<body>(?:  - [^\r\n]+\r?\n?)*)")
    if (-not $match.Success) { throw "Missing or malformed '$Name' list." }
    return @([regex]::Matches($match.Groups['body'].Value, '(?m)^  - (?<value>[^\r\n]+)\r?$') |
        ForEach-Object { $_.Groups['value'].Value.Trim('"') })
}

function Get-FrontMatter([System.IO.FileInfo]$File) {
    $text = Get-Content -Raw -LiteralPath $File.FullName
    $match = [regex]::Match($text, '(?s)\A---\r?\n(?<yaml>.*?)\r?\n---(?:\r?\n|\z)')
    if (-not $match.Success) { throw "Invalid YAML front matter: $($File.FullName)" }
    return $match.Groups['yaml'].Value
}

function Get-DisplayName([string]$Id) {
    $words = @($Id.Split('-') | ForEach-Object {
        if ($_.Length -eq 0) { return }
        [char]::ToUpperInvariant($_[0]) + $_.Substring(1)
    })
    return ($words -join ' ')
}

function Write-GeneratedFile([string]$Name, [System.Collections.Generic.List[string]]$Lines) {
    $path = Join-Path $OutputDirectory $Name
    $content = (($Lines -join "`n").TrimEnd() + "`n")
    [IO.File]::WriteAllText($path, $content, [Text.UTF8Encoding]::new($false))
}

$nodeFiles = @(Get-ChildItem (Join-Path $repoRoot 'nodes') -Recurse -Filter '*.md' |
    Where-Object Name -ne 'README.md')
$vocabularyText = Get-Content -Raw -LiteralPath (Join-Path $repoRoot 'schemas/relationship-types.yaml')
$validTypes = Get-OrdinalSortedStrings ([regex]::Matches($vocabularyText, '(?m)^  - id: (?<id>[a-z0-9_]+)\r?$') |
    ForEach-Object { $_.Groups['id'].Value })
if ($validTypes.Count -eq 0) { throw 'No canonical relationship types found.' }

$nodes = @{}
foreach ($file in $nodeFiles) {
    $yaml = Get-FrontMatter $file
    if ($yaml -match '(?m)^related_nodes:') { throw "Legacy related_nodes field remains: $($file.FullName)" }
    $id = Get-YamlScalar $yaml 'id' '[a-z0-9]+(?:-[a-z0-9]+)*'
    if ($nodes.ContainsKey($id)) { throw "Duplicate node id: $id" }
    $title = Get-YamlScalar $yaml 'title'
    $domain = Get-YamlScalar $yaml 'domain' '[a-z0-9]+(?:-[a-z0-9]+)*'
    $sessions = @(Get-YamlList $yaml 'session_origin')
    $sources = @(Get-YamlList $yaml 'sources')

    $edges = @()
    if ($yaml -notmatch '(?m)^relationships: \[\]\r?$') {
        $block = [regex]::Match($yaml, '(?m)^relationships:\r?\n(?<body>(?:  [^\r\n]*\r?\n?)*)')
        if (-not $block.Success) { throw "Missing relationships array: $id" }
        $matches = [regex]::Matches($block.Groups['body'].Value,
            '(?m)^  - target: (?<target>[a-z0-9]+(?:-[a-z0-9]+)*)\r?\n    type: (?<type>[a-z0-9_]+)\r?$')
        $entries = [regex]::Matches($block.Groups['body'].Value, '(?m)^  - target:').Count
        if ($matches.Count -ne $entries) { throw "Malformed relationship entry: $id" }
        $edges = @($matches | ForEach-Object {
            [pscustomobject]@{ Target = $_.Groups['target'].Value; Type = $_.Groups['type'].Value }
        })
    }
    $nodes[$id] = [pscustomobject]@{
        Id = $id; Title = $title; Domain = $domain; Sessions = $sessions; Sources = $sources; Edges = $edges
    }
}

$sourceRegistryPath = Join-Path $repoRoot 'sources/source-registry.yaml'
$sourceRegistryText = Get-Content -Raw -LiteralPath $sourceRegistryPath
$sourceEntries = [regex]::Matches($sourceRegistryText,
    '(?ms)^  - id: (?<id>[a-z0-9]+(?:-[a-z0-9]+)*)\r?\n    type: (?<type>[^\r\n]+)\r?\n    title: (?<title>[^\r\n]+)\r?\n    locator: (?<locator>[^\r\n]+)')
$sources = @{}
foreach ($entry in $sourceEntries) {
    $sourceId = $entry.Groups['id'].Value
    if ($sources.ContainsKey($sourceId)) { throw "Duplicate source id: $sourceId" }
    $locator = $entry.Groups['locator'].Value.Trim('"')
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot $locator))) { throw "Source locator does not resolve: $sourceId" }
    $sources[$sourceId] = [pscustomobject]@{
        Id = $sourceId
        Type = $entry.Groups['type'].Value.Trim()
        Title = $entry.Groups['title'].Value.Trim('"')
        Locator = $locator
    }
}
if ($sources.Count -eq 0) { throw 'No source registry entries found.' }

$edgeList = [Collections.Generic.List[object]]::new()
$sessionToNodes = @{}
$sourceToNodes = @{}
foreach ($nodeId in (Get-OrdinalSortedStrings $nodes.Keys)) {
    $node = $nodes[$nodeId]
    foreach ($sessionId in $node.Sessions) {
        if (-not $sources.ContainsKey($sessionId)) { throw "Node $nodeId references unresolved session: $sessionId" }
        if ($sources[$sessionId].Type -ne 'session') { throw "Node $nodeId session_origin references a non-session source: $sessionId" }
        if (-not $sessionToNodes.ContainsKey($sessionId)) { $sessionToNodes[$sessionId] = [Collections.Generic.List[string]]::new() }
        $sessionToNodes[$sessionId].Add($nodeId)
    }
    foreach ($sourceId in $node.Sources) {
        if (-not $sources.ContainsKey($sourceId)) { throw "Node $nodeId references unresolved source: $sourceId" }
        if (-not $sourceToNodes.ContainsKey($sourceId)) { $sourceToNodes[$sourceId] = [Collections.Generic.List[string]]::new() }
        $sourceToNodes[$sourceId].Add($nodeId)
    }
    $seenEdges = @{}
    foreach ($edge in $node.Edges) {
        if (-not $nodes.ContainsKey($edge.Target)) { throw "Node $nodeId has unresolved relationship target: $($edge.Target)" }
        if ($edge.Type -notin $validTypes) { throw "Node $nodeId has invalid relationship type: $($edge.Type)" }
        if ($edge.Target -eq $nodeId) { throw "Node $nodeId has a self-link." }
        $key = "$($edge.Type)|$($edge.Target)"
        if ($seenEdges.ContainsKey($key)) { throw "Node $nodeId has duplicate relationship: $key" }
        $seenEdges[$key] = $true
        $edgeList.Add([pscustomobject]@{ Source = $nodeId; Target = $edge.Target; Type = $edge.Type })
    }
}

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$nodeIds = Get-OrdinalSortedStrings $nodes.Keys
$domains = Get-OrdinalSortedStrings ($nodes.Values.Domain | Select-Object -Unique)
$representedTypes = Get-OrdinalSortedStrings ($edgeList.Type | Select-Object -Unique)
$sessionIds = Get-OrdinalSortedStrings $sessionToNodes.Keys
$sourceIds = Get-OrdinalSortedStrings $sources.Keys

$lines = [Collections.Generic.List[string]]::new()
$lines.Add('# Nodes by Domain'); $lines.Add('')
$lines.Add("Canonical nodes: $($nodes.Count)"); $lines.Add("Domains represented: $($domains.Count)")
foreach ($domain in $domains) {
    $members = @($nodeIds | Where-Object { $nodes[$_].Domain -eq $domain })
    $lines.Add(''); $lines.Add("## $(Get-DisplayName $domain) — $($members.Count) nodes"); $lines.Add('')
    foreach ($id in $members) { $lines.Add(('- **{0}** (`{1}`)' -f $nodes[$id].Title, $id)) }
}
Write-GeneratedFile 'nodes-by-domain.md' $lines

$lines = [Collections.Generic.List[string]]::new()
$lines.Add('# Relationships by Type'); $lines.Add('')
$lines.Add("Canonical relationships: $($edgeList.Count)"); $lines.Add("Relationship types represented: $($representedTypes.Count)")
foreach ($type in $representedTypes) {
    $typedEdges = @($edgeList | Where-Object Type -eq $type | ForEach-Object { "$($_.Source)|$($_.Target)" })
    $typedEdges = Get-OrdinalSortedStrings $typedEdges
    $lines.Add(''); $lines.Add(('## `{0}` — {1} relationships' -f $type, $typedEdges.Count)); $lines.Add('')
    foreach ($edgeKey in $typedEdges) {
        $parts = $edgeKey.Split('|'); $lines.Add(('- `{0}` → `{1}`' -f $parts[0], $parts[1]))
    }
}
Write-GeneratedFile 'relationships-by-type.md' $lines

$lines = [Collections.Generic.List[string]]::new()
$lines.Add('# Node Connections'); $lines.Add('')
$lines.Add("Canonical relationships: $($edgeList.Count)"); $lines.Add("Outbound total: $($edgeList.Count)"); $lines.Add("Inbound total: $($edgeList.Count)")
foreach ($nodeId in $nodeIds) {
    $outbound = Get-OrdinalSortedStrings @($edgeList | Where-Object Source -eq $nodeId | ForEach-Object { "$($_.Type)|$($_.Target)" })
    $inbound = Get-OrdinalSortedStrings @($edgeList | Where-Object Target -eq $nodeId | ForEach-Object { "$($_.Source)|$($_.Type)" })
    $lines.Add(''); $lines.Add(('## {0} (`{1}`)' -f $nodes[$nodeId].Title, $nodeId)); $lines.Add('')
    $lines.Add("Outbound: $($outbound.Count)"); $lines.Add("Inbound: $($inbound.Count)"); $lines.Add('')
    $lines.Add('### Outbound')
    if ($outbound.Count -eq 0) { $lines.Add(''); $lines.Add('- None') } else {
        $lines.Add(''); foreach ($item in $outbound) { $p = $item.Split('|'); $lines.Add(('- `{0}` via `{1}`' -f $p[1], $p[0])) }
    }
    $lines.Add(''); $lines.Add('### Inbound')
    if ($inbound.Count -eq 0) { $lines.Add(''); $lines.Add('- None') } else {
        $lines.Add(''); foreach ($item in $inbound) { $p = $item.Split('|'); $lines.Add(('- `{0}` via `{1}`' -f $p[0], $p[1])) }
    }
}
Write-GeneratedFile 'node-connections.md' $lines

$lines = [Collections.Generic.List[string]]::new()
$lines.Add('# Session Coverage'); $lines.Add('')
$lines.Add("Sessions represented: $($sessionIds.Count)"); $lines.Add("Nodes covered: $(@($nodeIds | Where-Object { $nodes[$_].Sessions.Count -gt 0 }).Count) of $($nodes.Count)")
$lines.Add(''); $lines.Add('## Session → Nodes')
foreach ($sessionId in $sessionIds) {
    $members = Get-OrdinalSortedStrings $sessionToNodes[$sessionId]
    $lines.Add(''); $lines.Add(('### {0} (`{1}`) — {2} nodes' -f $sources[$sessionId].Title, $sessionId, $members.Count)); $lines.Add('')
    foreach ($id in $members) { $lines.Add(('- `{0}` — {1}' -f $id, $nodes[$id].Title)) }
}
$lines.Add(''); $lines.Add('## Node → Sessions')
foreach ($nodeId in $nodeIds) {
    $members = Get-OrdinalSortedStrings $nodes[$nodeId].Sessions
    $lines.Add(''); $lines.Add(('### {0} (`{1}`) — {2} sessions' -f $nodes[$nodeId].Title, $nodeId, $members.Count)); $lines.Add('')
    if ($members.Count -eq 0) { $lines.Add('- None') } else { foreach ($id in $members) { $lines.Add(('- `{0}` — {1}' -f $id, $sources[$id].Title)) } }
}
Write-GeneratedFile 'session-coverage.md' $lines

$zeroSourceNodes = @($nodeIds | Where-Object { $nodes[$_].Sources.Count -eq 0 })
$sharedSources = @($sourceIds | Where-Object { $sourceToNodes.ContainsKey($_) -and $sourceToNodes[$_].Count -gt 1 })
$lines = [Collections.Generic.List[string]]::new()
$lines.Add('# Source / Provenance Coverage'); $lines.Add('')
$lines.Add("Registered sources: $($sources.Count)"); $lines.Add("Sources referenced by nodes: $($sourceToNodes.Count)"); $lines.Add("Nodes with source coverage: $($nodes.Count - $zeroSourceNodes.Count) of $($nodes.Count)")
$lines.Add(''); $lines.Add('## Coverage by Node')
foreach ($nodeId in $nodeIds) {
    $members = Get-OrdinalSortedStrings $nodes[$nodeId].Sources
    $lines.Add(''); $lines.Add(('### {0} (`{1}`) — {2} sources' -f $nodes[$nodeId].Title, $nodeId, $members.Count)); $lines.Add('')
    if ($members.Count -eq 0) { $lines.Add('- None') } else { foreach ($id in $members) { $lines.Add(('- `{0}` — {1}' -f $id, $sources[$id].Title)) } }
}
$lines.Add(''); $lines.Add("## Nodes with No Source Coverage — $($zeroSourceNodes.Count)"); $lines.Add('')
if ($zeroSourceNodes.Count -eq 0) { $lines.Add('- None') } else { foreach ($id in $zeroSourceNodes) { $lines.Add(('- `{0}` — {1}' -f $id, $nodes[$id].Title)) } }
$lines.Add(''); $lines.Add("## Sources Referenced by Multiple Nodes — $($sharedSources.Count)")
foreach ($sourceId in $sharedSources) {
    $members = Get-OrdinalSortedStrings $sourceToNodes[$sourceId]
    $lines.Add(''); $lines.Add(('### {0} (`{1}`) — {2} nodes' -f $sources[$sourceId].Title, $sourceId, $members.Count)); $lines.Add('')
    foreach ($id in $members) { $lines.Add(('- `{0}` — {1}' -f $id, $nodes[$id].Title)) }
}
Write-GeneratedFile 'source-coverage.md' $lines

$lines = [Collections.Generic.List[string]]::new()
$lines.Add('# AudioMuse Knowledge Index'); $lines.Add('')
$lines.Add('This directory contains generated, read-only views of canonical AudioMuse repository content. Node, session, schema, and source/provenance files remain authoritative. These indexes are navigation and audit conveniences—not a database or a duplicate knowledge store.')
$lines.Add(''); $lines.Add('Do not edit these files manually. Regenerate and validate them from the repository root:')
$lines.Add(''); $lines.Add('```powershell'); $lines.Add('.\tools\build-knowledge-index.ps1'); $lines.Add('.\tools\validate-knowledge-index.ps1'); $lines.Add('```')
$lines.Add(''); $lines.Add('## Summary'); $lines.Add('')
$lines.Add("- Nodes: $($nodes.Count)"); $lines.Add("- Relationships: $($edgeList.Count)"); $lines.Add("- Relationship types represented: $($representedTypes.Count)"); $lines.Add("- Sessions represented: $($sessionIds.Count)"); $lines.Add("- Registered sources: $($sources.Count)"); $lines.Add("- Sources referenced by nodes: $($sourceToNodes.Count)"); $lines.Add("- Domains represented: $($domains.Count)")
$lines.Add(''); $lines.Add('## Views'); $lines.Add('')
$lines.Add('- `nodes-by-domain.md` groups canonical nodes by canonical domain metadata.')
$lines.Add('- `relationships-by-type.md` groups explicit directed edges by canonical type.')
$lines.Add('- `node-connections.md` shows typed outbound and inbound navigation without synthesizing reverse edges.')
$lines.Add('- `session-coverage.md` shows the many-to-many session-to-node contribution map in both directions.')
$lines.Add('- `source-coverage.md` reports provenance presence and reuse without scoring source quality.')
Write-GeneratedFile 'README.md' $lines

Write-Output "Generated 6 knowledge index files from $($nodes.Count) nodes and $($edgeList.Count) relationships."
