<#
.SYNOPSIS
    Prepare a Windows machine to work on this repository.

.DESCRIPTION
    Installs Go module dependencies, the pinned dev tools, pre-commit and the
    git hooks. Safe to re-run. The macOS and Linux equivalent is
    scripts/bootstrap.sh.

    GNU Make is optional but recommended; without it, use the `go` commands
    this script prints at the end.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

function Write-Step($message) { Write-Host "==> $message" -ForegroundColor Cyan }
function Write-Warn($message) { Write-Host "warn $message" -ForegroundColor Yellow }

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

function Test-Command($name) {
    return [bool](Get-Command $name -ErrorAction SilentlyContinue)
}

# ------------------------------------------------------------------------ Go
if (-not (Test-Command go)) {
    throw 'Go is not installed. Install it with: winget install GoLang.Go'
}

$requiredMinor = (Select-String -Path go.mod -Pattern '^go (\d+)\.(\d+)').Matches[0].Groups[2].Value
$actualMinor = ((go env GOVERSION) -replace '^go', '').Split('.')[1]
if ([int]$actualMinor -lt [int]$requiredMinor) {
    throw "Go 1.$requiredMinor or newer is required, found $(go env GOVERSION)"
}
Write-Step "Go $(go env GOVERSION) on $(go env GOOS)/$(go env GOARCH)"

Write-Step 'downloading modules'
go mod download

Write-Step 'installing pinned dev tools into .\bin'
$golangciVersion = (Select-String -Path Makefile -Pattern 'GOLANGCI_LINT_VERSION := (\S+)').Matches[0].Groups[1].Value
$env:GOBIN = Join-Path $repoRoot 'bin'
go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$golangciVersion"

# ---------------------------------------------------------------- pre-commit
function Install-PreCommit {
    if (Test-Command pre-commit) {
        Write-Step 'pre-commit is already installed'
        return $true
    }
    # Try the tool managers most likely to be present, best first.
    foreach ($candidate in @(
        @{ Tool = 'uv';    Args = @('tool', 'install', 'pre-commit') },
        @{ Tool = 'pipx';  Args = @('install', 'pre-commit') },
        @{ Tool = 'scoop'; Args = @('install', 'pre-commit') }
    )) {
        if (Test-Command $candidate.Tool) {
            Write-Step "installing pre-commit with $($candidate.Tool)"
            & $candidate.Tool @($candidate.Args)
            if ($LASTEXITCODE -eq 0) { return $true }
        }
    }
    foreach ($py in @('python', 'python3', 'py')) {
        if (Test-Command $py) {
            Write-Step "installing pre-commit with $py -m pip --user"
            & $py -m pip install --user --upgrade pre-commit
            if ($LASTEXITCODE -eq 0) { return $true }
        }
    }
    return $false
}

if ((Install-PreCommit) -and (Test-Command pre-commit)) {
    Write-Step 'installing git hooks'
    pre-commit install --install-hooks
} else {
    Write-Warn 'could not install pre-commit automatically.'
    Write-Warn 'install it from https://pre-commit.com/#install and run: pre-commit install --install-hooks'
    Write-Warn 'if pre-commit is installed but not on PATH, add the Python user scripts directory to PATH.'
}

Write-Step 'done.'
Write-Host ''
Write-Host 'With GNU Make (winget install ezwinports.make):'
Write-Host '  make check'
Write-Host ''
Write-Host 'Without Make:'
Write-Host '  .\bin\golangci-lint.exe run'
Write-Host '  go test ./internal/...'
Write-Host '  go tool ginkgo --randomize-all -p .\test\functional'
Write-Host '  go tool ginkgo --randomize-all .\test\e2e'
