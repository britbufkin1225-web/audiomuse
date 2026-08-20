[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$sourceClaims = Join-Path $repoRoot 'claims/records'
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-claim-tests-' + [guid]::NewGuid().ToString('N'))
$tempClaims = Join-Path $tempRoot 'records'; $tempIndex = Join-Path $tempRoot 'index.md'
$validator = Join-Path $PSScriptRoot 'validate-claims.ps1'; $builder = Join-Path $PSScriptRoot 'build-claim-index.ps1'
$technique = Join-Path $tempClaims 'audio-technique.yaml'
$houston = Join-Path $tempClaims 'houston-third-coast.yaml'
$passed = 0

function Reset-Fixture {
    if (Test-Path $tempClaims) { Remove-Item -LiteralPath $tempClaims -Recurse -Force }
    New-Item -ItemType Directory -Path $tempClaims -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $sourceClaims 'audio-technique.yaml') -Destination $tempClaims
    Copy-Item -LiteralPath (Join-Path $sourceClaims 'houston-third-coast.yaml') -Destination $tempClaims
    & $builder -ClaimDirectory $tempClaims -OutputPath $tempIndex | Out-Null
}
function Replace-Text([string]$Path, [string]$Old, [string]$New) {
    $text = [IO.File]::ReadAllText($Path)
    if (-not $text.Contains($Old, [StringComparison]::Ordinal)) { throw "Test fixture did not contain expected text: $Old" }
    [IO.File]::WriteAllText($Path, $text.Replace($Old, $New, [StringComparison]::Ordinal), [Text.UTF8Encoding]::new($false))
}
function Rebuild { & $builder -ClaimDirectory $tempClaims -OutputPath $tempIndex | Out-Null }

# Every negative case declares the message it must fail with. A validator that rejects a fixture for
# an unrelated reason proves nothing about the rule the case is named after.
function Expect-Failure([string]$Name, [scriptblock]$Sabotage, [string]$Pattern) {
    Reset-Fixture
    # A mutation the index builder itself refuses is still a rejection by the canonical parser, and it
    # is checked against the same pattern. A missing fixture anchor is a broken test, never a pass.
    $message = $null
    try { & $Sabotage }
    catch {
        if ($_.Exception.Message -like 'Test fixture did not contain expected text:*') { throw }
        $message = $_.Exception.Message
    }
    if ($null -eq $message) {
        try {
            & $validator -ClaimDirectory $tempClaims -IndexPath $tempIndex *> $null
            throw "Adversarial test unexpectedly passed: $Name"
        } catch {
            if ($_.Exception.Message -like 'Adversarial test unexpectedly passed:*') { throw }
            $message = $_.Exception.Message
        }
    }
    if ($message -cnotmatch $Pattern) { throw "Adversarial test '$Name' failed for the wrong reason. Expected /$Pattern/, got: $message" }
    $script:passed++; Write-Output "PASS: $Name"
}

try {
    # Baseline: the committed fixtures must actually validate, or every negative below proves nothing.
    Reset-Fixture
    try { & $validator -ClaimDirectory $tempClaims -IndexPath $tempIndex *> $null }
    catch { throw "Baseline claim fixtures failed validation: $($_.Exception.Message)" }
    $passed++; Write-Output 'PASS: baseline fixtures validate'

    # --- Closed vocabularies ------------------------------------------------------------------
    Expect-Failure 'invalid confidence value' { Replace-Text $technique 'confidence: "high"' 'confidence: "very_high"'; Rebuild } 'Invalid confidence for .*: very_high'
    Expect-Failure 'invalid claim type' { Replace-Text $technique 'claim_type: "technical_fact"' 'claim_type: "absolute_truth"'; Rebuild } 'Invalid claim_type for .*: absolute_truth'
    Expect-Failure 'invalid dispute status' { Replace-Text $houston 'dispute_status: "disputed"' 'dispute_status: "contested"'; Rebuild } 'Invalid dispute_status for .*: contested'
    Expect-Failure 'invalid temporal precision' { Replace-Text $technique 'temporal_precision: "not_temporal"' 'temporal_precision: "roughly"'; Rebuild } 'Invalid temporal_precision for .*: roughly'
    Expect-Failure 'invalid evidence relation' { Replace-Text $technique '"relation": "supported_by", "source_id": "session-01-what-is-sound"' '"relation": "mentions", "source_id": "session-01-what-is-sound"'; Rebuild } 'Invalid evidence relation for .*: mentions'
    Expect-Failure 'invalid appears_in kind' { Replace-Text $technique '{"kind": "vocabulary", "ref": "varispeed"}' '{"kind": "paragraph", "ref": "varispeed"}'; Rebuild } 'Invalid appears_in kind for .*: paragraph'
    Expect-Failure 'confidence enum case drift' { Replace-Text $technique 'confidence: "high"' 'confidence: "High"'; Rebuild } 'Invalid confidence for .*: High'
    Expect-Failure 'claim type enum case drift' { Replace-Text $technique 'claim_type: "technical_fact"' 'claim_type: "Technical_fact"'; Rebuild } 'Invalid claim_type for .*: Technical_fact'

    # --- Identity and structure ---------------------------------------------------------------
    Expect-Failure 'duplicate claim id' { Replace-Text $technique 'id: "phase-vocoder-attributed-to-flanagan-and-golden"' 'id: "playback-rate-couples-time-and-pitch"'; Rebuild } 'Duplicate claim id: playback-rate-couples-time-and-pitch'
    Expect-Failure 'missing required field' { Replace-Text $technique "open_questions: []`n" '' } 'does not contain exactly the required fields'
    Expect-Failure 'unknown top-level field' { [IO.File]::AppendAllText($technique, 'certainty: 0.87' + "`n", [Text.UTF8Encoding]::new($false)) } "Unsupported or duplicate claim field 'certainty'"
    Expect-Failure 'malformed evidence object' { Replace-Text $technique '{"relation": "supported_by", "source_id": "session-01-what-is-sound", "note": "Establishes that frequency is set by the rate at which cycles pass, which is the mechanism that makes rate change move pitch."}' '{"relation": "supported_by", "source_id": "session-01-what-is-sound"}'; Rebuild } "is missing required key 'note'"
    Expect-Failure 'unsupported key inside an evidence object' { Replace-Text $technique '"note": "Establishes that frequency is set by the rate at which cycles pass, which is the mechanism that makes rate change move pitch."}' '"note": "Establishes that frequency is set by the rate at which cycles pass.", "weight": "0.9"}'; Rebuild } "has unsupported key 'weight'"
    Expect-Failure 'duplicate evidence entry' { Replace-Text $technique '"relation": "supported_by", "source_id": "session-01-what-is-sound", "note": "Establishes' '"relation": "supported_by", "source_id": "smith-spectral-audio-signal-processing", "note": "Establishes'; Rebuild } 'Duplicate evidence entry for'
    Expect-Failure 'confidence_basis restates the claim' { Replace-Text $technique 'confidence_basis: "Two retrieved sources support the mechanism from different directions: Session 1 establishes that frequency follows from how quickly stored cycles pass, and the technical reference states the contrast between mechanical rate change and phase-vocoder time-scale modification explicitly. The statement is a physical consequence of the storage format rather than a historical report, so no further corroboration is needed."' 'confidence_basis: "Changing the playback rate of a fixed recording scales duration and frequency together, so tempo, pitch, and spectral envelope cannot be moved independently of one another by that means."'; Rebuild } 'restates the claim'

    # --- Reference resolution -----------------------------------------------------------------
    Expect-Failure 'nonexistent source reference' { Replace-Text $technique '"source_id": "session-01-what-is-sound"' '"source_id": "nonexistent-source"'; Rebuild } 'Unresolved evidence source for .*: nonexistent-source'
    Expect-Failure 'source reference case drift' { Replace-Text $technique '"source_id": "session-01-what-is-sound"' '"source_id": "SESSION-01-WHAT-IS-SOUND"'; Rebuild } 'Unresolved evidence source for .*: SESSION-01-WHAT-IS-SOUND'
    Expect-Failure 'nonexistent node appearance site' { Replace-Text $technique '{"kind": "node", "ref": "time-stretching"}' '{"kind": "node", "ref": "time-stretchingg"}'; Rebuild } 'Unresolved appears_in node for .*: time-stretchingg'
    Expect-Failure 'nonexistent vocabulary appearance site' { Replace-Text $technique '{"kind": "vocabulary", "ref": "varispeed"}' '{"kind": "vocabulary", "ref": "vari-speed"}'; Rebuild } 'Unresolved appears_in vocabulary entry for .*: vari-speed'
    Expect-Failure 'nonexistent document appearance site' { Replace-Text $houston '{"kind": "document", "ref": "docs/houston-musical-cartography.md"}' '{"kind": "document", "ref": "docs/houston-cartography.md"}'; Rebuild } 'Unresolved appears_in document for .*: docs/houston-cartography.md'
    Expect-Failure 'generated projection cited as an appearance site' { Replace-Text $houston '{"kind": "document", "ref": "docs/houston-musical-cartography.md"}' '{"kind": "document", "ref": "indexes/source-coverage.md"}'; Rebuild } 'cites a generated projection as an appearance site'
    Expect-Failure 'absolute document path' { Replace-Text $houston '{"kind": "document", "ref": "docs/houston-musical-cartography.md"}' '{"kind": "document", "ref": "/docs/houston-musical-cartography.md"}'; Rebuild } 'non-repository-relative document path'
    Expect-Failure 'non-session source used as a session appearance site' { Replace-Text $technique '{"kind": "vocabulary", "ref": "varispeed"}' '{"kind": "session", "ref": "tsha-dj-screw"}'; Rebuild } 'non-session source as a session appearance site'
    Expect-Failure 'unresolved derived_from claim' { Replace-Text $houston '{"kind": "claim", "ref": "screw-master-tape-method"}' '{"kind": "claim", "ref": "screw-master-tape-methods"}'; Rebuild } 'Unresolved derived_from claim for .*: screw-master-tape-methods'
    Expect-Failure 'claim with no appearance site' { Replace-Text $technique '{"kind": "node", "ref": "time-stretching"}]' ']'; Rebuild } 'records no appearance site'
    Expect-Failure 'self-derivation' { Replace-Text $houston '{"kind": "claim", "ref": "screw-master-tape-method"}, ' '{"kind": "claim", "ref": "screw-slowing-alters-vocal-timbre"}, '; Rebuild } 'Claim derives from itself'

    # --- Evidence and confidence semantics ----------------------------------------------------
    Expect-Failure 'high confidence on one weak source' {
        Replace-Text $houston '"relation": "supported_by", "source_id": "uh-dj-screw-collection-finding-aid", "note": "Describes custom requests' '"relation": "supported_by", "source_id": "popula-screw-tape-records", "note": "Describes custom requests'
        Rebuild } 'carries confidence high without two supporting sources'
    Expect-Failure 'unknown confidence while citing support' { Replace-Text $houston '"relation": "qualified_by", "source_id": "popula-screw-tape-records"' '"relation": "supported_by", "source_id": "popula-screw-tape-records"'; Rebuild } 'carries confidence unknown while citing supporting evidence'
    Expect-Failure 'one source recorded as both supporting and contradicting' { Replace-Text $houston '"relation": "contradicted_by", "source_id": "uh-dj-screw-collection-finding-aid"' '"relation": "contradicted_by", "source_id": "tsha-dj-screw"'; Rebuild } 'recorded as both supporting and contradicting'
    Expect-Failure 'undisputed claim citing contradicting evidence' { Replace-Text $houston 'dispute_status: "disputed"' 'dispute_status: "undisputed"'; Rebuild } 'marked undisputed while citing contradicting evidence'
    Expect-Failure 'disputed claim with no contradicting evidence' { Replace-Text $houston '"relation": "contradicted_by", "source_id": "uh-dj-screw-collection-finding-aid"' '"relation": "qualified_by", "source_id": "uh-dj-screw-collection-finding-aid"'; Rebuild } 'marked disputed without citing contradicting evidence'
    Expect-Failure 'unresolved claim without an open question' { Replace-Text $houston '["What licensing, FCC, or state broadcast record would establish whether an earlier Black-owned Texas station existed?", "Do the institutional Houston archives hold station records that carry the same characterization independently?"]' '[]'; Rebuild } 'marked unresolved without recording an open question'
    Expect-Failure 'settled fact downgraded to low confidence' { Replace-Text $houston 'confidence: "high"' 'confidence: "low"'; Rebuild } 'is typed established_fact with confidence low'
    Expect-Failure 'settled fact left undisputed while contradicted' { Replace-Text $houston '"relation": "qualified_by", "source_id": "tsha-fat-pat", "note": "Records that the exact address differs' '"relation": "contradicted_by", "source_id": "uh-dj-screw-collection-finding-aid", "note": "Records that the exact address differs'; Rebuild } 'marked undisputed while citing contradicting evidence'
    Expect-Failure 'attributed_claim without attribution' { Replace-Text $technique 'attribution: [{"actor": "Flanagan and Golden", "source_id": "smith-spectral-audio-signal-processing"}]' 'attribution: []'; Rebuild } 'is typed attributed_claim without an attribution entry'
    Expect-Failure 'oral_history without attribution' { Replace-Text $houston 'attribution: [{"actor": "Mike Dean", "source_id": "npr-microphone-check-mike-dean"}]' 'attribution: []'; Rebuild } 'is typed oral_history without an attribution entry'
    Expect-Failure 'synthesis with no supporting source' {
        Replace-Text $houston '[{"relation": "supported_by", "source_id": "smith-spectral-audio-signal-processing", "note": "Supports the decoupling contrast that the second half of the statement depends on."}, {"relation": "supported_by", "source_id": "uh-dj-screw-collection-finding-aid", "note": "Establishes that the method applied sustained mechanical slowing, first between two copies and then to the master."}]' '[{"relation": "qualified_by", "source_id": "smith-spectral-audio-signal-processing", "note": "Supports the decoupling contrast that the second half of the statement depends on."}]'
        Rebuild } 'cites no supporting source'
    Expect-Failure 'hypothesis promoted to high confidence' {
        Replace-Text $houston '"relation": "qualified_by", "source_id": "uh-dj-screw-collection-finding-aid", "note": "Describes the master-production chain' '"relation": "supported_by", "source_id": "uh-dj-screw-collection-finding-aid", "note": "Describes the master-production chain'
        Replace-Text $houston '"relation": "qualified_by", "source_id": "popula-screw-tape-records"' '"relation": "supported_by", "source_id": "popula-screw-tape-records"'
        Replace-Text $houston 'confidence: "unknown"' 'confidence: "high"'
        Rebuild } 'carries confidence high'
    Expect-Failure 'origin claim with no attribution' {
        Replace-Text $houston 'claim_type: "attributed_claim"' 'claim_type: "historical_claim"'
        Replace-Text $houston 'attribution: [{"actor": "Black Enterprise", "source_id": "black-enterprise-kcoh"}]' 'attribution: []'
        Rebuild } "uses the origin term 'first' without naming who credits it"
    Expect-Failure 'origin claim typed as a settled fact' {
        Replace-Text $houston 'statement: "Patrick Lamont Hawkins, known as Fat Pat and one of the original members of the Screwed Up Click, was fatally shot in Houston on February 3, 1998."' 'statement: "Patrick Lamont Hawkins, known as Fat Pat, was the first member of the Screwed Up Click to release a solo album."'
        Rebuild } "uses the origin term 'first' but is typed established_fact"
    Expect-Failure 'source cited by a claim without a declared evidence class' {
        Replace-Text $technique '"source_id": "session-01-what-is-sound"' '"source_id": "session-02-what-is-music"'
        Rebuild } 'does not declare evidence_class: session-02-what-is-music'

    # --- Generated index reconciliation -------------------------------------------------------
    Expect-Failure 'invented index entry' { [IO.File]::AppendAllText($tempIndex, '- `invented-claim`' + "`n", [Text.UTF8Encoding]::new($false)) } 'Claim index lists an unknown claim: invented-claim'
    Expect-Failure 'omitted index entry' { $text = [IO.File]::ReadAllText($tempIndex); [IO.File]::WriteAllText($tempIndex, [regex]::Replace($text, '(?m)^- `fat-pat-death-1998` — `established_fact`[^\r\n]*\r?\n', '', 1), [Text.UTF8Encoding]::new($false)) } 'Claim index is missing expected line'
    Expect-Failure 'wrong index total' { Replace-Text $tempIndex 'Canonical claims: 10' 'Canonical claims: 99' } 'Claim index is missing expected line: Canonical claims: 10'
    Expect-Failure 'index falsifies a confidence level' { Replace-Text $tempIndex '- `kcoh-first-black-owned-texas-station-attribution` — `attributed_claim` — `low`' '- `kcoh-first-black-owned-texas-station-attribution` — `attributed_claim` — `high`' } 'Claim index is missing expected line'
    Expect-Failure 'index falsifies a dispute status' { Replace-Text $tempIndex '- `screwed-up-records-1996-store-claim` — `historical_claim` — `low` — `disputed`' '- `screwed-up-records-1996-store-claim` — `historical_claim` — `low` — `undisputed`' } 'Claim index is missing expected line'
    Expect-Failure 'index invents a provenance heading' { [IO.File]::AppendAllText($tempIndex, '### `fabricated-source` — 1 claims' + "`n", [Text.UTF8Encoding]::new($false)) } 'unexpected grouping heading'
    Expect-Failure 'reordered index claims' {
        $text = [IO.File]::ReadAllText($tempIndex)
        $a = [regex]::Match($text, '(?m)^- `fat-pat-death-1998` [^
]*$').Value
        $b = [regex]::Match($text, '(?m)^- `houston-layered-infrastructure-reading` [^
]*$').Value
        if (-not $a -or -not $b) { throw 'Test fixture did not contain expected text: index claim lines' }
        $text = $text.Replace($a, '__ORDER_SENTINEL__').Replace($b, $a).Replace('__ORDER_SENTINEL__', $b)
        [IO.File]::WriteAllText($tempIndex, $text, [Text.UTF8Encoding]::new($false)) } 'not in ordinal id order'
    Expect-Failure 'stale generated index' { [IO.File]::AppendAllText($tempIndex, "`n", [Text.UTF8Encoding]::new($false)) } 'Generated claim index is stale'
    Expect-Failure 'unexpanded template artifact' { [IO.File]::AppendAllText($tempIndex, '$(' + 'unexpanded)' + "`n", [Text.UTF8Encoding]::new($false)) } 'unexpanded template output'

    Write-Output "adversarial_tests_passed: $passed"
} finally { if (Test-Path $tempRoot) { Remove-Item -LiteralPath $tempRoot -Recurse -Force } }
