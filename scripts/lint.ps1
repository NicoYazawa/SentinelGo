<#
.SYNOPSIS
    SentinelGo backend style check: gofmt + build + vet + goimports + golangci-lint.

.DESCRIPTION
    Portable replacement for a pre-commit hook, usable locally and in CI
    (including environments without a git checkout).

    Messages are kept ASCII-only on purpose: Windows PowerShell 5.1 misreads
    UTF-8 files without a BOM, which breaks string literals containing CJK text.

.PARAMETER Fix
    Auto-fix what can be fixed (gofmt / goimports) instead of only reporting.

.EXAMPLE
    ./scripts/lint.ps1 -Fix
    ./scripts/lint.ps1
#>
param(
    [switch]$Fix
)

$ErrorActionPreference = 'Stop'
$backend = Join-Path $PSScriptRoot '..\backend'
Set-Location $backend

function Invoke-Step {
    param([string]$Title, [scriptblock]$Body)
    Write-Host "`n==> $Title" -ForegroundColor Cyan
    & $Body
    if ($LASTEXITCODE -ne 0) {
        Write-Host "    [FAIL] $Title" -ForegroundColor Red
        exit $LASTEXITCODE
    }
    Write-Host "    [OK] $Title" -ForegroundColor Green
}

Invoke-Step 'gofmt' {
    if ($Fix) {
        gofmt -w .
    } else {
        $dirty = @(gofmt -l .)
        if ($dirty.Count -gt 0) {
            Write-Host "    $($dirty.Count) file(s) not gofmt-ed; rerun with -Fix:" -ForegroundColor Yellow
            $dirty | ForEach-Object { Write-Host "      $_" -ForegroundColor Yellow }
            exit 1
        }
    }
}

Invoke-Step 'build' { go build ./... }
Invoke-Step 'vet'   { go vet ./... }

# goimports groups stdlib / third-party / local (sentinelgo) imports.
if (Get-Command goimports -ErrorAction SilentlyContinue) {
    Invoke-Step 'goimports (local sentinelgo)' {
        if ($Fix) {
            goimports -local sentinelgo -w .
        } else {
            $dirty = @(goimports -local sentinelgo -l .)
            if ($dirty.Count -gt 0) {
                Write-Host "    $($dirty.Count) file(s) with unsorted imports; rerun with -Fix:" -ForegroundColor Yellow
                $dirty | ForEach-Object { Write-Host "      $_" -ForegroundColor Yellow }
                exit 1
            }
        }
    }
} else {
    Write-Host "`n==> goimports [SKIP] not installed (go install golang.org/x/tools/cmd/goimports@latest)" -ForegroundColor DarkGray
}

if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
    Invoke-Step 'golangci-lint' { golangci-lint run ./... }
} else {
    Write-Host "`n==> golangci-lint [SKIP] not installed (go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest)" -ForegroundColor DarkGray
}

Write-Host "`nAll checks passed." -ForegroundColor Green