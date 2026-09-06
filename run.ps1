$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$nodeDir = "C:\Program Files\nodejs"
if (Test-Path (Join-Path $nodeDir "npm.cmd")) {
    $env:Path = "$nodeDir;" + $env:Path
}

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    Write-Host "npm is required for a local frontend build. Install Node.js LTS, or run: docker compose up --build"
    exit 1
}

Set-Location frontend
if (-not (Test-Path node_modules)) {
    npm install
}
npm run build
Set-Location ..

Set-Location backend
go run ./cmd/server
