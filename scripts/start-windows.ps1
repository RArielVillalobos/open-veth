[CmdletBinding()]
param(
    [switch]$UpdateImages,
    [switch]$NoPause,
    [ValidateRange(1, 3600)][int]$StartupTimeoutSeconds = 180
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Invoke-Docker {
    param([string[]]$DockerArgs)
    & docker @DockerArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Docker command failed: docker $($DockerArgs -join ' ')"
    }
}

function Test-DockerImage {
    param([string]$Image)
    # PS 5.1 promotes redirected native stderr to errors under Stop, even for
    # expected failures such as a missing image. Only this probe suppresses it.
    $ErrorActionPreference = 'SilentlyContinue'
    & docker image inspect $Image *> $null
    return $LASTEXITCODE -eq 0
}

function Ensure-NodeImage {
    param([string]$Name, [bool]$Refresh, [ref]$BaseReady)
    $image = "openveth/${Name}:latest"
    $exists = Test-DockerImage $image
    if ($exists -and -not $Refresh) {
        Write-Host "Using local image: $image"
        return
    }

    Write-Host "Pulling $image..."
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        & docker pull $image
        $pullExitCode = $LASTEXITCODE
    } finally { $ErrorActionPreference = $previousPreference }
    if ($pullExitCode -eq 0) { return }
    if ($exists) {
        Write-Warning "Could not update $image; keeping the existing local image."
        return
    }

    if ($Name -in @('server', 'monitor', 'storage', 'zabbix') -and -not $BaseReady.Value) {
        if (-not (Test-DockerImage 'openveth/base:latest')) {
            Write-Host 'Building the shared base image...'
            Invoke-Docker @('build', '-t', 'openveth/base:latest', './images/base-node')
        }
        $BaseReady.Value = $true
    }
    Write-Host "Pull failed; building $image locally..."
    Invoke-Docker @('build', '-t', $image, "./images/$Name-node")
}

function Get-PortSettings {
    param([string[]]$Lines)
    $ports = @{ FRONTEND_PORT = 80; BACKEND_PORT = 8080 }
    foreach ($line in $Lines) {
        if ($line -match '^\s*(?:export\s+)?(FRONTEND_PORT|BACKEND_PORT)\s*=\s*(.*?)\s*$') {
            $key, $value = $Matches[1], $Matches[2]
            if ($value -notmatch '^[''"]?(\d+)[''"]?\s*(?:#.*)?$') {
                throw "Invalid $key in .env: expected a port number."
            }
            $port = 0
            if (-not [int]::TryParse($Matches[1], [ref]$port) -or $port -lt 1 -or $port -gt 65535) {
                throw "Invalid $key in .env: use a number between 1 and 65535."
            }
            $ports[$key] = $port
        }
    }
    foreach ($key in @('FRONTEND_PORT', 'BACKEND_PORT')) {
        $override = [Environment]::GetEnvironmentVariable($key, 'Process')
        if ([string]::IsNullOrWhiteSpace($override)) { continue }
        $port = 0
        if (-not [int]::TryParse($override, [ref]$port) -or $port -lt 1 -or $port -gt 65535) {
            throw "Invalid environment variable ${key}: use a port between 1 and 65535."
        }
        $ports[$key] = $port
    }
    return $ports
}

function Get-DockerPortOwners {
    param([string]$RepositoryPath)
    $owners = @{}
    $ids = @(Invoke-Docker @('ps', '-q'))
    if ($ids.Count -eq 0) { return $owners }
    $containers = (Invoke-Docker (@('inspect') + $ids)) -join "`n" | ConvertFrom-Json
    foreach ($container in $containers) {
        $name = $container.Name.TrimStart('/')
        $owner = 'other'
        if ($name -in @('openveth-backend', 'openveth-frontend')) {
            $labels = $container.Config.Labels
            if ($null -eq $labels) { throw "Container $name has no Compose ownership metadata. Resolve the name conflict before starting." }
            $service = $labels.PSObject.Properties['com.docker.compose.service']
            $directory = $labels.PSObject.Properties['com.docker.compose.project.working_dir']
            if ($null -eq $service -or $null -eq $directory -or
                $name -ne "openveth-$($service.Value)" -or
                $directory.Value.Replace('\', '/').TrimEnd('/') -ne $RepositoryPath.Replace('\', '/').TrimEnd('/')) {
                throw "Container $name belongs to another project. Resolve the name conflict before starting."
            }
            $owner = $service.Value
        }
        if ($null -eq $container.NetworkSettings.Ports) { continue }
        foreach ($property in $container.NetworkSettings.Ports.PSObject.Properties) {
            if ($property.Name -notlike '*/tcp') { continue }
            foreach ($binding in @($property.Value)) {
                if ($null -eq $binding) { continue }
                $port = [int]$binding.HostPort
                if ($owners.ContainsKey($port) -and $owners[$port] -ne $owner) {
                    $owners[$port] = 'other'
                } else {
                    $owners[$port] = $owner
                }
            }
        }
    }
    return $owners
}

function Test-LocalPort {
    param([int]$Port)
    foreach ($address in @([Net.IPAddress]::Any, [Net.IPAddress]::IPv6Any)) {
        $listener = $null
        try {
            $listener = [Net.Sockets.TcpListener]::new($address, $Port)
            if ($address.AddressFamily -eq [Net.Sockets.AddressFamily]::InterNetworkV6) {
                $listener.Server.DualMode = $false
            }
            $listener.ExclusiveAddressUse = $true
            $listener.Start()
        } catch [Net.Sockets.SocketException] {
            if ($_.Exception.SocketErrorCode -ne [Net.Sockets.SocketError]::AddressFamilyNotSupported) {
                return $false
            }
        } finally {
            if ($null -ne $listener) { $listener.Stop() }
        }
    }
    return $true
}

function Find-OpenVethPort {
    param([int]$Preferred, [int]$First, [int]$Last, [string]$Service, [hashtable]$Owners, [int]$Reserved = 0)
    foreach ($port in @($Preferred) + @($First..$Last)) {
        if ($port -eq $Reserved) { continue }
        if ($Owners.ContainsKey($port)) {
            if ($Owners[$port] -eq $Service) { return $port }
            continue
        }
        if (Test-LocalPort $port) { return $port }
    }
    throw "No available port for $Service. Free port $Preferred or one in the range ${First}-${Last}."
}

function Save-PortSettings {
    param([string]$Path, [string[]]$Lines, [hashtable]$Ports)
    $seen = @{}
    $updated = @(
        foreach ($line in $Lines) {
            if ($line -match '^\s*(?:export\s+)?(FRONTEND_PORT|BACKEND_PORT)\s*=') {
                $key = $Matches[1]
                if (-not $seen.ContainsKey($key)) { "$key=$($Ports[$key])"; $seen[$key] = $true }
            } else { $line }
        }
        foreach ($key in @('FRONTEND_PORT', 'BACKEND_PORT')) {
            if (-not $seen.ContainsKey($key)) { "$key=$($Ports[$key])" }
        }
    )
    # Preserve unrelated settings and avoid the UTF-16 default of Windows PowerShell.
    [IO.File]::WriteAllLines($Path, [string[]]$updated, [Text.UTF8Encoding]::new($false))
}

function Test-HttpEndpoint {
    param([string]$Url)
    try {
        $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 3 -ErrorAction Stop
        return $response.StatusCode -eq 200
    } catch { return $false }
}

function Wait-OpenVeth {
    param([int]$BackendPort, [int]$FrontendPort, [int]$TimeoutSeconds)
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        if ((Test-HttpEndpoint "http://localhost:$BackendPort/health") -and
            (Test-HttpEndpoint "http://localhost:$FrontendPort/") -and
            (Test-HttpEndpoint "http://localhost:$FrontendPort/api/v1/laboratories")) {
            return
        }
        if ([DateTime]::UtcNow -ge $deadline) { break }
        Start-Sleep -Seconds 2
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "OpenVeth did not become ready within $TimeoutSeconds seconds. Containers have been left available for diagnosis."
}

function Start-OpenVeth {
    param([string]$RepositoryPath, [bool]$RefreshImages, [int]$TimeoutSeconds)
    $started = $false
    $compose = @('compose', '--project-directory', $RepositoryPath, '-f', (Join-Path $RepositoryPath 'docker-compose.yml'))
    Push-Location -LiteralPath $RepositoryPath
    try {
        Write-Host '========================================'
        Write-Host ' OpenVeth - Starting...'
        Write-Host '========================================'
        if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
            throw 'Docker CLI was not found. Install Docker Desktop and reopen this terminal.'
        }
        Write-Host 'Checking Docker and Compose...'
        $info = (Invoke-Docker @('info', '--format', '{{json .}}')) -join "`n" | ConvertFrom-Json
        if ($info.OSType -ne 'linux') { throw 'Switch Docker Desktop to Linux containers with the WSL 2 engine enabled.' }
        if ($info.KernelVersion -notmatch '(?i)(microsoft|wsl)') {
            throw 'The Docker kernel does not identify as WSL. Enable the WSL 2 based engine in Docker Desktop settings.'
        }
        Invoke-Docker @('compose', 'version')
        Invoke-Docker ($compose + @('config', '--quiet'))

        $baseReady = $false
        foreach ($name in @('host', 'router', 'switch', 'server', 'monitor', 'tester', 'haproxy', 'storage', 'zabbix')) {
            Ensure-NodeImage -Name $name -Refresh $RefreshImages -BaseReady ([ref]$baseReady)
        }
        Write-Host 'Building application images before changing running services...'
        Invoke-Docker ($compose + @('build'))

        $envPath = Join-Path $RepositoryPath '.env'
        $lines = @()
        if (Test-Path -LiteralPath $envPath) { $lines = @(Get-Content -LiteralPath $envPath -Encoding UTF8) }
        $ports = Get-PortSettings $lines
        $owners = Get-DockerPortOwners $RepositoryPath
        $ports.FRONTEND_PORT = Find-OpenVethPort $ports.FRONTEND_PORT 8080 8100 'frontend' $owners
        $ports.BACKEND_PORT = Find-OpenVethPort $ports.BACKEND_PORT 8081 8200 'backend' $owners $ports.FRONTEND_PORT
        Save-PortSettings -Path $envPath -Lines $lines -Ports $ports
        # Process variables take precedence over .env in Compose; keep them consistent.
        $env:FRONTEND_PORT = [string]$ports.FRONTEND_PORT
        $env:BACKEND_PORT = [string]$ports.BACKEND_PORT
        Write-Host "Ports: frontend=$($ports.FRONTEND_PORT), backend=$($ports.BACKEND_PORT)"

        $started = $true
        Invoke-Docker ($compose + @('up', '-d', '--no-build', '--timeout', '30'))
        Write-Host "Waiting for the API, frontend, and API proxy (up to $TimeoutSeconds seconds)..."
        Wait-OpenVeth $ports.BACKEND_PORT $ports.FRONTEND_PORT $TimeoutSeconds
        Write-Host ''
        Write-Host 'OpenVeth is ready!'
        Write-Host "Frontend: http://localhost:$($ports.FRONTEND_PORT)"
        Write-Host "Backend:  http://localhost:$($ports.BACKEND_PORT)"
    } catch {
        $startupError = $_
        if ($started) {
            Write-Host 'Startup failed. Container status and recent logs:'
            try {
                $ErrorActionPreference = 'Continue'
                & docker @compose ps
                & docker @compose logs --tail 60 backend frontend
            } catch {
                Write-Warning 'Could not retrieve Docker diagnostics.'
            } finally { $ErrorActionPreference = 'Stop' }
        }
        throw $startupError
    } finally {
        Pop-Location
    }
}

# Dot sourcing exposes functions for isolated tests without starting Docker.
if ($MyInvocation.InvocationName -ne '.') {
    $exitCode = 0
    try {
        Start-OpenVeth -RepositoryPath (Split-Path -Parent $PSScriptRoot) -RefreshImages $UpdateImages.IsPresent -TimeoutSeconds $StartupTimeoutSeconds
    } catch {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
        $exitCode = 1
    }
    if (-not $NoPause) { [void](Read-Host 'Press Enter to close') }
    exit $exitCode
}
