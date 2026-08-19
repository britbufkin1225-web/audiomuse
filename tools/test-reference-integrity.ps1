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
        if ($output -cnotmatch $MessagePattern) { throw "$Name failed for the wrong reason. Output: $output" }
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

function Assert-MutatedPass([string]$Name, [scriptblock]$Mutate, [string]$Validator) {
    $root = New-Fixture
    try {
        & $Mutate $root
        $output = @(& pwsh -NoProfile -File (Join-Path $root "tools/$Validator") 2>&1) -join "`n"
        if ($LASTEXITCODE -ne 0) { throw "$Name was rejected but should be accepted: $output" }
        $script:passed++; Write-Output "PASS: $Name"
    } finally { Remove-Item -LiteralPath $root -Recurse -Force }
}

Assert-Pass 'exact canonical references and generated indexes' 'validate-knowledge-index.ps1'
Assert-Failure 'graph target case drift rejected by the lowercase relationship parser' { param($r) Replace-Exact (Join-Path $r 'nodes/acoustics/sound.md') '  - target: frequency' '  - target: Frequency' } 'validate-graph.ps1' 'Malformed relationship entry in node: sound'
Assert-Failure 'relationship type case drift rejected by the lowercase relationship parser' { param($r) Replace-Exact (Join-Path $r 'nodes/acoustics/sound.md') '    type: characterized_by' '    type: Characterized_by' } 'validate-graph.ps1' 'Malformed relationship entry in node: sound'
Assert-Failure 'node source case drift' { param($r) Replace-Exact (Join-Path $r 'nodes/acoustics/sound.md') '  - session-01-vocabulary-atlas' '  - SESSION-01-VOCABULARY-ATLAS' } 'build-knowledge-index.ps1' 'references unresolved source: SESSION-01-VOCABULARY-ATLAS'
Assert-Failure 'node session case drift' { param($r) Replace-Exact (Join-Path $r 'nodes/acoustics/sound.md') '  - session-01-what-is-sound' '  - SESSION-01-WHAT-IS-SOUND' } 'build-knowledge-index.ps1' 'references unresolved session: SESSION-01-WHAT-IS-SOUND'
Assert-Failure 'vocabulary node reference case drift' { param($r) Replace-Exact (Join-Path $r 'vocabulary/entries/spectrum-perception.yaml') 'node_refs: ["frequency", "pitch"]' 'node_refs: ["Frequency", "pitch"]' } 'validate-vocabulary.ps1' 'Unresolved node reference.*Frequency'
Assert-Failure 'vocabulary cross-reference case drift' { param($r) Replace-Exact (Join-Path $r 'vocabulary/entries/spectrum-perception.yaml') 'related_terms: ["frequency", "harmonic"' 'related_terms: ["Frequency", "harmonic"' } 'validate-vocabulary.ps1' 'Unresolved related term.*Frequency'
Assert-Failure 'experiment reference case drift' { param($r) Replace-Exact (Join-Path $r 'experiment-runs/records/near-frequency-beating-planned-a.yaml') 'experiment_id: "near-frequency-beating"' 'experiment_id: "Near-Frequency-Beating"' } 'validate-experiment-runs.ps1' 'Unresolved experiment id.*Near-Frequency-Beating'
Assert-Failure 'source reference case drift' { param($r) Replace-Exact (Join-Path $r 'experiment-runs/records/near-frequency-beating-planned-a.yaml') 'source_refs: []' 'source_refs: ["SESSION-01-WHAT-IS-SOUND"]' } 'validate-experiment-runs.ps1' 'Unresolved source reference.*SESSION-01-WHAT-IS-SOUND'
Assert-Failure 'closed enum case drift' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'status: "proof"' 'status: "Proof"' } 'validate-experiments.ps1' 'Invalid experiment status.*Proof'
Assert-Failure 'unrelated malformed reference still fails' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'node_refs: [' 'node_refs: not-json-[' } 'validate-experiments.ps1' 'must use a JSON-compatible YAML value|must be an array'

# Closed enums the suite did not previously reach.
Assert-Failure 'experiment type enum case drift' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'type: "hybrid"' 'type: "Hybrid"' } 'validate-experiments.ps1' 'Invalid experiment type.*Hybrid'
Assert-Failure 'experiment difficulty enum case drift' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'difficulty: "introductory"' 'difficulty: "Introductory"' } 'validate-experiments.ps1' 'Invalid experiment difficulty.*Introductory'
Assert-Failure 'vocabulary domain enum case drift' { param($r) Replace-Exact (Join-Path $r 'vocabulary/entries/spectrum-perception.yaml') 'domain: "psychoacoustics"' 'domain: "Psychoacoustics"' } 'validate-vocabulary.ps1' 'Invalid vocabulary domain.*Psychoacoustics'

# Canonical reference categories the suite did not previously reach.
Assert-Failure 'vocabulary session reference case drift' { param($r) Replace-Exact (Join-Path $r 'vocabulary/entries/spectrum-perception.yaml') 'session_refs: ["session-01-what-is-sound"' 'session_refs: ["SESSION-01-WHAT-IS-SOUND"' } 'validate-vocabulary.ps1' 'Unresolved session reference.*SESSION-01-WHAT-IS-SOUND'
Assert-Failure 'experiment vocabulary reference case drift' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'vocabulary_refs: ["frequency"' 'vocabulary_refs: ["Frequency"' } 'validate-experiments.ps1' 'Unresolved vocabulary reference.*Frequency'
Assert-Failure 'experiment session reference case drift' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'session_refs: ["session-01-what-is-sound"' 'session_refs: ["SESSION-01-WHAT-IS-SOUND"' } 'validate-experiments.ps1' 'Invalid session reference.*SESSION-01-WHAT-IS-SOUND'
Assert-Failure 'experiment source reference case drift' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'source_refs: ["session-01-what-is-sound"' 'source_refs: ["SESSION-01-WHAT-IS-SOUND"' } 'validate-experiments.ps1' 'Unresolved source reference.*SESSION-01-WHAT-IS-SOUND'

# Duplicate detection is ordinal, so a case-drifted reference sitting beside its canonical form is
# reported as an unresolved reference. A case-insensitive set reported it as a duplicate instead,
# which named the wrong defect and made these validators disagree with validate-vocabulary.ps1.
Assert-Failure 'case-drifted node reference beside its canonical form' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'node_refs: ["frequency", "phase"' 'node_refs: ["frequency", "Frequency", "phase"' } 'validate-experiments.ps1' 'Unresolved node reference.*Frequency'
Assert-Failure 'case-drifted run source reference beside its canonical form' { param($r) Replace-Exact (Join-Path $r 'experiment-runs/records/near-frequency-beating-planned-a.yaml') 'source_refs: []' 'source_refs: ["session-01-what-is-sound", "SESSION-01-WHAT-IS-SOUND"]' } 'validate-experiment-runs.ps1' 'Unresolved source reference.*SESSION-01-WHAT-IS-SOUND'

# The same ordinal rule must not reject prose entries that legitimately differ only by case.
Assert-MutatedPass 'prose values differing only by case are distinct' { param($r) Replace-Exact (Join-Path $r 'experiments/records/near-frequency-beating.yaml') 'project_connections: ["instrument tuning",' 'project_connections: ["instrument tuning", "Instrument tuning",' } 'validate-experiments.ps1'
Assert-MutatedPass 'run prose values differing only by case are distinct' { param($r) Replace-Exact (Join-Path $r 'experiment-runs/records/near-frequency-beating-planned-a.yaml') 'equipment: [' 'equipment: ["Bench notes", "bench notes", ' } 'validate-experiment-runs.ps1'

Write-Output "reference_integrity_tests: $passed"
