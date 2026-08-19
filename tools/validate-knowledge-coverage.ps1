[CmdletBinding()]
param([string]$RepositoryRoot,[string]$ReportPath,[string]$JsonPath)
$ErrorActionPreference='Stop';if(-not $RepositoryRoot){$RepositoryRoot=Split-Path -Parent $PSScriptRoot};if(-not $ReportPath){$ReportPath=Join-Path $RepositoryRoot 'indexes/knowledge-coverage.md'};if(-not $JsonPath){$JsonPath=Join-Path $RepositoryRoot 'indexes/knowledge-coverage.json'}
function IdSet([string[]]$Ids){return ,[Collections.Generic.HashSet[string]]::new($Ids,[StringComparer]::Ordinal)}
function AssertRefs([string]$Kind,[object[]]$Refs,$Known){foreach($r in $Refs){if(-not $Known.Contains([string]$r)){throw "Unresolved $Kind reference: $r"}}}
function RecordFiles([string]$Dir){$r=@();foreach($f in Get-ChildItem $Dir -Filter '*.yaml' -File){$v=[ordered]@{};foreach($line in [IO.File]::ReadAllLines($f.FullName)){if($line -notmatch '^(?<k>[a-z_]+): (?<v>.+)$'){throw "Malformed record: $($f.Name)"};$v[$Matches.k]=$Matches.v|ConvertFrom-Json -NoEnumerate};$r += [pscustomobject]$v};return @($r)}
function VocabFiles([string]$Dir){$r=@();foreach($f in Get-ChildItem $Dir -Filter '*.yaml' -File){foreach($doc in ([regex]::Split([IO.File]::ReadAllText($f.FullName),'(?m)^---\r?\n')|Where-Object{$_.Trim()})){$v=[ordered]@{};foreach($line in ($doc.Trim() -split '\r?\n')){if($line -notmatch '^(?<k>[a-z_]+): (?<v>.+)$'){throw "Malformed vocabulary: $($f.Name)"};$v[$Matches.k]=$Matches.v|ConvertFrom-Json -NoEnumerate};$r += [pscustomobject]$v}};return @($r)}
if(-not(Test-Path $ReportPath)-or-not(Test-Path $JsonPath)){throw 'Missing generated knowledge coverage output.'};$model=[IO.File]::ReadAllText($JsonPath)|ConvertFrom-Json
if($model.schema -cne 'audiomuse-knowledge-coverage'-or$model.version-ne 1-or$model.authority-cne'derived-read-only'){throw 'Invalid knowledge coverage contract identity.'}
$nodeTexts=@(Get-ChildItem (Join-Path $RepositoryRoot 'nodes') -Recurse -Filter '*.md'|Where-Object Name -ne 'README.md'|ForEach-Object{[IO.File]::ReadAllText($_.FullName)});$nodeIds=@($nodeTexts|ForEach-Object{[regex]::Match($_,'(?m)^id: (?<v>[a-z0-9-]+)\r?$').Groups['v'].Value});$nodeSet=IdSet $nodeIds
$sourceText=[IO.File]::ReadAllText((Join-Path $RepositoryRoot 'sources/source-registry.yaml'));$sourceIds=@([regex]::Matches($sourceText,'(?m)^  - id: (?<v>[a-z0-9-]+)\r?$')|ForEach-Object{$_.Groups['v'].Value});$sourceSet=IdSet $sourceIds;$sessionIds=@([regex]::Matches($sourceText,'(?m)^  - id: (?<id>[a-z0-9-]+)\r?\n    type: session\r?$')|ForEach-Object{$_.Groups['id'].Value});$sessionSet=IdSet $sessionIds
$vocab=VocabFiles (Join-Path $RepositoryRoot 'vocabulary/entries');$vocabSet=IdSet @($vocab.id);$experiments=RecordFiles (Join-Path $RepositoryRoot 'experiments/records');$experimentSet=IdSet @($experiments.id);$runs=RecordFiles (Join-Path $RepositoryRoot 'experiment-runs/records');$runSet=IdSet @($runs.id)
foreach($t in $nodeTexts){AssertRefs 'node' @([regex]::Matches($t,'(?m)^  - target: (?<v>[a-z0-9-]+)\r?$')|ForEach-Object{$_.Groups['v'].Value}) $nodeSet;AssertRefs 'source' @([regex]::Match($t,'(?ms)^sources:\r?\n(?<b>(?:  - [^\r\n]+\r?\n?)*)').Groups['b'].Value -split '\r?\n'|Where-Object{$_}|ForEach-Object{$_.Substring(4)}) $sourceSet;AssertRefs 'session' @([regex]::Match($t,'(?ms)^session_origin:\r?\n(?<b>(?:  - [^\r\n]+\r?\n?)*)').Groups['b'].Value -split '\r?\n'|Where-Object{$_}|ForEach-Object{$_.Substring(4)}) $sessionSet}
foreach($v in $vocab){AssertRefs 'node' $v.node_refs $nodeSet;AssertRefs 'session' $v.session_refs $sessionSet;AssertRefs 'vocabulary' $v.related_terms $vocabSet}
foreach($e in $experiments){AssertRefs 'node' $e.node_refs $nodeSet;AssertRefs 'session' $e.session_refs $sessionSet;AssertRefs 'source' $e.source_refs $sourceSet;AssertRefs 'vocabulary' $e.vocabulary_refs $vocabSet;AssertRefs 'experiment' $e.related_experiments $experimentSet}
foreach($r in $runs){AssertRefs 'experiment' @($r.experiment_id) $experimentSet;AssertRefs 'source' $r.source_refs $sourceSet}
$edgeCount=@($nodeTexts|ForEach-Object{[regex]::Matches($_,'(?m)^  - target: [a-z0-9-]+\r?$').Count}|Measure-Object -Sum).Sum;$typeCount=@([regex]::Matches([IO.File]::ReadAllText((Join-Path $RepositoryRoot 'schemas/relationship-types.yaml')),'(?m)^  - id: [a-z0-9_]+\r?$')).Count
$expected=[ordered]@{node_count=$nodeIds.Count;relationship_count=$edgeCount;relationship_type_count=$typeCount;source_count=$sourceIds.Count;session_count=$sessionIds.Count;vocabulary_count=$vocab.Count;experiment_count=$experiments.Count;experiment_run_count=$runs.Count};foreach($p in $expected.GetEnumerator()){if([int]$model.overview.($p.Key)-ne$p.Value){throw "Generated coverage count mismatch: $($p.Key) expected $($p.Value), found $($model.overview.($p.Key))"}}

# Independent reconstruction. Every derived claim below is recomputed from canonical
# files and compared against the generated view, so a builder defect cannot validate
# itself through the rebuild byte-comparison alone.
function CanonList([string]$Text,[string]$Name){if($Text -match "(?m)^${Name}: \[\]\r?$"){return @()};$m=[regex]::Match($Text,"(?ms)^${Name}:\r?\n(?<b>(?:  - [^\r\n]+\r?\n?)*)");if(-not $m.Success){throw "Missing canonical list '$Name'."};return @([regex]::Matches($m.Groups['b'].Value,'(?m)^  - (?<v>[^\r\n]+)\r?$')|ForEach-Object{$_.Groups['v'].Value.Trim('"')})}
function IdList($Items){return @($Items|ForEach-Object{[string]$_.id})}
function OrdinalSorted([string[]]$Values){$a=[string[]]@(@($Values)|Where-Object{$null -ne $_});[Array]::Sort($a,[StringComparer]::Ordinal);return ,$a}
function AssertOrdinalOrder([string]$Label,[string[]]$Ids){$previous='';foreach($id in $Ids){if($previous -and [StringComparer]::Ordinal.Compare($previous,$id) -ge 0){throw "Generated $Label ordering is not deterministic: $id"};$previous=$id}}
function AssertDimension([string]$Label,$Actual,[string]$ExpectedState,[string[]]$ExpectedIds){
    $expectedSorted=OrdinalSorted $ExpectedIds
    $ids=@($Actual.ids|ForEach-Object{[string]$_})
    if([int]$Actual.count -ne $expectedSorted.Count){throw "Coverage count disagrees with canonical facts: $Label expected $($expectedSorted.Count), found $($Actual.count)"}
    if($ids.Count -ne $expectedSorted.Count){throw "Coverage id list disagrees with its own count: $Label"}
    for($i=0;$i -lt $ids.Count;$i++){if($ids[$i] -cne $expectedSorted[$i]){throw "Coverage id list disagrees with canonical facts: $Label"}}
    if($Actual.state -cne $ExpectedState){throw "Coverage state disagrees with canonical facts: $Label expected $ExpectedState, found $($Actual.state)"}
}
$schemaText=[IO.File]::ReadAllText((Join-Path $RepositoryRoot 'schemas/knowledge-coverage.schema.yaml'))
$declaredStates=@([regex]::Matches([regex]::Match($schemaText,'(?ms)^coverage_states:\r?\n(?<b>(?:  - [a-z_]+\r?\n)*)').Groups['b'].Value,'(?m)^  - (?<v>[a-z_]+)\r?$')|ForEach-Object{$_.Groups['v'].Value})
$declaredCandidateTypes=@([regex]::Matches([regex]::Match($schemaText,'(?ms)^candidate_types:\r?\n(?<b>(?:  - [a-z_]+\r?\n)*)').Groups['b'].Value,'(?m)^  - (?<v>[a-z_]+)\r?$')|ForEach-Object{$_.Groups['v'].Value})
if($declaredStates.Count -eq 0 -or $declaredCandidateTypes.Count -eq 0){throw 'Coverage schema declares no states or candidate types.'}
$domainEnumBlock=[regex]::Match([IO.File]::ReadAllText((Join-Path $RepositoryRoot 'schemas/node.schema.yaml')),'(?ms)^  domain:\r?\n.*?    enum:\r?\n(?<b>(?:      - [a-z0-9-]+\r?\n)*)').Groups['b'].Value
$domainIds=@([regex]::Matches($domainEnumBlock,'(?m)^      - (?<v>[a-z0-9-]+)\r?$')|ForEach-Object{$_.Groups['v'].Value})
if($domainIds.Count -eq 0){throw 'Canonical node schema declares no domains.'}
$validStates=$declaredStates;foreach($n in $model.nodes){if(-not$nodeSet.Contains($n.id)){throw "Coverage node does not exist: $($n.id)"};foreach($d in @('sessions','sources','vocabulary','experiments','completed_experiment_runs')){if($n.$d.state -cnotin $validStates){throw "Impossible coverage state: $($n.$d.state)"}}}
$canonicalNodes=@{}
foreach($t in $nodeTexts){
    $fm=[regex]::Match($t,'(?s)\A---\r?\n(?<y>.*?)\r?\n---').Groups['y'].Value
    $id=[regex]::Match($fm,'(?m)^id: (?<v>[a-z0-9-]+)\r?$').Groups['v'].Value
    $edges=@()
    if($fm -notmatch '(?m)^relationships: \[\]\r?$'){$edges=@([regex]::Matches($fm,'(?m)^  - target: (?<t>[a-z0-9-]+)\r?\n    type: (?<y>[a-z0-9_]+)\r?$')|ForEach-Object{[pscustomobject]@{Target=$_.Groups['t'].Value;Type=$_.Groups['y'].Value}})}
    $canonicalNodes[$id]=[pscustomobject]@{Domain=[regex]::Match($fm,'(?m)^domain: (?<v>[a-z0-9-]+)\r?$').Groups['v'].Value;Sessions=@(CanonList $fm 'session_origin');Sources=@(CanonList $fm 'sources');Practical=@(CanonList $fm 'practical_applications');Edges=$edges}
}
$expectedCandidates=[Collections.Generic.Dictionary[string,object]]::new([StringComparer]::Ordinal)
$generatedNodeIds=@($model.nodes|ForEach-Object{[string]$_.id})
if($generatedNodeIds.Count -ne $canonicalNodes.Count){throw "Generated coverage node row count mismatch: expected $($canonicalNodes.Count), found $($generatedNodeIds.Count)"}
AssertOrdinalOrder 'node row' $generatedNodeIds
foreach($id in (OrdinalSorted @($canonicalNodes.Keys))){
    $n=$canonicalNodes[$id]
    $rows=@($model.nodes|Where-Object id -ceq $id)
    if($rows.Count -ne 1){throw "Generated coverage has $($rows.Count) rows for node: $id"}
    $row=$rows[0]
    $linkedVocabulary=@($vocab|Where-Object{$id -cin @($_.node_refs)})
    $linkedExperiments=@($experiments|Where-Object{$id -cin @($_.node_refs)})
    $completedRuns=@($runs|Where-Object{$_.status -ceq 'completed' -and $_.experiment_id -cin (IdList $linkedExperiments)})
    $inbound=@($canonicalNodes.Values|ForEach-Object{$_.Edges|Where-Object Target -ceq $id})
    if($row.domain -cne $n.Domain){throw "Coverage domain disagrees with canonical facts: $id"}
    AssertDimension "node $id sessions" $row.sessions $(if($n.Sessions.Count -gt 0){'covered'}else{'unlinked'}) $n.Sessions
    AssertDimension "node $id sources" $row.sources $(if($n.Sources.Count -gt 0){'covered'}else{'unlinked'}) $n.Sources
    AssertDimension "node $id vocabulary" $row.vocabulary $(if($linkedVocabulary.Count -gt 0){'covered'}else{'unlinked'}) (IdList $linkedVocabulary)
    $experimentState=if($n.Practical.Count -eq 0){'not_applicable'}elseif($linkedExperiments.Count -eq 0){'unlinked'}else{'covered'}
    $runState=if($n.Practical.Count -eq 0){'not_applicable'}elseif($completedRuns.Count -gt 0){'covered'}elseif($linkedExperiments.Count -gt 0){'partial'}else{'unlinked'}
    AssertDimension "node $id experiments" $row.experiments $experimentState (IdList $linkedExperiments)
    AssertDimension "node $id completed_experiment_runs" $row.completed_experiment_runs $runState (IdList $completedRuns)
    if([int]$row.inbound_relationships -ne $inbound.Count){throw "Coverage inbound relationship count disagrees with canonical facts: $id"}
    if([int]$row.outbound_relationships -ne $n.Edges.Count){throw "Coverage outbound relationship count disagrees with canonical facts: $id"}
    $expectedDiversity=@(@(@($n.Edges.Type)+@($inbound.Type))|Where-Object{$_}|Select-Object -Unique).Count
    if([int]$row.relationship_type_diversity -ne $expectedDiversity){throw "Coverage relationship type diversity disagrees with canonical facts: $id"}
    if($n.Sources.Count -eq 0){$expectedCandidates["source_coverage|node|$id"]=[ordered]@{source_count=0}}
    if($n.Sessions.Count -eq 0){$expectedCandidates["session_coverage|node|$id"]=[ordered]@{session_count=0}}
    if($linkedVocabulary.Count -eq 0){$expectedCandidates["vocabulary_bridge|node|$id"]=[ordered]@{vocabulary_count=0}}
    if($n.Practical.Count -gt 0 -and $linkedExperiments.Count -eq 0){$expectedCandidates["practical_evidence|node|$id"]=[ordered]@{practical_application_count=$n.Practical.Count;experiment_count=0}}
    if(($inbound.Count+$n.Edges.Count) -le 1){$expectedCandidates["relationship_coverage|node|$id"]=[ordered]@{inbound_count=$inbound.Count;outbound_count=$n.Edges.Count;total_connection_count=($inbound.Count+$n.Edges.Count)}}
}
$generatedDomainIds=@($model.domains|ForEach-Object{[string]$_.id})
if($generatedDomainIds.Count -ne $domainIds.Count){throw "Generated coverage domain row count mismatch: expected $($domainIds.Count), found $($generatedDomainIds.Count)"}
AssertOrdinalOrder 'domain row' $generatedDomainIds
foreach($d in (OrdinalSorted $domainIds)){
    $rows=@($model.domains|Where-Object id -ceq $d)
    if($rows.Count -ne 1){throw "Generated coverage has $($rows.Count) rows for domain: $d"}
    $row=$rows[0]
    $domainNodeIds=@($canonicalNodes.Keys|Where-Object{$canonicalNodes[$_].Domain -ceq $d})
    $expectedDomain=[ordered]@{node_count=$domainNodeIds.Count;vocabulary_count=@($vocab|Where-Object{$_.domain -ceq $d}).Count;experiment_count=@($experiments|Where-Object{@(@($_.node_refs)|Where-Object{$_ -cin $domainNodeIds}).Count -gt 0}).Count;source_count=@(@($domainNodeIds|ForEach-Object{$canonicalNodes[$_].Sources})|Select-Object -Unique).Count}
    foreach($p in $expectedDomain.GetEnumerator()){if([int]$row.($p.Key) -ne $p.Value){throw "Coverage domain $($p.Key) disagrees with canonical facts: $d expected $($p.Value), found $($row.($p.Key))"}}
    if($domainNodeIds.Count -le 1){$expectedCandidates["domain_representation|domain|$d"]=[ordered]@{node_count=$domainNodeIds.Count}}
}
$generatedSessionIds=@($model.sessions|ForEach-Object{[string]$_.id})
if($generatedSessionIds.Count -ne $sessionIds.Count){throw "Generated coverage session row count mismatch: expected $($sessionIds.Count), found $($generatedSessionIds.Count)"}
AssertOrdinalOrder 'session row' $generatedSessionIds
foreach($s in (OrdinalSorted $sessionIds)){
    $rows=@($model.sessions|Where-Object id -ceq $s)
    if($rows.Count -ne 1){throw "Generated coverage has $($rows.Count) rows for session: $s"}
    $row=$rows[0]
    $expectedSession=[ordered]@{mapped_node_count=@($canonicalNodes.Values|Where-Object{$s -cin $_.Sessions}).Count;vocabulary_count=@($vocab|Where-Object{$s -cin @($_.session_refs)}).Count;experiment_count=@($experiments|Where-Object{$s -cin @($_.session_refs)}).Count}
    foreach($p in $expectedSession.GetEnumerator()){if([int]$row.($p.Key) -ne $p.Value){throw "Coverage session $($p.Key) disagrees with canonical facts: $s expected $($p.Value), found $($row.($p.Key))"}}
}
$keys=IdSet @();$previous=''
foreach($c in $model.research_gap_candidates){
    foreach($field in @('candidate_type','subject_type','subject_id','evidence','reason')){if($null -eq $c.$field -or ('' -ceq [string]$c.$field -and $field -cne 'evidence')){throw "Candidate is missing required field '$field'."}}
    $key="$($c.candidate_type)|$($c.subject_type)|$($c.subject_id)"
    if($c.candidate_type -cnotin $declaredCandidateTypes){throw "Malformed candidate type: $key"}
    if($c.subject_type -cnotin @('node','domain')){throw "Malformed candidate subject type: $key"}
    if(-not $keys.Add($key)){throw "Duplicate gap candidate: $key"}
    if($c.subject_type -ceq 'node' -and -not $nodeSet.Contains($c.subject_id)){throw "Candidate subject does not exist: $key"}
    if($c.subject_type -ceq 'domain' -and $c.subject_id -cnotin $domainIds){throw "Candidate subject does not exist: $key"}
    if(-not $expectedCandidates.ContainsKey($key)){throw "Candidate is not supported by canonical facts: $key"}
    $expectedEvidence=$expectedCandidates[$key]
    $actualEvidence=@($c.evidence.PSObject.Properties)
    if($actualEvidence.Count -ne $expectedEvidence.Count){throw "Candidate evidence inconsistent with canonical facts: $key"}
    foreach($p in $expectedEvidence.GetEnumerator()){if($p.Key -cnotin @($actualEvidence|ForEach-Object{$_.Name})){throw "Candidate evidence inconsistent with canonical facts: $key is missing $($p.Key)"};if([int]$c.evidence.($p.Key) -ne $p.Value){throw "Candidate evidence inconsistent with canonical facts: $key expected $($p.Key)=$($p.Value), found $($c.evidence.($p.Key))"}}
    if($previous -and [StringComparer]::Ordinal.Compare($previous,$key) -gt 0){throw 'Generated candidate ordering is not deterministic.'}
    $previous=$key
}
foreach($key in $expectedCandidates.Keys){if(-not $keys.Contains($key)){throw "Missing research-gap candidate: $key"}}
$tmp=Join-Path ([IO.Path]::GetTempPath()) ('audiomuse-coverage-'+[guid]::NewGuid().ToString('N'));try{& (Join-Path $PSScriptRoot 'build-knowledge-coverage.ps1') -RepositoryRoot $RepositoryRoot -OutputDirectory $tmp|Out-Null;foreach($pair in @(@($ReportPath,(Join-Path $tmp 'knowledge-coverage.md')),@($JsonPath,(Join-Path $tmp 'knowledge-coverage.json')))){if([Convert]::ToBase64String([IO.File]::ReadAllBytes($pair[0]))-cne[Convert]::ToBase64String([IO.File]::ReadAllBytes($pair[1]))){throw "Generated coverage index is stale: $($pair[0])"}}}finally{if(Test-Path $tmp){Remove-Item $tmp -Recurse -Force}}
Write-Output "coverage_nodes: $($expected.node_count)";Write-Output "coverage_relationships: $($expected.relationship_count)";Write-Output "coverage_candidates: $($model.research_gap_candidates.Count)";Write-Output 'knowledge_coverage_current: true'
