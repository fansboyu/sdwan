param([ValidateRange(30,600)][int]$TimeoutSeconds = 210)

$ErrorActionPreference = 'Stop'
$controller = 'http://127.0.0.1:18181'
$branch = 'sdwan-lab-branch-east-1'
$relay = 'sdwan-lab-lab-relay-1'
$hubIP = '10.254.10.10'
$url = 'http://192.168.250.10:8080/'
$password = if ($env:LAB_ADMIN_PASSWORD) { $env:LAB_ADMIN_PASSWORD } else { 'LabNetwork2026!' }
$login = Invoke-RestMethod "$controller/admin/auth/login" -Method Post -ContentType 'application/json' -Body (@{email='lab-admin@sdwan.test';password=$password} | ConvertTo-Json -Compress)
$headers = @{Authorization="Bearer $($login.token)"}
function Account { Invoke-RestMethod "$controller/admin/account" -Headers $headers -TimeoutSec 5 }
function Mode($value) { Invoke-RestMethod "$controller/admin/path-mode" -Headers $headers -Method Post -ContentType 'application/json' -Body (@{mode=$value} | ConvertTo-Json -Compress) | Out-Null }

$before = Account
if (!$before.relay_healthy) { throw 'Lab Relay is not ready; wait for its first WireGuard handshake.' }
if ($before.active_relay.endpoint -ne '10.254.10.250:51873' -or $before.user.path_mode -notin @('auto','direct')) {
    throw 'Expected the dedicated Docker Relay and an auto/direct demo account'
}
$device = @(docker exec sdwan-lab-postgres-1 psql -U sdwan -d sdwan -Atc "select id from devices where hostname='branch-east' and status='active'")
if ($LASTEXITCODE -ne 0 -or $device.Count -ne 1) { throw 'Expected one active branch-east device' }
$deviceId = $device[0]

function DirectBlock([bool]$enabled) {
    $operation = if ($enabled) { '-I' } else { '-D' }
    foreach ($rule in @(@('OUTPUT','-d'), @('INPUT','-s'))) {
        docker exec $branch iptables $operation $rule[0] $rule[1] $hubIP -p udp -m comment --comment sdwan-lab-direct-fault -j DROP
        if ($enabled -and $LASTEXITCODE -ne 0) { throw 'Could not install the local fault rule' }
    }
}

function Measure-LabPhase($phase, $expected) {
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $firstFailure = $null
    $longest = 0.0
    $failures = 0
    $samples = 0
    $path = $null
    while ($clock.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        $started = $clock.Elapsed.TotalSeconds
        docker exec $branch curl -fsS --max-time 1 $url 2>$null | Out-Null
        $ok = $LASTEXITCODE -eq 0
        $samples++
        if (!$ok) {
            $failures++
            if ($null -eq $firstFailure) { $firstFailure = $started }
        } elseif ($null -ne $firstFailure) {
            $longest = [Math]::Max($longest, $started - $firstFailure)
            $firstFailure = $null
        }
        if (($samples -eq 1) -or ($samples % 4 -eq 0)) {
            $path = (Account).peer_paths | Where-Object client_device_id -eq $deviceId
            if ($ok -and $path.current_path -eq $expected -and $path.desired_path -eq $expected -and
                $path.verification_state -eq 'verified' -and $path.client_applied_generation -eq $path.generation -and $path.hub_applied_generation -eq $path.generation) { break }
        }
        Start-Sleep -Milliseconds 450
    }
    if ($null -ne $firstFailure) { $longest = [Math]::Max($longest, $clock.Elapsed.TotalSeconds - $firstFailure) }
    $reached = $clock.Elapsed.TotalSeconds -lt $TimeoutSeconds
    [pscustomobject]@{phase=$phase; reached=$reached; samples=$samples; failed=$failures; longestObservedFailureSeconds=[Math]::Round($longest,2);
        phaseSeconds=[Math]::Round($clock.Elapsed.TotalSeconds,2); current=$path.current_path; target=$path.desired_path;
        state=$path.state; verification=$path.verification_state; generation=$path.generation;
        clientGeneration=$path.client_applied_generation; hubGeneration=$path.hub_applied_generation} | ConvertTo-Json -Compress | Write-Host
    return $reached
}

$blocked = $false
$stopped = $false
try {
    Mode 'auto'
    if (!(Measure-LabPhase 'baseline' 'direct')) { throw 'Direct baseline failed' }
    $blocked = $true
    DirectBlock $true
    if (!(Measure-LabPhase 'direct-blocked' 'relay')) { throw 'Direct-to-Relay did not converge' }
    DirectBlock $false
    $blocked = $false
    if (!(Measure-LabPhase 'direct-restored' 'direct')) { throw 'Relay-to-direct did not converge' }
    $blocked = $true
    DirectBlock $true
    if (!(Measure-LabPhase 'relay-outage-setup' 'relay')) { throw 'Could not re-establish Relay before the outage' }
    DirectBlock $false
    $blocked = $false
    $stopped = $true
    docker stop -t 1 $relay | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Could not stop lab Relay' }
    if (!(Measure-LabPhase 'relay-stopped' 'direct')) { throw 'Relay outage did not converge to verified direct' }
} finally {
    if ($blocked) { DirectBlock $false }
    if ($stopped) { docker start $relay | Out-Null }
    if ($before.user.path_mode -eq 'direct') { Mode 'direct' }
}
