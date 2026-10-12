#Requires -RunAsAdministrator
# Окружение windows_user для AiCrew (docs/stage7-5-manager-skills-mcp.md, раздел 5).
# Запускает ВЛАДЕЛЕЦ ПК в PowerShell «от имени администратора». Пароль вводится здесь и никуда не пишется.
#   - пользователь ai_worker: обычный (группа «Пользователи»), без прав администратора;
#   - рабочая папка D:\aicrew-work: права только у владельца, ai_worker, SYSTEM и администраторов.
#     К вашему профилю (C:\Users\<вы>) у ai_worker доступа нет — это права Windows по умолчанию.
param(
    [string]$User = 'ai_worker',
    [string]$WorkRoot = 'D:\aicrew-work'
)
$ErrorActionPreference = 'Stop'
$owner = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name

if (Get-LocalUser -Name $User -ErrorAction SilentlyContinue) {
    Write-Host "Пользователь $User уже есть — не трогаю."
} else {
    $pw = Read-Host "Придумайте пароль для $User (понадобится ещё раз для aicrew-host set-worker)" -AsSecureString
    New-LocalUser -Name $User -Password $pw -FullName 'AiCrew worker' `
        -Description 'AiCrew: задачи windows_user, без прав администратора' -PasswordNeverExpires | Out-Null
    $users = (New-Object System.Security.Principal.SecurityIdentifier 'S-1-5-32-545').Translate([System.Security.Principal.NTAccount]).Value
    Add-LocalGroupMember -Group $users.Split('\')[-1] -Member $User
    Write-Host "Создан пользователь $User (группа «Пользователи»)."
}

New-Item -ItemType Directory -Force $WorkRoot, "$WorkRoot\.runs", "$WorkRoot\repos" | Out-Null
# Наследование прав с диска отключается: доступ только у перечисленных.
icacls $WorkRoot /inheritance:r /grant:r "${owner}:(OI)(CI)F" "${User}:(OI)(CI)M" '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' | Out-Null
Write-Host "Рабочая папка $WorkRoot: полный доступ — $owner, изменение — $User."

Write-Host @"

Дальше (один раз):
 1. Войдите в Windows как $User (или: runas /user:$User powershell) и установите Claude Code:
      irm https://claude.ai/install.ps1 | iex
 2. Там же для Roblox: установите Roblox Studio, войдите в свой аккаунт Roblox, включите
      Assistant → Manage MCP Servers → Enable Studio as MCP server;
    и инструменты проверок (rokit, selene, stylua): https://github.com/rojo-rbx/rokit
 3. В своём обычном терминале сохраните пароль для службы (шифруется DPAPI):
      aicrew-host set-worker -roblox
 4. Репозитории Roblox держите в $WorkRoot\repos и привязывайте с профилем {"template":"roblox"}.
"@
