[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$indexDirectory = Join-Path $repoRoot 'indexes'
$expectedFiles = @(
    'README.md',
    'node-connections.md',
    'nodes-by-domain.md',
    'relationships-by-type.md',
    'session-coverage.md',
    'source-coverage.md'
)

$graphValidation = @(& (Join-Path $PSScriptRoot 'validate-graph.ps1'))
if (-not $?) { throw 'Canonical graph validation failed.' }
$graphValidation | Write-Output
$canonicalCountLine = @($graphValidation | Where-Object { $_ -match '^typed_relationships: \d+$' })
if ($canonicalCountLine.Count -ne 1) { throw 'Canonical graph validator did not report one relationship total.' }
$canonicalRelationshipCount = [int]($canonicalCountLine[0] -replace '^typed_relationships: ', '')

$temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) ("audiomuse-index-" + [guid]::NewGuid().ToString('N'))
try {
    & (Join-Path $PSScriptRoot 'build-knowledge-index.ps1') -OutputDirectory $temporaryDirectory
    foreach ($name in $expectedFiles) {
        $current = Join-Path $indexDirectory $name
        $generated = Join-Path $temporaryDirectory $name
        if (-not (Test-Path -LiteralPath $current)) { throw "Missing generated index: indexes/$name" }
        $currentBytes = [IO.File]::ReadAllBytes($current)
        $generatedBytes = [IO.File]::ReadAllBytes($generated)
        if ($currentBytes.Length -ne $generatedBytes.Length -or
            [Convert]::ToBase64String($currentBytes) -cne [Convert]::ToBase64String($generatedBytes)) {
            throw "Generated index is stale: indexes/$name"
        }
    }
    $unexpected = @(Get-ChildItem $indexDirectory -File | Where-Object Name -cnotin $expectedFiles)
    if ($unexpected.Count -gt 0) { throw "Unexpected generated index file: $($unexpected[0].Name)" }

    $connections = Get-Content -Raw -LiteralPath (Join-Path $indexDirectory 'node-connections.md')
    $relationshipIndex = Get-Content -Raw -LiteralPath (Join-Path $indexDirectory 'relationships-by-type.md')
    if ($connections -notmatch '(?m)^Canonical relationships: (?<edges>\d+)\r?$' -or
        $connections -notmatch '(?m)^Outbound total: (?<outbound>\d+)\r?$' -or
        $connections -notmatch '(?m)^Inbound total: (?<inbound>\d+)\r?$') {
        throw 'Node connection totals are missing or malformed.'
    }
    $edges = [int]([regex]::Match($connections, '(?m)^Canonical relationships: (\d+)').Groups[1].Value)
    $outbound = [int]([regex]::Match($connections, '(?m)^Outbound total: (\d+)').Groups[1].Value)
    $inbound = [int]([regex]::Match($connections, '(?m)^Inbound total: (\d+)').Groups[1].Value)
    $indexedEdges = [regex]::Matches($relationshipIndex, '(?m)^- `[a-z0-9-]+` → `[a-z0-9-]+`\r?$').Count
    if ($edges -ne $canonicalRelationshipCount -or $outbound -ne $canonicalRelationshipCount -or
        $inbound -ne $canonicalRelationshipCount -or $indexedEdges -ne $canonicalRelationshipCount) {
        throw "Edge totals do not reconcile: canonical=$canonicalRelationshipCount generated=$edges outbound=$outbound inbound=$inbound indexed=$indexedEdges"
    }

    # Independent reconciliation.
    #
    # Regenerating into a temporary directory and byte-comparing proves only that
    # the committed indexes match current generator output. It cannot detect a
    # generator that is consistently wrong, because the comparison would still
    # succeed. The checks below therefore re-derive the expected totals straight
    # from canonical node files, using their own parsing, and compare them with
    # what the generated views actually list.
    $canonicalNodeFiles = @(Get-ChildItem (Join-Path $repoRoot 'nodes') -Recurse -Filter '*.md' |
        Where-Object Name -ne 'README.md')
    if ($canonicalNodeFiles.Count -eq 0) { throw 'No canonical node files found.' }

    $canonicalNodeIds = [Collections.Generic.List[string]]::new()
    $canonicalDomains = [Collections.Generic.List[string]]::new()
    $canonicalSessionRefs = 0
    $canonicalSourceRefs = 0
    foreach ($file in $canonicalNodeFiles) {
        $text = [IO.File]::ReadAllText($file.FullName)
        $idMatch = [regex]::Match($text, '(?m)^id: (?<v>[a-z0-9]+(?:-[a-z0-9]+)*)\r?$')
        $domainMatch = [regex]::Match($text, '(?m)^domain: (?<v>[a-z0-9]+(?:-[a-z0-9]+)*)\r?$')
        if (-not $idMatch.Success -or -not $domainMatch.Success) {
            throw "Canonical node is missing a parsable id or domain: $($file.FullName)"
        }
        $canonicalNodeIds.Add($idMatch.Groups['v'].Value)
        $canonicalDomains.Add($domainMatch.Groups['v'].Value)
        foreach ($field in @('session_origin', 'sources')) {
            $count = 0
            if ($text -notmatch "(?m)^${field}: \[\]\r?$") {
                $block = [regex]::Match($text, "(?ms)^${field}:\r?\n(?<body>(?:[ ]{2}[^\r\n]*\r?\n?)*)")
                if (-not $block.Success) { throw "Canonical field '$field' is malformed: $($file.FullName)" }
                $count = ([regex]::Matches($block.Groups['body'].Value, '(?m)^  - [^\r\n]+\r?$')).Count
            }
            if ($field -eq 'session_origin') { $canonicalSessionRefs += $count } else { $canonicalSourceRefs += $count }
        }
    }
    $canonicalNodeCount = $canonicalNodeIds.Count
    $canonicalDomainCount = @($canonicalDomains | Select-Object -Unique).Count

    function Get-IndexSectionBullets([string]$Text, [string]$StartHeading, [string]$BulletPattern) {
        $lines = $Text -split '\r?\n'
        $inSection = ($StartHeading -eq '')
        $bullets = [Collections.Generic.List[string]]::new()
        foreach ($line in $lines) {
            if ($line -like '## *') { $inSection = ($line -eq $StartHeading); continue }
            if ($inSection -and $line -match $BulletPattern) { $bullets.Add($Matches[1]) }
        }
        return $bullets
    }

    $domainIndex = Get-Content -Raw -LiteralPath (Join-Path $indexDirectory 'nodes-by-domain.md')
    $sessionIndex = Get-Content -Raw -LiteralPath (Join-Path $indexDirectory 'session-coverage.md')
    $sourceIndex = Get-Content -Raw -LiteralPath (Join-Path $indexDirectory 'source-coverage.md')

    $declaredNodes = [int]([regex]::Match($domainIndex, '(?m)^Canonical nodes: (\d+)\r?$').Groups[1].Value)
    $declaredDomains = [int]([regex]::Match($domainIndex, '(?m)^Domains represented: (\d+)\r?$').Groups[1].Value)
    $listedNodeIds = @([regex]::Matches($domainIndex, '(?m)^- \*\*[^\r\n]*\*\* \(`(?<id>[a-z0-9-]+)`\)\r?$') |
        ForEach-Object { $_.Groups['id'].Value })
    $domainHeadings = ([regex]::Matches($domainIndex, '(?m)^## [^\r\n]+\r?$')).Count
    if ($declaredNodes -ne $canonicalNodeCount -or $listedNodeIds.Count -ne $canonicalNodeCount) {
        throw "Node totals do not reconcile: canonical=$canonicalNodeCount declared=$declaredNodes listed=$($listedNodeIds.Count)"
    }
    if ($declaredDomains -ne $canonicalDomainCount -or $domainHeadings -ne $canonicalDomainCount) {
        throw "Domain totals do not reconcile: canonical=$canonicalDomainCount declared=$declaredDomains headings=$domainHeadings"
    }
    $missingIds = @($canonicalNodeIds | Where-Object { $_ -cnotin $listedNodeIds })
    $extraIds = @($listedNodeIds | Where-Object { $_ -cnotin $canonicalNodeIds })
    if ($missingIds.Count -gt 0) { throw "Canonical node missing from nodes-by-domain.md: $($missingIds[0])" }
    if ($extraIds.Count -gt 0) { throw "nodes-by-domain.md lists a non-canonical node: $($extraIds[0])" }

    $sessionToNodeRefs = (Get-IndexSectionBullets $sessionIndex '## Session → Nodes' '^- `([a-z0-9-]+)` — ').Count
    $nodeToSessionRefs = (Get-IndexSectionBullets $sessionIndex '## Node → Sessions' '^- `([a-z0-9-]+)` — ').Count
    if ($sessionToNodeRefs -ne $canonicalSessionRefs -or $nodeToSessionRefs -ne $canonicalSessionRefs) {
        throw "Session coverage does not reconcile: canonical=$canonicalSessionRefs session_to_node=$sessionToNodeRefs node_to_session=$nodeToSessionRefs"
    }
    $nodeToSourceRefs = (Get-IndexSectionBullets $sourceIndex '## Coverage by Node' '^- `([a-z0-9-]+)` — ').Count
    if ($nodeToSourceRefs -ne $canonicalSourceRefs) {
        throw "Source coverage does not reconcile: canonical=$canonicalSourceRefs listed=$nodeToSourceRefs"
    }
    $outboundBullets = 0
    $inboundBullets = 0
    $section = ''
    foreach ($line in ($connections -split '\r?\n')) {
        if ($line -eq '### Outbound') { $section = 'out'; continue }
        if ($line -eq '### Inbound') { $section = 'in'; continue }
        if ($line -match '^- `[a-z0-9-]+` via `[a-z0-9_]+`$') {
            if ($section -eq 'out') { $outboundBullets++ } elseif ($section -eq 'in') { $inboundBullets++ }
        }
    }
    if ($outboundBullets -ne $canonicalRelationshipCount -or $inboundBullets -ne $canonicalRelationshipCount) {
        throw "Node connection listings do not reconcile: canonical=$canonicalRelationshipCount outbound_listed=$outboundBullets inbound_listed=$inboundBullets"
    }

    Write-Output "knowledge_index_files: $($expectedFiles.Count)"
    Write-Output "canonical_relationships: $canonicalRelationshipCount"
    Write-Output "outbound_total: $outbound"
    Write-Output "inbound_total: $inbound"
    Write-Output "canonical_nodes: $canonicalNodeCount"
    Write-Output "canonical_domains: $canonicalDomainCount"
    Write-Output "canonical_session_refs: $canonicalSessionRefs"
    Write-Output "canonical_source_refs: $canonicalSourceRefs"
    Write-Output 'knowledge_index_current: true'
} finally {
    if (Test-Path -LiteralPath $temporaryDirectory) {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}
