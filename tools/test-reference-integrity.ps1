[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$fixtureItems = @('tools','schemas','nodes','sources','sessions','vocabulary','experiments','experiment-runs','indexes','assets','research')
$passed = 0

function New-Fixture {
    $root = Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-reference-integrity-' + [guid]::NewGuid().ToString('N'))
    [void](New-Item -ItemType Directory -Path $root)
    foreach ($item in $fixtureItems) { Copy-Item -LiteralPath (Join-Path $repoRoot $item) -Destination $root -Recurse }
    return $root
}

function Replace-Exact([string]$Path, [string]$Old, [string]$New) {
    $text = [IO.File]::ReadAllText($Path)
    if (-not $text.Contains($Old, [StringComparison]::Ordinal)) { throw "Fixture text not found: $Old" }
    [IO.File]::WriteAllText($Path, $text.Replace($Old, $New, [StringComparison]::Ordinal), [Text.UTF8Encoding]::new($false))
}

function Assert-Failure([string]$Name, [scriptblock]$Mutate, [string]$Validator, [string]$MessagePattern) {
    $root = New-Fixture
    try {
        & $Mutate $root
        $output = @(& pwsh -NoProfile -File (Join-Path $root "tools/$Validator") 2>&1) -join "`n"
        if ($LASTEXITCODE -eq 0) { throw "$Name unexpectedly passed." }
        if ($output -notmatch $MessagePattern) { throw "$Name failed for the wrong reason. Output: $output" }
        $script:passed++; Write-Output "PASS: $Name"
    } finally { Remove-Item -LiteralPath $root -Recurse -Force }
}

function Assert-Pass([string]$Name, [string]$Validator) {
    $root = New-Fixture
    try {
        $output = @(& pwsh -NoProfile -File (Join-Path $root "tools/$Validator") 2>&1) -join "`n"
        if ($LASTEXITCODE -ne 0) { throw "$Name failed: $output" }
        $script:passed++; Write-Output "PASS: $Name"
    } finally { Remove-Item -LiteralPath $root -Recurse -Force }
}

Assert-Pass 'exact canonical references and generated indexes' 'validate-knowledge-index.ps1'
Assert-Failure 'graph target case drift' { param($r) Replace-Exact (Join-Path $r 'nodes/acoustics/sound.md') '  - target: frequency' '  - target: Frequency' } 'validate-graph.ps1' 'Malformed relationship entry in node: sound'
Assert-Failure 'relationship type case drift' { param($r) Replace-Exact (Join-Path $r 'nodes/acoustics/sound.md') '    type: characterized_by' '    type: Characterized_by' } 'validate-graph.ps1' 'Malformed relationship entry in node: sound'
Assert-Failure 'node source case drift' { param($r) Replace-Exact (Join-Path $r 'nodes/acoustics/sound.md') '  - session-01-vocabulary-atlas' '  - SESSION-01-VOCABULARY-ATLAS' } 'build-knowledge-index.ps1' 'references unresolved source: SESSION-01-VOCABULARY-ATLAS'
Assert-Failure 'node session case drift' { param($r) Replace-Exact (Join-Path $r 'nodes/acoustics/sound.md') '  - session-01-what-is-sound' '  - SESSION-01-WHAT-IS-SOUND' } 'build-knowledge-index.ps1' 'references unresolved session: SESSION-01-WHAT-IS-SOUND'
Assert-Failure 'vocabulary node reference case drift' { param($r) Replace-Exact (Join-Path $r 'vocabulary/entries/spectrum-perception.yaml') 'node_refs: ["frequency", "pitch"]' 'node_refs: ["Frequency", "pitch"]' } 'validate-vocabulary.ps1' 'Unresolved node reference.*Frequency'
Assert-Failure 'vocabulary cross-reference case drift' { param($r) Replace-Exact (Join-Path $r 'vocabulary/entries/spectrum-perception.yaml') 'related_terms: ["frequency", "harmonic"' 'related_terms: ["Frequency", "harmonic"' } 'validate-vocabulary.ps1' 'Unresolved related term.*Frequency'
Assert-Failure 'experiment reference case drift' { param($r) Replace-Exact (Join-Path $r 'experiment-runs/records/near-frequency-beating-planned-a.yaml') 'experiment_id: "near-frequency-beating"' 'experiment_id: "Near-Frequency-Beating"' } 'validate-experiment-runs.ps1' 'Unresolved experiment id.*Near-Frequency-Beating'
Assert-Failure 'source reference case drift' { param($r) Replace-Exact (Join-Path $r 'experiment-runs/records/near-frequency-beating-planned-a.yaml') 'source_refs: []' 'source_refs: ["SESSION-01-WHAT-IS-SOUND"]' } 'validate-experiment-runs.ps1' 'Unresolved source reference.*SESSION-01-WHAT-IS-SOUND'
Assert-Failure 'closed enum case drift' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'status: "proof"' 'status: "Proof"' } 'validate-experiments.ps1' 'Invalid experiment status.*Proof'
Assert-Failure 'unrelated malformed reference still fails' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'node_refs: [' 'node_refs: not-json-[' } 'validate-experiments.ps1' 'must use a JSON-compatible YAML value|must be an array'

Write-Output "reference_integrity_tests: $passed"
