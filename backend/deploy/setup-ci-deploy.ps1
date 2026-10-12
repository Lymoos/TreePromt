# Настройка автодеплоя PromptTree с GitHub Actions на VPS. Запускать на своём ПК:
#   powershell -NoProfile -ExecutionPolicy Bypass -File backend\deploy\setup-ci-deploy.ps1 -Server 194.87.148.14
# 1) создаёт отдельный ключ для CI (без пароля; на сервере он умеет только обновить PromptTree);
# 2) по SSH (пароль root вводите вы) разрешает этот ключ на сервере — allow-ci-deploy.sh;
# 3) по очереди кладёт в буфер обмена три секрета для GitHub (VPS_HOST, VPS_KNOWN_HOSTS, VPS_SSH_KEY).
param([Parameter(Mandatory = $true)][string]$Server)
# Не Stop: PowerShell 5.1 считает ошибкой любую строку нативной программы в stderr
# (ssh пишет туда предупреждения); коды возврата проверяются явно.
$ErrorActionPreference = 'Continue'

$dir = Join-Path $env:USERPROFILE '.ssh'
$key = Join-Path $dir 'prompttree_ci'
New-Item -ItemType Directory -Force $dir | Out-Null
# Каждый запуск — новый ключ: на сервере он заменит прежний (если старый где-то засветился).
Remove-Item "$key", "$key.pub" -ErrorAction SilentlyContinue
ssh-keygen -q -t ed25519 -N '""' -C prompttree-ci -f $key
$pub = (Get-Content "$key.pub" -Raw).Trim()

Write-Host "`n1/2 Разрешаю ключ CI на сервере. Введите пароль root, когда ssh спросит." -ForegroundColor Cyan
# Ключ сервера записывается в отдельный файл при этом подключении и потом уходит в секрет VPS_KNOWN_HOSTS
# (ssh-keyscan из Windows не договаривается с новым OpenSSH на сервере).
$knownFile = Join-Path $dir 'prompttree_ci_known_hosts'
Remove-Item $knownFile -ErrorAction SilentlyContinue
ssh -o HostKeyAlgorithms=ssh-ed25519 -o StrictHostKeyChecking=accept-new -o HashKnownHosts=no -o "UserKnownHostsFile=$knownFile" "root@$Server" "curl -fsSL https://raw.githubusercontent.com/Lymoos/TreePromt/main/backend/deploy/allow-ci-deploy.sh -o /tmp/allow-ci-deploy.sh && bash /tmp/allow-ci-deploy.sh '$pub'"
if ($LASTEXITCODE -ne 0) { throw "на сервере не получилось (см. сообщение выше)" }
$known = ((Get-Content $knownFile) | Where-Object { $_ -match ' ssh-ed25519 ' }) -join "`n"
if (-not $known) { throw "не удалось записать ключ сервера" }
ssh-keygen -lf $knownFile

Write-Host "`n2/2 Три секрета для GitHub. Каждый раз откроется страница нового секрета в браузере." -ForegroundColor Cyan
Write-Host "Значение уже будет в буфере обмена: вставьте его В БРАУЗЕРЕ (поле Secret), нажмите Add secret." -ForegroundColor Yellow
Write-Host "Сюда, в терминал, ничего вставлять не нужно — только нажать Enter." -ForegroundColor Yellow
$secrets = [ordered]@{
    'VPS_HOST'        = $Server
    'VPS_KNOWN_HOSTS' = $known
    'VPS_SSH_KEY'     = (Get-Content $key -Raw).Trim()
}
foreach ($name in $secrets.Keys) {
    Set-Clipboard -Value $secrets[$name]
    Start-Process 'https://github.com/Lymoos/TreePromt/settings/secrets/actions/new'
    Write-Host "`n  Name:   $name" -ForegroundColor White
    Write-Host "  Secret: (в буфере обмена — Ctrl+V в браузере)"
    [void](Read-Host "  Сохранили в GitHub? Нажмите Enter")
}
Set-Clipboard -Value ' '
Write-Host "`nГотово. Проверка: https://github.com/Lymoos/TreePromt/settings/secrets/actions — там VPS_HOST, VPS_KNOWN_HOSTS, VPS_SSH_KEY." -ForegroundColor Green
Write-Host "Следующий push в main обновит сервер (Actions -> CI -> deploy)." -ForegroundColor Green
