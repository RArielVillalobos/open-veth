# Standalone regression tests; no Docker daemon or external test modules needed.
# Run with: powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/start-windows.tests.ps1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/start-windows.ps1" -NoPause

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw "Assertion failed: $Message" }
}

function Assert-Fails {
    param([scriptblock]$Action, [string]$Message)
    $failed = $false
    try { & $Action } catch { $failed = $true }
    Assert-True $failed $Message
}

function Reset-DockerMock {
    $script:Calls = [Collections.Generic.List[string]]::new()
    $script:LocalImages = @{}
    $script:PullFails = $false
    $script:BuildFails = $false
    $script:UpFails = $false
    $script:ContainerJson = '[]'
    $script:ContainerIDs = @()
    $script:Engine = 'linux'
    $script:HttpCalls = [Collections.Generic.List[string]]::new()
    $script:HttpReady = $true
    $script:BusyPorts = @{}
}

function docker {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$DockerArguments)
    $command = $DockerArguments -join ' '
    $script:Calls.Add($command)
    $global:LASTEXITCODE = 0
    switch ($DockerArguments[0]) {
        'info' { @{ OSType = $script:Engine; KernelVersion = '6.6.0-microsoft-standard-WSL2' } | ConvertTo-Json -Compress; return }
        'image' {
            if (-not $script:LocalImages.ContainsKey($DockerArguments[2])) { $global:LASTEXITCODE = 1 }
            return
        }
        'pull' { if ($script:PullFails) { $global:LASTEXITCODE = 1 }; return }
        'ps' { $script:ContainerIDs; return }
        'inspect' { $script:ContainerJson; return }
        'compose' {
            if ($DockerArguments[-1] -eq 'build' -and $script:BuildFails) { $global:LASTEXITCODE = 1 }
            if ('up' -in $DockerArguments -and $script:UpFails) { $global:LASTEXITCODE = 1 }
            return
        }
    }
}

$realPortCheck = ${function:Test-LocalPort}
function Test-LocalPort { param([int]$Port); return -not $script:BusyPorts.ContainsKey($Port) }
function Test-HttpEndpoint { param([string]$Url); $script:HttpCalls.Add($Url); return $script:HttpReady }

$originalFrontendPort = $env:FRONTEND_PORT
$originalBackendPort = $env:BACKEND_PORT
$testDirectory = Join-Path ([IO.Path]::GetTempPath()) ("openveth-start-tests-" + [guid]::NewGuid())
$envPath = Join-Path $testDirectory '.env'
[void](New-Item -ItemType Directory -Path $testDirectory)
try {
    $env:FRONTEND_PORT = $null
    $env:BACKEND_PORT = $null
    Reset-DockerMock
    $ports = Get-PortSettings @('# comment', 'FRONTEND_PORT="8088"', 'BACKEND_PORT=9090 # API')
    Assert-True ($ports.FRONTEND_PORT -eq 8088 -and $ports.BACKEND_PORT -eq 9090) 'Read quoted and commented .env ports'
    Assert-Fails { Get-PortSettings @('FRONTEND_PORT=0') } 'Reject invalid ports'

    Save-PortSettings $envPath @('TOKEN=keep-me', 'FRONTEND_PORT=80', 'FRONTEND_PORT=81', '# keep comment') $ports
    $saved = Get-Content -LiteralPath $envPath
    Assert-True ($saved -contains 'TOKEN=keep-me' -and $saved -contains '# keep comment') 'Preserve unrelated .env settings'
    Assert-True (@($saved | Where-Object { $_ -match '^FRONTEND_PORT=' }).Count -eq 1) 'Remove duplicate port definitions'

    $script:BusyPorts[80] = $true
    $port = Find-OpenVethPort 80 8080 8082 'frontend' @{}
    Assert-True ($port -eq 8080) 'Find another port when preferred port is occupied'
    Assert-True ((Find-OpenVethPort 8080 8080 8082 'backend' @{} 8080) -eq 8081) 'Frontend and backend must use different ports'
    Assert-True ((Find-OpenVethPort 80 8080 8082 'frontend' @{ 80 = 'frontend' }) -eq 80) 'Reuse a port already owned by this service'
    Assert-True ((Find-OpenVethPort 80 8080 8082 'frontend' @{ 80 = 'other' }) -eq 8080) 'Never reuse another container port'
    Assert-Fails { Find-OpenVethPort 80 8080 8081 'frontend' @{80='other'; 8080='other'; 8081='other'} } 'Fail clearly when no candidate ports are free'

    Reset-DockerMock
    $script:PullFails = $true
    $baseReady = $false
    Ensure-NodeImage 'server' $false ([ref]$baseReady)
    Ensure-NodeImage 'monitor' $false ([ref]$baseReady)
    $commands = @($script:Calls)
    Assert-True ($commands.IndexOf('build -t openveth/base:latest ./images/base-node') -lt $commands.IndexOf('build -t openveth/server:latest ./images/server-node')) 'Build base before dependent image'
    Assert-True (@($commands | Where-Object { $_ -eq 'build -t openveth/base:latest ./images/base-node' }).Count -eq 1) 'Build base only once'

    Reset-DockerMock
    $script:LocalImages['openveth/host:latest'] = $true
    $baseReady = $false
    Ensure-NodeImage 'host' $false ([ref]$baseReady)
    Assert-True (@($script:Calls | Where-Object { $_ -like 'pull *' }).Count -eq 0) 'Normal startup must not pull local images'
    $script:PullFails = $true
    Ensure-NodeImage 'host' $true ([ref]$baseReady)
    Assert-True (@($script:Calls | Where-Object { $_ -like 'build *' }).Count -eq 0) 'Failed update must keep usable local image'

    Reset-DockerMock
    $script:ContainerIDs = @('backend-id')
    $script:ContainerJson = @(@{
        Name = '/openveth-backend'
        Config = @{ Labels = @{'com.docker.compose.service' = 'backend'; 'com.docker.compose.project.working_dir' = $testDirectory.Replace('\', '/')} }
        NetworkSettings = @{ Ports = @{'8080/tcp' = @(@{HostPort = '8080'})} }
    }) | ConvertTo-Json -Depth 8
    $owners = Get-DockerPortOwners $testDirectory
    Assert-True ($owners[8080] -eq 'backend') 'Recognize own running container and normalize Windows paths'
    Assert-Fails { Get-DockerPortOwners (Join-Path $testDirectory 'another-project') } 'Reject same container name owned by another project'

    Reset-DockerMock
    foreach ($name in @('host', 'router', 'switch', 'server', 'monitor', 'tester', 'haproxy', 'storage', 'zabbix')) {
        $script:LocalImages["openveth/${name}:latest"] = $true
    }
    Start-OpenVeth $testDirectory $false 1
    Assert-True (@($script:Calls | Where-Object { $_ -like 'rm *' }).Count -eq 0) 'Never force-remove running services'
    $buildIndex = $script:Calls.FindIndex([Predicate[string]]{ param($s) $s -like 'compose * build' })
    $upIndex = $script:Calls.FindIndex([Predicate[string]]{ param($s) $s -like 'compose * up *' })
    Assert-True ($buildIndex -ge 0 -and $upIndex -gt $buildIndex) 'Build successfully before updating services'
    Assert-True ($script:HttpCalls.Count -eq 3) 'Verify API health, frontend, and API proxy'

    $script:Calls.Clear()
    $script:BuildFails = $true
    Assert-Fails { Start-OpenVeth $testDirectory $false 1 } 'Build failure must stop startup'
    Assert-True (@($script:Calls | Where-Object { $_ -like 'compose * up *' }).Count -eq 0) 'Never touch services after a failed build'
    $script:BuildFails = $false
    $script:UpFails = $true
    $script:Calls.Clear()
    Assert-Fails { Start-OpenVeth $testDirectory $false 1 } 'Compose startup failure must be reported'
    Assert-True (@($script:Calls | Where-Object { $_ -like 'compose * logs --tail 60 backend frontend' }).Count -eq 1) 'Show diagnostics after failed startup'
    $script:HttpReady = $false
    Assert-Fails { Wait-OpenVeth 8080 80 1 } 'Timeout must fail instead of announcing readiness'

    Reset-DockerMock
    $script:Engine = 'windows'
    Assert-Fails { Start-OpenVeth $testDirectory $false 1 } 'Reject Windows container engine before changing resources'
    Write-Host 'All Windows startup regression tests passed.' -ForegroundColor Green
} finally {
    $env:FRONTEND_PORT = $originalFrontendPort
    $env:BACKEND_PORT = $originalBackendPort
    # This test creates only .env in its unique temporary directory.
    if (Test-Path -LiteralPath $envPath) { Remove-Item -LiteralPath $envPath }
    Remove-Item -LiteralPath $testDirectory
    Set-Item Function:Test-LocalPort $realPortCheck
}
