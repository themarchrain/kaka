param(
    [switch]$Race,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$RemainingArguments
)

$ErrorActionPreference = "Stop"

if ($RemainingArguments.Count -gt 0) {
    Write-Error "usage: .\scripts\test.ps1 [-Race]"
    exit 2
}

$root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$modules = @(
    ".",
    "middleware/gin",
    "middleware/http",
    "examples/http-example",
    "examples/gin-example"
)

function Invoke-GoTest {
    param(
        [string[]]$Arguments
    )

    go @Arguments
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

foreach ($module in $modules) {
    Push-Location (Join-Path $root $module)
    try {
        Write-Host "==> go test -count=1 ./... ($module)"
        Invoke-GoTest @("test", "-count=1", "./...")
    } finally {
        Pop-Location
    }
}

if ($Race) {
    Push-Location $root
    $previousCGOEnabled = $env:CGO_ENABLED
    try {
        Write-Host "==> go test -race -count=1 ./... (.)"
        $env:CGO_ENABLED = "1"
        Invoke-GoTest @("test", "-race", "-count=1", "./...")
    } finally {
        $env:CGO_ENABLED = $previousCGOEnabled
        Pop-Location
    }
}
