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
    $unexpected = @(Get-ChildItem $indexDirectory -File | Where-Object Name -notin $expectedFiles)
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
    Write-Output "knowledge_index_files: $($expectedFiles.Count)"
    Write-Output "canonical_relationships: $canonicalRelationshipCount"
    Write-Output "outbound_total: $outbound"
    Write-Output "inbound_total: $inbound"
    Write-Output 'knowledge_index_current: true'
} finally {
    if (Test-Path -LiteralPath $temporaryDirectory) {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}
