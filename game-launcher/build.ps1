# Сборка pLauncher одной командой.
#
# Обходит проблему окружения: внутри `wails build` не находятся npm/vite
# (npm резолвится в npm.ps1, который не запускается дочерним процессом),
# поэтому фронтенд собираем сами через node, а wails запускаем с -s (skip frontend).
#
#   .\build.ps1              — тесты + фронтенд + exe
#   .\build.ps1 -NoTests     — без go test
#   .\build.ps1 -Clean       — очистить build/bin перед сборкой
#
# Результат: build\bin\pLauncher.exe

param(
    [switch]$NoTests,
    [switch]$Clean
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

function Step($name) { Write-Host "`n==> $name" -ForegroundColor Cyan }
function Check($what) { if ($LASTEXITCODE -ne 0) { throw "$what завершился с кодом $LASTEXITCODE" } }

# Версия берётся из wails.json (productVersion) и вшивается в бинарник
$version = (Get-Content wails.json -Raw | ConvertFrom-Json).info.productVersion

# Go — той же версии, что в go.mod (её же ставит CI). Wails v2.12 разбирает код старой библиотекой
# golang.org/x/tools, которая не читает стандартную библиотеку более новых Go («package "fmt" without
# types»), поэтому установленный в системе Go 1.27+ ломал бы генерацию привязок. Тулчейн скачается сам.
$goVersion = (Select-String -Path go.mod -Pattern '^go\s+(\S+)').Matches[0].Groups[1].Value
if (-not $env:GOTOOLCHAIN) { $env:GOTOOLCHAIN = "go$goVersion" }

if (-not $NoTests) {
    Step 'go test'
    go test ./...
    Check 'go test'
}

Step 'frontend (vite)'
Push-Location frontend
try {
    if (-not (Test-Path node_modules\vite\bin\vite.js)) {
        npm.cmd install
        Check 'npm install'
    }
    node node_modules\vite\bin\vite.js build
    Check 'vite build'
} finally {
    Pop-Location
}

Step "wails build v$version"
$wailsArgs = @('build', '-s', '-ldflags', "-X main.appVersion=$version")
if ($Clean) { $wailsArgs += '-clean' }
wails @wailsArgs
Check 'wails build'

$exe = Join-Path $PSScriptRoot 'build\bin\pLauncher.exe'
Write-Host "`nГотово: $exe ($((Get-Item $exe).LastWriteTime))" -ForegroundColor Green
