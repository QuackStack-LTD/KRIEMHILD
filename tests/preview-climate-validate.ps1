param([int]$WaitForProcess = 0, [switch]$ResumeAfterPreview)
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $taskRoot
$evidence = Join-Path $taskRoot 'docs/preview-climate-investigation'
New-Item -ItemType Directory -Force (Join-Path $evidence 'regression') | Out-Null
function Run-Check([string]$Name, [string]$File, [string[]]$Arguments) {
  Write-Output "Starting $Name"
  $priorErrorPreference = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  & $File @Arguments 2>&1 | Out-File -LiteralPath (Join-Path $evidence "regression/$Name.txt") -Encoding utf8
  $checkExitCode = $LASTEXITCODE
  $ErrorActionPreference = $priorErrorPreference
  if ($checkExitCode -ne 0) { throw "$Name failed ($checkExitCode); see regression/$Name.txt" }
  Write-Output "Passed $Name"
}
if ($WaitForProcess -gt 0) {
  while (Get-Process -Id $WaitForProcess -ErrorAction SilentlyContinue) { Start-Sleep -Seconds 1 }
}
if (-not $ResumeAfterPreview) {
Run-Check 'format' 'npx.cmd' @('prettier','--write','src/continuous/Studio.jsx','src/continuous/preview-quality.js','tests/preview-quality-ui.js','tests/preview-quality-run.mjs','tests/preview-quality-cold.mjs','tests/preview-quality-integration.mjs','tests/preview-quality-memory.mjs','tests/preview-quality-benchmark.js','tests/preview-reference-build.mjs','tests/deepzoom-climate-ui.mjs','tests/deepzoom-cold-smoke.mjs','tests/viewport-memory.mjs')
Run-Check 'build' 'npm.cmd' @('run','build')
Run-Check 'reference-build' 'node.exe' @('tests/preview-reference-build.mjs')
Run-Check 'backend-build' '.tools/go/bin/go.exe' @('build','-o','.cache/kriemhild-preview-climate.exe','./cmd/kriemhild')
# Replace only the owned disposable test process, never the user's port 8124.
$testPid = (Get-NetTCPConnection -LocalPort 8127 -State Listen -ErrorAction SilentlyContinue).OwningProcess
if ($testPid) {
  $owned = Get-CimInstance Win32_Process -Filter "ProcessId = $testPid"
  if ($owned.CommandLine -notlike '*-addr 127.0.0.1:8127*' -or $owned.CommandLine -notlike '*quality-climate-20261022*') { throw 'Port 8127 is not the disposable fixture server' }
  Stop-Process -Id $testPid
}
if (Get-NetTCPConnection -LocalPort 8128 -State Listen -ErrorAction SilentlyContinue) { throw 'Reference port 8128 is occupied' }
$previewServer = Start-Process -FilePath .cache/kriemhild-preview-climate.exe -ArgumentList @('-addr','127.0.0.1:8127','-dist','dist','-data','.cache/quality-climate-20261022') -WindowStyle Hidden -RedirectStandardOutput .cache/quality-final-server-out.log -RedirectStandardError .cache/quality-final-server-error.log -PassThru
$referenceServer = Start-Process -FilePath .cache/kriemhild-preview-climate.exe -ArgumentList @('-addr','127.0.0.1:8128','-dist','.cache/preview-reference-dist','-data','.cache/preview-cold-data') -WindowStyle Hidden -RedirectStandardOutput .cache/quality-reference-out.log -RedirectStandardError .cache/quality-reference-error.log -PassThru
$env:KRIEMHILD_BROWSER_EXECUTABLE = 'C:/Users/tomyi/AppData/Local/ms-playwright/chromium-1243/chrome-win64/chrome.exe'
try {
  Run-Check 'memory' 'node.exe' @('tests/preview-quality-memory.mjs')
} finally { Stop-Process -Id $referenceServer.Id -ErrorAction SilentlyContinue }
Run-Check 'quality-integration' 'node.exe' @('tests/preview-quality-integration.mjs')
} else {
  $env:KRIEMHILD_BROWSER_EXECUTABLE = 'C:/Users/tomyi/AppData/Local/ms-playwright/chromium-1243/chrome-win64/chrome.exe'
  $testPid = (Get-NetTCPConnection -LocalPort 8127 -State Listen).OwningProcess
  $owned = Get-CimInstance Win32_Process -Filter "ProcessId = $testPid"
  if ($owned.CommandLine -notlike '*quality-climate-20261022*') { throw 'Port 8127 is not the disposable fixture server' }
  $previewServer = Get-Process -Id $testPid
}
$env:KRIEMHILD_AUDIT_URL='http://127.0.0.1:8127'
$env:KRIEMHILD_AUDIT_WORLD='977d134ee5aadc480e1410ade3c84587'
$env:KRIEMHILD_AUDIT_RENDERER='viewport'
$env:KRIEMHILD_AUDIT_OUT='docs/preview-climate-investigation/builders'
Run-Check 'builder-integration' 'node.exe' @('tests/run-continuous-investigation.mjs')
$env:KRIEMHILD_AUDIT_SITE='1.3,0.22'
Run-Check 'settlement-integration' 'node.exe' @('tests/run-continuous-settlement-investigation.mjs')
Run-Check 'javascript-tests' 'npm.cmd' @('test')
Run-Check 'go-tests' '.tools/go/bin/go.exe' @('test','./internal/terrain/...','./internal/httpapi')
Run-Check 'vet' '.tools/go/bin/go.exe' @('vet','./internal/terrain/...','./internal/httpapi')
Run-Check 'climate-audit-build' '.tools/go/bin/go.exe' @('build','-o','.cache/climate-quality-audit.exe','./cmd/climate-audit')
$pairedSeeds='Kriemhild,gallery-001,gallery-002,8fef5907901a,gallery-003,gallery-004,d1feae98-6510-4a72-951b-44e93a01ae86'
Run-Check 'paired-v2' '.cache/climate-quality-audit.exe' @('-climate-version','2','-seeds',$pairedSeeds,'-out','docs/preview-climate-investigation/paired-v2')
Run-Check 'paired-v3' '.cache/climate-quality-audit.exe' @('-climate-version','3','-seeds',$pairedSeeds,'-out','docs/preview-climate-investigation/paired-v3')
$env:CLIMATE_UI_VERSION='3'
$env:CLIMATE_UI_OUT='docs/preview-climate-investigation/climate-ui'
Run-Check 'climate-ui' 'node.exe' @('tests/deepzoom-climate-ui.mjs')
Stop-Process -Id $previewServer.Id
$previewServer = Start-Process -FilePath .cache/kriemhild-preview-climate.exe -ArgumentList @('-addr','127.0.0.1:8127','-dist','dist','-data','.cache/quality-climate-20261022') -WindowStyle Hidden -RedirectStandardOutput .cache/quality-cold-server-out.log -RedirectStandardError .cache/quality-cold-server-error.log -PassThru
$env:CLIMATE_UI_FIXTURE='docs/preview-climate-investigation/climate-ui/results.json'
$env:CLIMATE_UI_COLD_OUT='docs/preview-climate-investigation/climate-ui/cold-restart.json'
Run-Check 'climate-cold-restart' 'node.exe' @('tests/deepzoom-cold-smoke.mjs')
Run-Check 'summary' 'python.exe' @('tests/preview-climate-summary.py')
Write-Output "Validation complete. Disposable preview server PID $($previewServer.Id), port 8127."
