---
name: roblox-luau
description: Roblox game development in Luau — project layout with Rojo, client/server split, secure RemoteEvents, DataStores, typed Luau, and checking the game in Roblox Studio through its built-in MCP server. Use for any task in a Roblox project.
when: Roblox projects (execution_env windows_user, mcp_profile roblox)
roles: [tech_lead, junior]
projects: [roblox]
version: "1"
---

# Roblox / Luau

## Project layout (Rojo)
- Code lives in the repository and is synced into Studio by Rojo (`default.project.json`). Edit files in the repo, never only inside Studio: what is not in git is lost.
- Typical tree: `src/server` → ServerScriptService, `src/client` → StarterPlayer.StarterPlayerScripts, `src/shared` → ReplicatedStorage. Shared modules are `ModuleScript`s returning a table.
- One module = one responsibility; require shared code with `require(ReplicatedStorage.Shared.Module)`.

## Luau style
- Start files with `--!strict` and type public functions and tables (`type Item = { id: string, price: number }`).
- `local` everything; cache services at the top: `local Players = game:GetService("Players")` (never `game.Players`).
- Use `task.spawn`, `task.delay`, `task.wait` — not `spawn`, `delay`, `wait`.
- Disconnect connections you no longer need; clean up on `Player.AncestryChanged`/`PlayerRemoving`.
- The checks are `stylua --check .` and `selene .`; keep both clean.

## Client and server
- **The client is untrusted.** Every `RemoteEvent`/`RemoteFunction` handler on the server validates the player, argument types, ranges and rate (cooldowns). Never accept prices, damage or positions from the client as truth.
- Game state and currency are changed only on the server. The client asks; the server decides and replicates.
- Prefer `RemoteEvent`s; a `RemoteFunction` that the server invokes on the client can hang forever — don't.

## Data
- DataStores: wrap every call in `pcall`, retry with backoff, use `UpdateAsync` for anything that can race. Save on `PlayerRemoving` and in `game:BindToClose`.
- Never store more than needed; keep a `version` field in saved tables for migrations.

## Checking your work in Studio (MCP profile `roblox`)
1. `list_roblox_studios` / `get_studio_state` — make sure the right place is open.
2. Make the change in the repo files; Rojo syncs it.
3. `start_stop_play` to run the game, `get_console_output` to read errors and your prints, `screen_capture` to look at UI.
4. For gameplay checks use `character_navigation`, `user_keyboard_input`, `user_mouse_input`; then stop play.
5. Report what you ran and what the console showed. Asset generation and marketplace tools are not available to agents.

## Do not
- Put secrets, webhooks or API keys in scripts — everything in a place can be read by exploiters.
- Use `loadstring` or `require` by asset id.
- Rely on `wait()` timing for correctness.
