# Переснимает скриншоты README: .\make.ps1 [-Shots home,detail] [-NoConvert]
# Нужны: собранный фронтенд (game-launcher\frontend\dist — build.ps1 или vite build),
# Node.js, Python с Pillow (pip install pillow) и Microsoft Edge.
param(
    [string[]]$Shots = @('home', 'detail', 'discreet', 'bulk', 'settings'),
    [switch]$NoConvert
)
$ErrorActionPreference = 'Stop'
$dir = Split-Path -Parent $MyInvocation.MyCommand.Path
$out = Join-Path $dir 'out'
$png = Join-Path $out 'png'
$images = Resolve-Path (Join-Path $dir '..\..\images')
$dist = Join-Path $dir '..\..\..\game-launcher\frontend\dist\index.html'
if (-not (Test-Path $dist)) { throw "Сначала соберите фронтенд: game-launcher\frontend\dist не найден" }

$edge = @("${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe", "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe") |
    Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $edge) { throw 'Microsoft Edge не найден' }
$profileDir = Join-Path $out 'edge-profile'

function Stop-Edge {
    # headless Edge иногда не завершается сам — добиваем процессы с нашим профилем
    Get-CimInstance Win32_Process -Filter "Name='msedge.exe'" | Where-Object { $_.CommandLine -like "*$profileDir*" } |
        ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}
function Shoot([string]$url, [string]$file, [string]$size) {
    Remove-Item $file -ErrorAction SilentlyContinue
    $args = @('--headless=new', '--disable-gpu', '--hide-scrollbars', '--no-first-run', "--user-data-dir=$profileDir",
        "--window-size=$size", '--virtual-time-budget=6000', "--screenshot=$file", $url)
    Start-Process -FilePath $edge -ArgumentList $args -WindowStyle Hidden | Out-Null
    $i = 0; while (-not (Test-Path $file) -and $i -lt 60) { Start-Sleep -Milliseconds 300; $i++ }
    Start-Sleep -Milliseconds 500
    Stop-Edge
    if (-not (Test-Path $file)) { throw "не удалось снять $file" }
    '{0,-12} {1,6:N0} KB' -f (Split-Path -Leaf $file), ((Get-Item $file).Length / 1KB)
}

Push-Location $dir
try {
    node art.cjs
    node build-ui.cjs
    New-Item -ItemType Directory -Force $png | Out-Null

    # Интерфейс с моком Wails отдаётся с localhost (пути /media/… абсолютные)
    $server = Start-Process python -ArgumentList '-m', 'http.server', '5393' -WorkingDirectory (Join-Path $out 'ui') -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 1
    try {
        foreach ($s in $Shots) { Shoot "http://localhost:5393/?shot=$s" (Join-Path $png "$s.png") '1440,900' }
    } finally { Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue }

    $banner = 'file:///' + ((Join-Path $dir 'banner.html') -replace '\\', '/')
    Shoot $banner (Join-Path $png 'banner.png') '1280,640'

    if (-not $NoConvert) {
        python convert.py $png $images
    }
} finally {
    Stop-Edge
    Pop-Location
}
