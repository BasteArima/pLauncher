# Переснимает скриншоты README: .\make.ps1 [-Shots home,detail] [-NoConvert]
# Нужны: собранный фронтенд (game-launcher\frontend\dist — build.ps1 или vite build),
# Node.js, Python с Pillow (pip install pillow) и Microsoft Edge.
param(
    [ValidateSet('home','detail','discreet','bulk','settings','find','stats','launch','editor','smart')]
    # editor и smart в README не входят, но снимаются по запросу: -Shots editor,smart
    [string[]]$Shots = @('home', 'detail', 'discreet', 'bulk', 'settings', 'find', 'stats', 'launch'),
    [int]$Port = 5393,
    [int]$CDPPort = 9336,
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
    Remove-Item -LiteralPath $file -ErrorAction SilentlyContinue
    $args = @('--headless=new', '--disable-gpu', '--hide-scrollbars', '--no-first-run', "--user-data-dir=$profileDir",
        '--disable-background-timer-throttling', '--disable-renderer-backgrounding', "--remote-debugging-port=$CDPPort", 'about:blank')
    Start-Process -FilePath $edge -ArgumentList $args -WindowStyle Hidden | Out-Null
    try {
        node (Join-Path $dir 'capture-readme.cjs') $url $file $size $CDPPort
        if ($LASTEXITCODE -ne 0) { throw "Screenshot verification failed: $url" }
    } finally { Stop-Edge }
    '{0,-12} {1,6:N0} KB' -f (Split-Path -Leaf $file), ((Get-Item $file).Length / 1KB)
}

# Do not attach to an unrelated service/browser when a default port is occupied.
foreach ($number in @($Port, $CDPPort)) {
    if (Get-NetTCPConnection -State Listen -LocalPort $number -ErrorAction SilentlyContinue) {
        throw "Port $number is busy; choose -Port / -CDPPort for this run"
    }
}

Push-Location $dir
try {
    node art.cjs
    if ($LASTEXITCODE -ne 0) { throw "Artwork generation failed" }
    node build-ui.cjs
    if ($LASTEXITCODE -ne 0) { throw "Mock UI build failed" }
    New-Item -ItemType Directory -Force $png | Out-Null

    # Интерфейс с моком Wails отдаётся с localhost (пути /media/… абсолютные)
    $server = Start-Process python -ArgumentList '-m', 'http.server', $Port, '--bind', '127.0.0.1' -WorkingDirectory (Join-Path $out 'ui') -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 1
    try {
        foreach ($s in $Shots) {
            $size = switch ($s) { 'detail' { '1440,1330' } 'editor' { '1440,1500' } 'find' { '1440,1220' } { $_ -in @('stats','launch') } { '1440,1100' } default { '1440,900' } }
            Shoot "http://127.0.0.1:$Port/?shot=$s&readme=1" (Join-Path $png "$s.png") $size
        }
    } finally { Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue }

    $banner = 'file:///' + ((Join-Path $dir 'banner.html') -replace '\\', '/')
    Shoot $banner (Join-Path $png 'banner.png') '1280,640'

    if (-not $NoConvert) {
        python convert.py $png $images @Shots banner
        if ($LASTEXITCODE -ne 0) { throw "Image conversion failed" }
    }
} finally {
    Stop-Edge
    Pop-Location
}
