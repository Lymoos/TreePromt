# Настройка автодеплоя PromptTree с GitHub Actions на VPS. Запускать на своём ПК:
#   powershell -NoProfile -ExecutionPolicy Bypass -File backend\deploy\setup-ci-deploy.ps1 -Server 194.87.148.14
# 1) создаёт отдельный ключ для CI (без пароля; на сервере он умеет только обновить PromptTree);
# 2) по SSH (пароль root вводите вы) разрешает этот ключ на сервере — allow-ci-deploy.sh;
# 3) по очереди кладёт в буфер обмена три секрета для GitHub.
param([Parameter(Mandatory = $true)][string]$Server)
$ErrorActionPreference = 'Stop'

$dir = Join-Path $env:USERPROFILE '.ssh'
$key = Join-Path $dir 'prompttree_ci'
New-Item -ItemType Directory -Force $dir | Out-Null
if (-not (Test-Path $key)) {
    ssh-keygen -q -t ed25519 -N '""' -C prompttree-ci -f $key
}
$pub = (Get-Content "$key.pub" -Raw).Trim()

Write-Host "`n1/3 Отпечаток ключа сервера (сравните с тем, что спросит ssh):" -ForegroundColor Cyan
$known = (ssh-keyscan -t ed25519 $Server 2>$null) -join "`n"
if (-not $known) { throw "сервер $Server не ответил по SSH (порт 22)" }
$tmp = New-TemporaryFile
Set-Content -Path $tmp -Value $known -Encoding ascii
ssh-keygen -lf $tmp
Remove-Item $tmp

Write-Host "`n2/3 Разрешаю ключ CI на сервере. Введите пароль root, когда ssh спросит." -ForegroundColor Cyan
ssh "root@$Server" "curl -fsSL https://raw.githubusercontent.com/Lymoos/TreePromt/main/backend/deploy/allow-ci-deploy.sh -o /tmp/allow-ci-deploy.sh && bash /tmp/allow-ci-deploy.sh '$pub'"
if ($LASTEXITCODE -ne 0) { throw "на сервере не получилось (см. сообщение выше)" }

Write-Host "`n3/3 Секреты GitHub: https://github.com/Lymoos/TreePromt/settings/secrets/actions -> New repository secret" -ForegroundColor Cyan
$secrets = [ordered]@{
    'VPS_HOST'        = $Server
    'VPS_KNOWN_HOSTS' = $known
    'VPS_SSH_KEY'     = (Get-Content $key -Raw).Trim()
}
foreach ($name in $secrets.Keys) {
    Set-Clipboard -Value $secrets[$name]
    Read-Host "Имя: $name — значение уже в буфере обмена. Вставьте, сохраните и нажмите Enter"
}
Set-Clipboard -Value ' '
Write-Host "`nГотово. Следующий push в main обновит сервер (вкладка Actions -> CI -> deploy)." -ForegroundColor Green
