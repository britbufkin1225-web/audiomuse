[CmdletBinding()]
param(
    [string]$ClaimDirectory,
    [string]$IndexPath
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if (-not $ClaimDirectory) { $ClaimDirectory = Join-Path $repoRoot 'claims/records' }
if (-not $IndexPath) { $IndexPath = Join-Path $repoRoot 'claims/index.md' }
$claimSchemaPath = Join-Path $repoRoot 'schemas/claim.schema.yaml'
$sourceSchemaPath = Join-Path $repoRoot 'schemas/source.schema.yaml'

function Sort-Ordinal([object[]]$Values) {
    $items = [string[]]@($Values | ForEach-Object { [string]$_ })
    [Array]::Sort($items, [StringComparer]::Ordinal)
    return $items
}

function Sort-UniqueOrdinal([object[]]$Values) {
    $set = [Collections.Generic.HashSet[string]]::new([string[]]@($Values | ForEach-Object { [string]$_ }), [StringComparer]::Ordinal)
    return (Sort-Ordinal @($set))
}

function Sort-ClaimsById([object[]]$Claims) {
    $items = [object[]]@($Claims)
    if ($items.Count -le 1) { return $items }
    $selector = [Func[object,string]]{ param($item) [string]$item.id }
    return [Linq.Enumerable]::ToArray([Linq.Enumerable]::OrderBy($items, $selector, [StringComparer]::Ordinal))
}

# Every claim vocabulary is read from schemas/claim.schema.yaml. Hard-coding the enums here would let
# a validator and its contract drift apart silently, which is the defect Phase 9 removed elsewhere.
function Get-SchemaList([string]$Text, [string]$Key) {
    $match = [regex]::Match($Text, ('(?ms)^{0}:\r?\n(?<body>(?:  - [^\r\n]+\r?\n)+)' -f [regex]::Escape($Key)))
    if (-not $match.Success) { throw "Claim schema does not declare a '$Key' list." }
    $values = @([regex]::Matches($match.Groups['body'].Value, '(?m)^  - (?<value>[^\r\n]+?)\r?$') | ForEach-Object { $_.Groups['value'].Value })
    if ($values.Count -eq 0) { throw "Claim schema list '$Key' is empty." }
    return $values
}

function Get-SchemaEnum([string]$Text, [string]$Property) {
    $match = [regex]::Match($Text, ('(?ms)^  {0}:\r?\n.*?^    enum: \[(?<values>[^\]]+)\]' -f [regex]::Escape($Property)))
    if (-not $match.Success) { throw "Source schema does not declare an enum for '$Property'." }
    return @($match.Groups['values'].Value -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

function Read-ClaimRecords([string]$Directory, [string[]]$Required) {
    $results = [Collections.Generic.List[object]]::new()
    $files = @(Get-ChildItem -LiteralPath $Directory -Filter '*.yaml' -File)
    foreach ($name in (Sort-Ordinal @($files | ForEach-Object { $_.Name }))) {
        $file = $files | Where-Object Name -ceq $name | Select-Object -First 1
        $documents = [regex]::Split([IO.File]::ReadAllText($file.FullName), '(?m)^---\r?\n') | Where-Object { $_.Trim() }
        foreach ($document in $documents) {
            $values = [ordered]@{}
            foreach ($line in ($document.Trim() -split '\r?\n')) {
                if ($line -notmatch '^(?<key>[a-z_]+): (?<value>.+)$') { throw "Malformed claim line in $($file.Name): $line" }
                $key = $Matches.key
                if ($key -notin $Required -or $values.Contains($key)) { throw "Unsupported or duplicate claim field '$key' in $($file.Name)" }
                try { $values[$key] = $Matches.value | ConvertFrom-Json -NoEnumerate } catch { throw "Claim field '$key' must use a JSON-compatible YAML value in $($file.Name)" }
            }
            if (@($values.Keys).Count -ne $Required.Count -or (@($Required | Where-Object { -not $values.Contains($_) })).Count) {
                throw "Claim record in $($file.Name) does not contain exactly the required fields."
            }
            $values['source_file'] = $file.Name
            $results.Add([pscustomobject]$values)
        }
    }
    return $results
}

function Assert-ObjectShape([object]$Item, [string[]]$Keys, [string]$Context) {
    if ($Item -isnot [psobject] -or $Item -is [string] -or $Item -is [array]) { throw "$Context must be an object." }
    $present = @($Item.PSObject.Properties.Name)
    foreach ($key in $Keys) { if ($key -cnotin $present) { throw "$Context is missing required key '$key'." } }
    foreach ($key in $present) { if ($key -cnotin $Keys) { throw "$Context has unsupported key '$key'." } }
    foreach ($key in $Keys) {
        $value = $Item.$key
        if ($value -isnot [string] -or -not $value.Trim()) { throw "$Context key '$key' must be a non-empty string." }
    }
}

# ---------------------------------------------------------------------------------------------
# Canonical vocabularies and reference universes.
# ---------------------------------------------------------------------------------------------
$claimSchemaText = [IO.File]::ReadAllText($claimSchemaPath)
$required = @(Get-SchemaList $claimSchemaText 'required')
$claimTypes = @(Get-SchemaList $claimSchemaText 'claim_types')
$confidenceLevels = @(Get-SchemaList $claimSchemaText 'confidence_levels')
$disputeStatuses = @(Get-SchemaList $claimSchemaText 'dispute_statuses')
$temporalPrecisions = @(Get-SchemaList $claimSchemaText 'temporal_precisions')
$evidenceRelations = @(Get-SchemaList $claimSchemaText 'evidence_relations')
$derivedFromKinds = @(Get-SchemaList $claimSchemaText 'derived_from_kinds')
$appearsInKinds = @(Get-SchemaList $claimSchemaText 'appears_in_kinds')
$authoritativeClasses = @(Get-SchemaList $claimSchemaText 'authoritative_evidence_classes')
$originTerms = @(Get-SchemaList $claimSchemaText 'origin_claim_terms')

$sourceSchemaText = [IO.File]::ReadAllText($sourceSchemaPath)
$evidenceClasses = @(Get-SchemaEnum $sourceSchemaText 'evidence_class')
$retrievalStates = @(Get-SchemaEnum $sourceSchemaText 'retrieval')
foreach ($class in $authoritativeClasses) {
    if ($class -cnotin $evidenceClasses) { throw "Claim schema names an authoritative evidence class the source schema does not define: $class" }
}

$registryText = [IO.File]::ReadAllText((Join-Path $repoRoot 'sources/source-registry.yaml'))
$sources = [Collections.Generic.Dictionary[string,object]]::new([StringComparer]::Ordinal)
foreach ($entry in [regex]::Matches($registryText, '(?ms)^  - id: (?<id>[a-z0-9]+(?:-[a-z0-9]+)*)\r?\n(?<body>(?:    [^\r\n]*\r?\n?)*)')) {
    $sourceId = $entry.Groups['id'].Value
    if ($sources.ContainsKey($sourceId)) { throw "Duplicate source id in registry: $sourceId" }
    $body = $entry.Groups['body'].Value
    $typeMatch = [regex]::Match($body, '(?m)^    type: (?<value>[^\r\n]+?)\r?$')
    if (-not $typeMatch.Success) { throw "Registry entry has no type: $sourceId" }
    $classMatch = [regex]::Match($body, '(?m)^    evidence_class: (?<value>[^\r\n]+?)\r?$')
    $retrievalMatch = [regex]::Match($body, '(?m)^    retrieval: (?<value>[^\r\n]+?)\r?$')
    $evidenceClass = if ($classMatch.Success) { $classMatch.Groups['value'].Value } else { $null }
    $retrieval = if ($retrievalMatch.Success) { $retrievalMatch.Groups['value'].Value } else { $null }
    if ($null -ne $evidenceClass -and $evidenceClass -cnotin $evidenceClasses) { throw "Registry source $sourceId declares an invalid evidence_class: $evidenceClass" }
    if ($null -ne $retrieval -and $retrieval -cnotin $retrievalStates) { throw "Registry source $sourceId declares an invalid retrieval value: $retrieval" }
    $sources[$sourceId] = [pscustomobject]@{ Type = $typeMatch.Groups['value'].Value; EvidenceClass = $evidenceClass; Retrieval = $retrieval }
}
if ($sources.Count -eq 0) { throw 'No source registry entries found.' }

$nodeIds = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
foreach ($file in @(Get-ChildItem (Join-Path $repoRoot 'nodes') -Recurse -Filter '*.md' | Where-Object Name -ne 'README.md')) {
    $match = [regex]::Match([IO.File]::ReadAllText($file.FullName), '(?m)^id: (?<id>[a-z0-9-]+)\r?$')
    if ($match.Success) { [void]$nodeIds.Add($match.Groups['id'].Value) }
}
if ($nodeIds.Count -eq 0) { throw 'No canonical nodes found.' }

$vocabularyIds = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
foreach ($file in @(Get-ChildItem (Join-Path $repoRoot 'vocabulary/entries') -Filter '*.yaml' -File)) {
    foreach ($match in [regex]::Matches([IO.File]::ReadAllText($file.FullName), '(?m)^id: "(?<id>[a-z0-9-]+)"\r?$')) {
        [void]$vocabularyIds.Add($match.Groups['id'].Value)
    }
}
if ($vocabularyIds.Count -eq 0) { throw 'No canonical vocabulary entries found.' }

$runIds = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
foreach ($file in @(Get-ChildItem (Join-Path $repoRoot 'experiment-runs/records') -Filter '*.yaml' -File)) {
    $match = [regex]::Match([IO.File]::ReadAllText($file.FullName), '(?m)^id: "(?<id>[a-z0-9-]+)"\r?$')
    if ($match.Success) { [void]$runIds.Add($match.Groups['id'].Value) }
}

# Generated projections are navigation, not authorship. A claim that cites one as its appearance site
# would make a derived view into evidence that the claim exists.
$generatedDocuments = [Collections.Generic.HashSet[string]]::new(
    [string[]]@('claims/index.md', 'vocabulary/index.md', 'experiments/index.md', 'experiment-runs/index.md'), [StringComparer]::Ordinal)

# ---------------------------------------------------------------------------------------------
# Structural validation.
# ---------------------------------------------------------------------------------------------
$claims = Sort-ClaimsById @(Read-ClaimRecords $ClaimDirectory $required)
if ($claims.Count -eq 0) { throw 'No canonical claim records found.' }

$claimIds = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
foreach ($claim in $claims) {
    if ($claim.id -isnot [string] -or $claim.id -notmatch '^[a-z0-9]+(?:-[a-z0-9]+)*$') { throw "Invalid claim id in $($claim.source_file): $($claim.id)" }
    if (-not $claimIds.Add($claim.id)) { throw "Duplicate claim id: $($claim.id)" }
}

$scalarFields = @('statement','claim_type','confidence','confidence_basis','dispute_status','temporal_precision')
$arrayFields = @('evidence','attribution','derived_from','appears_in','open_questions')
foreach ($claim in $claims) {
    foreach ($field in $scalarFields) {
        if ($claim.$field -isnot [string] -or -not $claim.$field.Trim()) { throw "Claim field '$field' must be a non-empty string: $($claim.id)" }
    }
    foreach ($field in $arrayFields) {
        if ($claim.$field -isnot [array]) { throw "Claim field '$field' must be an array: $($claim.id)" }
    }
    if ($claim.claim_type -cnotin $claimTypes) { throw "Invalid claim_type for $($claim.id): $($claim.claim_type)" }
    if ($claim.confidence -cnotin $confidenceLevels) { throw "Invalid confidence for $($claim.id): $($claim.confidence)" }
    if ($claim.dispute_status -cnotin $disputeStatuses) { throw "Invalid dispute_status for $($claim.id): $($claim.dispute_status)" }
    if ($claim.temporal_precision -cnotin $temporalPrecisions) { throw "Invalid temporal_precision for $($claim.id): $($claim.temporal_precision)" }
    if ($claim.confidence_basis.Trim() -ceq $claim.statement.Trim()) { throw "Claim confidence_basis restates the claim instead of grading its evidence: $($claim.id)" }
    foreach ($question in $claim.open_questions) {
        if ($question -isnot [string] -or -not $question.Trim()) { throw "Empty open_questions entry for $($claim.id)" }
    }
}

# ---------------------------------------------------------------------------------------------
# Reference resolution and evidence semantics.
# ---------------------------------------------------------------------------------------------
$evidenceRefs = 0; $attributionRefs = 0; $derivedRefs = 0; $appearanceRefs = 0
$citedSources = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
$derivationEdges = [Collections.Generic.Dictionary[string,string[]]]::new([StringComparer]::Ordinal)

foreach ($claim in $claims) {
    $seenEvidence = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    $supporting = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    $contradicting = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($item in $claim.evidence) {
        Assert-ObjectShape $item @('relation','source_id','note') "Evidence entry in claim $($claim.id)"
        if ($item.relation -cnotin $evidenceRelations) { throw "Invalid evidence relation for $($claim.id): $($item.relation)" }
        if (-not $sources.ContainsKey($item.source_id)) { throw "Unresolved evidence source for $($claim.id): $($item.source_id)" }
        if (-not $seenEvidence.Add("$($item.relation)|$($item.source_id)")) { throw "Duplicate evidence entry for $($claim.id): $($item.relation) $($item.source_id)" }
        if ($item.relation -ceq 'supported_by') { [void]$supporting.Add($item.source_id) }
        if ($item.relation -ceq 'contradicted_by') { [void]$contradicting.Add($item.source_id) }
        [void]$citedSources.Add($item.source_id)
        $evidenceRefs++
    }
    foreach ($sourceId in $supporting) {
        if ($contradicting.Contains($sourceId)) { throw "Source $sourceId is recorded as both supporting and contradicting claim $($claim.id)." }
    }

    $seenAttribution = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($item in $claim.attribution) {
        Assert-ObjectShape $item @('actor','source_id') "Attribution entry in claim $($claim.id)"
        if (-not $sources.ContainsKey($item.source_id)) { throw "Unresolved attribution source for $($claim.id): $($item.source_id)" }
        if (-not $seenAttribution.Add("$($item.actor)|$($item.source_id)")) { throw "Duplicate attribution entry for $($claim.id): $($item.actor)" }
        [void]$citedSources.Add($item.source_id)
        $attributionRefs++
    }

    $seenDerived = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    $derivedClaims = [Collections.Generic.List[string]]::new()
    foreach ($item in $claim.derived_from) {
        Assert-ObjectShape $item @('kind','ref') "Derivation entry in claim $($claim.id)"
        if ($item.kind -cnotin $derivedFromKinds) { throw "Invalid derived_from kind for $($claim.id): $($item.kind)" }
        if (-not $seenDerived.Add("$($item.kind)|$($item.ref)")) { throw "Duplicate derived_from entry for $($claim.id): $($item.kind) $($item.ref)" }
        switch ($item.kind) {
            'claim' {
                if ($item.ref -ceq $claim.id) { throw "Claim derives from itself: $($claim.id)" }
                if (-not $claimIds.Contains($item.ref)) { throw "Unresolved derived_from claim for $($claim.id): $($item.ref)" }
                $derivedClaims.Add($item.ref)
            }
            'node' { if (-not $nodeIds.Contains($item.ref)) { throw "Unresolved derived_from node for $($claim.id): $($item.ref)" } }
            'experiment_run' { if (-not $runIds.Contains($item.ref)) { throw "Unresolved derived_from experiment run for $($claim.id): $($item.ref)" } }
        }
        $derivedRefs++
    }
    $derivationEdges[$claim.id] = [string[]]@($derivedClaims)

    if ($claim.appears_in.Count -eq 0) { throw "Claim records no appearance site: $($claim.id)" }
    $seenAppearance = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($item in $claim.appears_in) {
        Assert-ObjectShape $item @('kind','ref') "Appearance entry in claim $($claim.id)"
        if ($item.kind -cnotin $appearsInKinds) { throw "Invalid appears_in kind for $($claim.id): $($item.kind)" }
        if (-not $seenAppearance.Add("$($item.kind)|$($item.ref)")) { throw "Duplicate appears_in entry for $($claim.id): $($item.kind) $($item.ref)" }
        switch ($item.kind) {
            'node' { if (-not $nodeIds.Contains($item.ref)) { throw "Unresolved appears_in node for $($claim.id): $($item.ref)" } }
            'vocabulary' { if (-not $vocabularyIds.Contains($item.ref)) { throw "Unresolved appears_in vocabulary entry for $($claim.id): $($item.ref)" } }
            'session' {
                if (-not $sources.ContainsKey($item.ref)) { throw "Unresolved appears_in session for $($claim.id): $($item.ref)" }
                if ($sources[$item.ref].Type -cne 'session') { throw "Claim $($claim.id) records a non-session source as a session appearance site: $($item.ref)" }
            }
            'document' {
                if ($item.ref -match '\\' -or $item.ref.StartsWith('/') -or $item.ref -match '(^|/)\.\.(/|$)') { throw "Claim $($claim.id) uses a non-repository-relative document path: $($item.ref)" }
                if ($generatedDocuments.Contains($item.ref) -or $item.ref.StartsWith('indexes/', [StringComparison]::Ordinal)) { throw "Claim $($claim.id) cites a generated projection as an appearance site: $($item.ref)" }
                $resolved = Join-Path $repoRoot $item.ref
                if (-not (Test-Path -LiteralPath $resolved -PathType Leaf)) { throw "Unresolved appears_in document for $($claim.id): $($item.ref)" }
            }
        }
        $appearanceRefs++
    }
}

# Every source a claim leans on must declare what kind of evidence it is and whether its text was
# read. The registry stays optional elsewhere, so annotation follows use instead of preceding it.
foreach ($sourceId in (Sort-Ordinal @($citedSources))) {
    $source = $sources[$sourceId]
    if (-not $source.EvidenceClass) { throw "Source cited by a claim does not declare evidence_class: $sourceId" }
    if (-not $source.Retrieval) { throw "Source cited by a claim does not declare retrieval: $sourceId" }
}

# ---------------------------------------------------------------------------------------------
# Claim semantics. These are the rules block in schemas/claim.schema.yaml.
# ---------------------------------------------------------------------------------------------
foreach ($claim in $claims) {
    $supportIds = @(Sort-UniqueOrdinal @($claim.evidence | Where-Object { $_.relation -ceq 'supported_by' } | ForEach-Object { $_.source_id }))
    $contradictCount = @($claim.evidence | Where-Object { $_.relation -ceq 'contradicted_by' }).Count
    $derivedCount = @($claim.derived_from).Count

    switch ($claim.confidence) {
        'high' {
            $authoritative = @($supportIds | Where-Object {
                $sources[$_].EvidenceClass -cin $authoritativeClasses -and $sources[$_].Retrieval -cne 'citation_only' })
            if ($supportIds.Count -lt 2 -and $authoritative.Count -lt 1) {
                throw "Claim $($claim.id) carries confidence high without two supporting sources or one retrieved authoritative source."
            }
        }
        'unknown' {
            if ($supportIds.Count -ne 0) { throw "Claim $($claim.id) carries confidence unknown while citing supporting evidence." }
        }
        default {
            if (@($claim.evidence).Count -eq 0) { throw "Claim $($claim.id) carries confidence $($claim.confidence) without any evidence entry." }
        }
    }

    if ($claim.claim_type -cin @('established_fact','technical_fact')) {
        if ($claim.dispute_status -cne 'undisputed') { throw "Claim $($claim.id) is typed $($claim.claim_type) but is not undisputed." }
        if ($claim.confidence -cin @('low','unknown')) { throw "Claim $($claim.id) is typed $($claim.claim_type) with confidence $($claim.confidence)." }
    }
    if ($claim.claim_type -cin @('attributed_claim','oral_history') -and @($claim.attribution).Count -eq 0) {
        throw "Claim $($claim.id) is typed $($claim.claim_type) without an attribution entry."
    }
    if ($claim.claim_type -ceq 'audiomuse_synthesis') {
        if ($supportIds.Count -lt 1) { throw "Synthesis claim $($claim.id) cites no supporting source." }
        if (($supportIds.Count + $derivedCount) -lt 2) { throw "Synthesis claim $($claim.id) combines fewer than two grounding references." }
    }
    if ($claim.claim_type -ceq 'hypothesis') {
        if ($claim.confidence -ceq 'high') { throw "Hypothesis $($claim.id) carries confidence high." }
        if ((@($claim.evidence).Count + $derivedCount) -eq 0) { throw "Hypothesis $($claim.id) cites neither evidence nor a derivation." }
    }
    if ($claim.claim_type -ceq 'experiment_observation') {
        if (@($claim.derived_from | Where-Object { $_.kind -ceq 'experiment_run' }).Count -eq 0) {
            throw "Observation claim $($claim.id) is not derived from an experiment run."
        }
    }

    switch ($claim.dispute_status) {
        'undisputed' { if ($contradictCount -ne 0) { throw "Claim $($claim.id) is marked undisputed while citing contradicting evidence." } }
        'disputed' { if ($contradictCount -eq 0) { throw "Claim $($claim.id) is marked disputed without citing contradicting evidence." } }
        'unresolved' {
            if ($claim.confidence -ceq 'high') { throw "Claim $($claim.id) is marked unresolved while carrying confidence high." }
            if (@($claim.open_questions).Count -eq 0) { throw "Claim $($claim.id) is marked unresolved without recording an open question." }
        }
    }

    # Priority language asserts exclusivity. AudioMuse may record such a claim, but only as somebody's
    # claim: see the origin-claim section of docs/claim-provenance-model.md.
    foreach ($term in $originTerms) {
        if ($claim.statement -match ('(?i)(?<![\w-])' + [regex]::Escape($term) + '(?![\w-])')) {
            if ($claim.claim_type -cnotin @('historical_claim','attributed_claim','oral_history')) {
                throw "Claim $($claim.id) uses the origin term '$term' but is typed $($claim.claim_type)."
            }
            if (@($claim.attribution).Count -eq 0) {
                throw "Claim $($claim.id) uses the origin term '$term' without naming who credits it."
            }
        }
    }
}

# A derivation chain that returns to its start makes a claim its own justification.
foreach ($start in (Sort-Ordinal @($derivationEdges.Keys))) {
    $stack = [Collections.Generic.Stack[string]]::new()
    $visited = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($next in $derivationEdges[$start]) { $stack.Push($next) }
    while ($stack.Count -gt 0) {
        $current = $stack.Pop()
        if ($current -ceq $start) { throw "Claim derivation cycle detected through: $start" }
        if (-not $visited.Add($current)) { continue }
        foreach ($next in $derivationEdges[$current]) { $stack.Push($next) }
    }
}

# ---------------------------------------------------------------------------------------------
# Generated index reconciliation.
#
# Every expected line is re-derived here from canonical records. The byte comparison against a fresh
# build runs afterwards and catches staleness; it cannot catch a builder that formats every run the
# same wrong way, which is what this independent derivation is for.
# ---------------------------------------------------------------------------------------------
if (-not (Test-Path -LiteralPath $IndexPath -PathType Leaf)) { throw 'Missing generated claim index.' }
$indexText = [IO.File]::ReadAllText($IndexPath)
foreach ($marker in @('System.Object[]', 'System.Collections.Hashtable', '$(')) {
    if ($indexText.Contains($marker)) { throw "Claim index contains unexpanded template output ('$marker')." }
}
$indexLines = [string[]]($indexText -split '\r?\n')
$present = [Collections.Generic.HashSet[string]]::new($indexLines, [StringComparer]::Ordinal)

$expected = [Collections.Generic.List[string]]::new()
$expected.Add('# AudioMuse Claim Index')
$expected.Add("Canonical claims: $($claims.Count)")
foreach ($heading in @('## Claims','## By Claim Type','## By Confidence','## By Dispute Status','## By Source','## By Appearance Site')) { $expected.Add($heading) }
foreach ($claim in $claims) {
    $expected.Add(('- `{0}` — `{1}` — `{2}` — `{3}` — {4}' -f $claim.id, $claim.claim_type, $claim.confidence, $claim.dispute_status, $claim.statement))
}
foreach ($pair in @(
        @{ Field = 'claim_type'; Values = $claimTypes },
        @{ Field = 'confidence'; Values = $confidenceLevels },
        @{ Field = 'dispute_status'; Values = $disputeStatuses })) {
    foreach ($value in $pair.Values) {
        $members = @($claims | Where-Object { $_.($pair.Field) -ceq $value })
        if ($members.Count -eq 0) { continue }
        $expected.Add(('### `{0}` — {1} claims' -f $value, $members.Count))
    }
}
$sourceRows = [Collections.Generic.List[object]]::new()
foreach ($claim in $claims) {
    foreach ($relation in @($evidenceRelations + @('attribution'))) {
        $ids = if ($relation -ceq 'attribution') {
            @($claim.attribution | ForEach-Object { $_.source_id })
        } else {
            @($claim.evidence | Where-Object { $_.relation -ceq $relation } | ForEach-Object { $_.source_id })
        }
        foreach ($sourceId in (Sort-UniqueOrdinal @($ids))) {
            $sourceRows.Add([pscustomobject]@{ SourceId = $sourceId; ClaimId = [string]$claim.id; Relation = $relation })
        }
    }
}
foreach ($sourceId in (Sort-UniqueOrdinal @($sourceRows | ForEach-Object { $_.SourceId }))) {
    $rows = @($sourceRows | Where-Object { $_.SourceId -ceq $sourceId })
    $expected.Add(('### `{0}` — {1} claims' -f $sourceId, @(Sort-UniqueOrdinal @($rows | ForEach-Object { $_.ClaimId })).Count))
    foreach ($row in $rows) { $expected.Add(('- `{0}` — `{1}`' -f $row.ClaimId, $row.Relation)) }
}
foreach ($kind in $appearsInKinds) {
    foreach ($ref in (Sort-UniqueOrdinal @($claims | ForEach-Object { $_.appears_in } | Where-Object { $_.kind -ceq $kind } | ForEach-Object { $_.ref }))) {
        $members = @($claims | Where-Object { @($_.appears_in | Where-Object { $_.kind -ceq $kind -and $_.ref -ceq $ref }).Count -gt 0 })
        $expected.Add(('### {0} `{1}` — {2} claims' -f $kind, $ref, $members.Count))
        foreach ($member in $members) { $expected.Add(('- `{0}`' -f $member.id)) }
    }
}
foreach ($line in $expected) { if (-not $present.Contains($line)) { throw "Claim index is missing expected line: $line" } }

# Nothing may appear in the projection that canonical records do not produce.
$listedClaims = @([regex]::Matches($indexText, '(?m)^- `(?<id>[a-z0-9-]+)` — `') | ForEach-Object { $_.Groups['id'].Value })
if ($listedClaims.Count -eq 0) { throw 'Claim index lists no claims.' }
$claimSection = [regex]::Match($indexText, '(?ms)^## Claims\r?\n(?<body>.*?)^## ')
if (-not $claimSection.Success) { throw 'Claim index has no readable Claims section.' }
$sectionIds = @([regex]::Matches($claimSection.Groups['body'].Value, '(?m)^- `(?<id>[a-z0-9-]+)` — `') | ForEach-Object { $_.Groups['id'].Value })
if ($sectionIds.Count -ne $claims.Count) { throw "Claim index Claims section lists $($sectionIds.Count) claims but $($claims.Count) canonical records exist." }
$sortedIds = Sort-Ordinal @($sectionIds)
for ($i = 0; $i -lt $sortedIds.Count; $i++) {
    if ($sectionIds[$i] -cne $sortedIds[$i]) { throw "Claim index Claims section is not in ordinal id order at position $($i + 1): expected '$($sortedIds[$i])', found '$($sectionIds[$i])'." }
}
foreach ($id in $listedClaims) { if (-not $claimIds.Contains($id)) { throw "Claim index lists an unknown claim: $id" } }
foreach ($match in [regex]::Matches($indexText, '(?m)^- `(?<id>[a-z0-9-]+)`\r?$')) {
    if (-not $claimIds.Contains($match.Groups['id'].Value)) { throw "Claim index lists an unknown claim: $($match.Groups['id'].Value)" }
}
$knownHeadings = [Collections.Generic.HashSet[string]]::new([string[]]@($expected | Where-Object { $_.StartsWith('### ', [StringComparison]::Ordinal) }), [StringComparer]::Ordinal)
foreach ($match in [regex]::Matches($indexText, '(?m)^### [^\r\n]+')) {
    if (-not $knownHeadings.Contains($match.Value)) { throw "Claim index contains an unexpected grouping heading: $($match.Value)" }
}

$temp = Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-claims-' + [guid]::NewGuid().ToString('N') + '.md')
try {
    & (Join-Path $PSScriptRoot 'build-claim-index.ps1') -ClaimDirectory $ClaimDirectory -OutputPath $temp | Out-Null
    if ([Convert]::ToBase64String([IO.File]::ReadAllBytes($temp)) -cne [Convert]::ToBase64String([IO.File]::ReadAllBytes($IndexPath))) { throw 'Generated claim index is stale.' }
} finally { if (Test-Path $temp) { Remove-Item -LiteralPath $temp -Force } }

Write-Output "claims: $($claims.Count)"
foreach ($type in $claimTypes) {
    $count = @($claims | Where-Object { $_.claim_type -ceq $type }).Count
    if ($count -gt 0) { Write-Output "  ${type}: $count" }
}
foreach ($level in $confidenceLevels) { Write-Output "confidence_${level}: $(@($claims | Where-Object { $_.confidence -ceq $level }).Count)" }
foreach ($status in $disputeStatuses) { Write-Output "dispute_${status}: $(@($claims | Where-Object { $_.dispute_status -ceq $status }).Count)" }
Write-Output "claim_evidence_refs: $evidenceRefs"
Write-Output "claim_attribution_refs: $attributionRefs"
Write-Output "claim_derived_from_refs: $derivedRefs"
Write-Output "claim_appearance_refs: $appearanceRefs"
Write-Output "cited_sources: $($citedSources.Count)"
Write-Output 'unresolved_claim_refs: 0'
Write-Output 'duplicate_claim_ids: 0'
Write-Output 'claim_derivation_cycles: 0'
Write-Output 'claim_index_structure_verified: true'
Write-Output 'claim_index_current: true'
