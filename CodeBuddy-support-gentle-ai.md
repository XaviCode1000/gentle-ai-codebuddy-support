---
title: CodeBuddy en gentle-ai — resumen para revisión (borrador de PR)
date: 2026-10-11
status: borrador / pendiente de revisión
scope: Fases 1 + 3 + 4 (Fase 5 = RDD, diferida)
repo: Gentleman-Programming/gentle-ai
base_commit: 25261cf (main, shallow clone)
sandbox: _sandbox/gentle-ai
---

# CodeBuddy en gentle-ai — resumen para revisión

## TL;DR

Se añade soporte de **CodeBuddy Code** a `gentle-ai` como un adapter de tier `full`, modelado sobre el adapter de **Claude Code** (CodeBuddy es, en configuración, casi un clon: `CODEBUDDY.md`, `~/.codebuddy/`, `agents/*.md`, `SKILL.md`, hooks, naming `mcp__server__tool`).

**Hecho y verificado:** instalación end-to-end (system prompt, skills, slash commands, output styles, agentes Judgment Day, hook de skill-registry), suite interna verde salvo 2 fallos ambientales preexistentes ajenos al cambio.

**Diferido a propósito:** RDD (review transport / agentes `review-*`). CodeBuddy queda **no-RDD** hasta validar su boundary de reviewer inmutable.

---

## Contexto

`gentle-ai` es un configurador (Go CLI+TUI) de agentes de IA. Cada agente se integra por 4 capas:

| Capa | Paquete | Qué aporta CodeBuddy |
|---|---|---|
| Modelo | `internal/model/types.go` | `AgentCodeBuddy = "codebuddy"` |
| Adapter | `internal/agents/codebuddy/` | rutas, detección, estrategias |
| Manifest | `internal/agents/capabilitymanifest/manifest.go` | feature claims + contratos |
| Registro/Catálogo | `agents/factory.go`, `catalog/agents.go` | alta del agente |

Y por componentes que inyectan contenido: `agentguidance` (orquestador/routing), `persona`, `permissions`, `skills`, `engram`, `mcp`, `communitytool`, `reviewassets`, `agenthooks`.

### Mapa CodeBuddy ↔ Claude Code (deltas relevantes)

| gentle-ai | Claude Code | CodeBuddy |
|---|---|---|
| Config root | `~/.claude` | **`~/.codebuddy`** |
| System prompt | `CLAUDE.md` | **`CODEBUDDY.md`** |
| MCP | `~/.claude/mcp/*.json` + `~/.claude.json` | **`~/.codebuddy/.mcp.json` (JSONC)** → `StrategyMCPConfigFile` |
| Delegación | tool `Task` | **tool `Agent`** ⚠️ |
| Hook project dir | `CLAUDE_PROJECT_DIR` | **`CODEBUDDY_PROJECT_DIR`** |
| Skills / commands / agents / output-styles | `~/.claude/*` | `~/.codebuddy/*` |
| Alias de entorno | `CLAUDE_*` | `CODEBUDDY_*` (reconoce `CLAUDE_*`) |

---

## Alcance

### Hecho (este PR)

**Fase 1 — adapter + wiring**
- Nuevo `internal/agents/codebuddy/{adapter.go,paths.go}` (clon de `claude/`): root `~/.codebuddy`, `CODEBUDDY.md`, `StrategyMarkdownSections`, `StrategyMCPConfigFile` (`~/.codebuddy/.mcp.json`), sub-agentes/commands/output-styles/skills.
- `model.AgentCodeBuddy`, alta en catálogo, fábrica (`NewAdapter` + `defaultAgentIDs`), manifest (feature claims).
- Resolver de instalación: `npm i -g @tencent-ai/codebuddy-code` (solo texto, nunca ejecutado por gentle-ai).
- Registro de skills: `.codebuddy/skills` (user + project) en `skillregistry` y en los `PROJECT_MARKERS` del plugin `skill-registry.ts`.
- CodeGraph: CodeBuddy entra en la tabla exhaustiva como `excluded` (no hay target upstream aún).
- Docs: fila en `docs/agents.md` + badge `17 → 18` en `README.md`.

**Fase 3 — agentes propios**
- `internal/assets/codebuddy/agents/{jd-fix-agent,jd-judge-a,jd-judge-b}.md` — **Judgment Day** (frontmatter `model: inherit`; sin placeholders `{{CLAUDE_MODEL}}`/`{{CLAUDE_EFFORT_FRONTMATTER}}`, que solo resuelve el adapter Claude).
- Wiring en `reviewassets.NativeAgentManifest[AgentCodeBuddy]` + `all:codebuddy` en `//go:embed`.

**Fase 4 — hooks**
- `agenthooks/skill_registry.go`: caso CodeBuddy → `~/.codebuddy/settings.json`, evento `UserPromptSubmit`, comando `gentle-ai skill-registry refresh … --cwd "${CODEBUDDY_PROJECT_DIR:-$PWD}"` (+ variante Windows PowerShell).

### Diferido (Fase 5, fuera de este PR)
- RDD: `ReviewTransportV1` / `ImmutableReviewExecutorV1`, `codebuddy/orchestrator.md` con contrato de review, agentes `review-*` (lenses + refuter), stop-hook de review, alta en `model.receiptDrivenDevelopmentAgents`.

---

## Cambios por archivo

### Nuevos
| Archivo | Contenido |
|---|---|
| `internal/agents/codebuddy/adapter.go` | Implementa `agents.Adapter` + `CapabilityManifest()` |
| `internal/agents/codebuddy/paths.go` | `ConfigPath()` → `~/.codebuddy` |
| `internal/assets/codebuddy/agents/jd-fix-agent.md` | Agente fix de Judgment Day |
| `internal/assets/codebuddy/agents/jd-judge-a.md` | Juez A |
| `internal/assets/codebuddy/agents/jd-judge-b.md` | Juez B |

### Modificados
| Archivo | Cambio |
|---|---|
| `internal/model/types.go` | `AgentCodeBuddy AgentID = "codebuddy"` |
| `internal/catalog/agents.go` | Entrada CodeBuddy (TierFull, `~/.codebuddy`) |
| `internal/agents/factory.go` | import + `case` + `defaultAgentIDs` |
| `internal/agents/capabilitymanifest/manifest.go` | `featureClaimsByAgent[AgentCodeBuddy]` (OutputStyles/SlashCommands/FileSubAgents/Skills/SystemPrompt/MCP) |
| `internal/installcmd/resolver.go` | `resolveCodeBuddyInstall` + `npmBasedAgents` |
| `internal/skillregistry/registry.go` | `.codebuddy/skills` en `UserSkillDirs` + `ProjectSkillDirs` |
| `internal/assets/opencode/plugins{,-v2}/skill-registry.ts` | `.codebuddy/skills` en `PROJECT_MARKERS` |
| `internal/components/opencoderuntimeplugins/released_digests.go` | Digests `unreleased` de los plugins modificados |
| `internal/components/communitytool/codegraph_contract.go` | `AgentCodeBuddy: excludedCompatibility` |
| `internal/components/reviewassets/install.go` | `NativeAgentManifest[AgentCodeBuddy]` = jd-* |
| `internal/components/agenthooks/skill_registry.go` | Hook de skill-registry para CodeBuddy |
| `internal/assets/assets.go` | `all:codebuddy` en `//go:embed` |
| `docs/agents.md`, `README.md` | Fila + badge |
| `internal/agents/factory_test.go`, `internal/cli/install_test.go`, `internal/components/agentguidance/{routing_test,inject_test,orchestrator_module_baseline_test}.go` + `testdata/orchestrator-module-baseline.json`, `internal/components/agenthooks/skill_registry_test.go` | Ratchets de paridad actualizados para el agente 18 |

> **Nota de mantenimiento:** los tests de conteo de agentes (`supportedAgentCount`, `NativeAgentManifest`, tablas exhaustivas) son "ratchets" deliberados: obligan a considerar cada agente nuevo. Se actualizaron de forma explícita, no silenciosa.

---

## Validación

Entorno: `_sandbox/` con **HOME aislado** (`_sandbox/home`) y CodeBuddy local (`@tencent-ai/codebuddy-code` 2.164.0). El `~/.codebuddy` y `~/.claude` reales **no se tocaron**.

- `go build ./...` → OK.
- `gentle-ai install --agent codebuddy --component persona --component skills --scope global` → **20/20 checks de verificación OK**; genera:
  - `~/.codebuddy/CODEBUDDY.md` (secciones `gentle-ai:persona`, `:orchestrator`, `:agent-routing`, `:remote-authorization`)
  - `~/.codebuddy/settings.json` (`outputStyle` + hook `UserPromptSubmit`)
  - `~/.codebuddy/output-styles/gentleman.md`
  - `~/.codebuddy/skills/**/SKILL.md` + `_shared/`
  - `~/.codebuddy/commands/*.md`
  - `~/.codebuddy/agents/{jd-fix-agent,jd-judge-a,jd-judge-b}.md` + ledger de ownership
- Segundo install → **idempotente** (0 cambios).
- `go test ./internal/...` → verde salvo 2 fallos **preexistentes y ambientales** (ver abajo).

### Fallos de test preexistentes (NO relacionados con este cambio)

| Test | Causa |
|---|---|
| `internal/cli/TestEngramPathGuidanceDefault` | El host tiene `GOBIN`/`GOPATH` apuntando al Go de `mise`, no a `$HOME/go/bin`. Reproducible sin este cambio. |
| `internal/components/reviewassets/TestOwnershipReuseRejectsLedgerWithoutMutation/hash_number` | El mensaje de error JSON de Go 1.27 cambió de formato respecto al string esperado. Reproducible sin este cambio. |

Ambos se confirmaron en un árbol sin los cambios de CodeBuddy.

---

## Cómo reproducir (sandbox)

```fish
# binarios aislados
_sandbox/bin/sb gentle-ai --version
_sandbox/bin/sb codebuddy --version

# instalación de prueba (no toca tu HOME real)
_sandbox/bin/sb gentle-ai install --agent codebuddy --component persona --component skills --scope global
```

`_sandbox/bin/sb` ejecuta cualquier comando con `HOME`/`XDG_CONFIG_HOME` apuntando a `_sandbox/home` y un `node` real (evita el shim de `mise`, que falla al cambiar `HOME`).

---

## Decisiones de diseño

1. **Clonar el adapter de Claude** en lugar de crear uno desde cero: maximiza paridad y minimiza superficie.
2. **Orquestador `generic`** (no un `codebuddy/orchestrator.md` propio): el asset genérico es tool-agnóstico ("usa el worker nativo de la plataforma"), así que evita el desajuste `Task` vs `Agent` sin duplicar contenido. Se puede revisar en Fase 5.
3. **CodeBuddy NO es RDD todavía**: sin `ReviewTransportV1` no entra en `receiptDrivenDevelopmentAgents`, así que no se envían los agentes `review-*`. Judgment Day sí se envía (no es RDD).
4. **MCP**: `StrategyMCPConfigFile` sobre `~/.codebuddy/.mcp.json` (JSONC), en lugar del esquema multi-archivo de Claude.
5. **CodeGraph `excluded`**: no existe target `codebuddy` upstream; evita escribir wiring que CodeGraph no reconocería.

---

## Riesgos y preguntas abiertas

- **Herramienta de delegación `Agent`**: el orquestador genérico no la nombra, pero cualquier asset futuro que mencione `Task` rompería la delegación en CodeBuddy.
- **`hooks.async`**: los hooks de telemetría de Claude usan `async`; no se ha confirmado que CodeBuddy lo soporte (por eso telemetría no se añadió).
- **Output styles**: se reutiliza el estilo compartido; conviene confirmar el directorio de usuario real (`~/.codebuddy/output-styles/`) contra una instalación viva.
- **Modelos**: los agentes JD usan `model: inherit`. Si se quiere asignación por fase como en Claude, haría falta implementar `ClaudeModelID` (o un resolver propio) y wire de UI.
- **Fase 5 (RDD)**: requiere validar empíricamente el boundary de reviewer inmutable antes de anunciar `ReviewTransportV1`.

---

## Borrador de PR (inglés)

**Title:** `feat(agent): add CodeBuddy Code support`

**Body:**

```markdown
## Summary

Adds CodeBuddy Code as a full-tier agent, modeled on the Claude Code adapter
(CodeBuddy is configuration-compatible: `CODEBUDDY.md`, `~/.codebuddy/`,
`agents/*.md`, `SKILL.md`, hooks, `mcp__server__tool` naming).

## What's included

- New `internal/agents/codebuddy/` adapter: root `~/.codebuddy`,
  `CODEBUDDY.md`, markdown-section prompt strategy, `StrategyMCPConfigFile`
  (`~/.codebuddy/.mcp.json`), sub-agents/commands/output-styles/skills.
- Model/catalog/factory/capability-manifest wiring.
- npm install hint (`@tencent-ai/codebuddy-code`), skill-registry dirs +
  OpenCode plugin `PROJECT_MARKERS`, CodeGraph compatibility entry (excluded).
- Judgment Day agents (`codebuddy/agents/jd-*.md`) via `NativeAgentManifest`.
- Skill-registry hook in `~/.codebuddy/settings.json` using
  `CODEBUDDY_PROJECT_DIR`.
- `docs/agents.md` row and README badge (17 → 18).

## Not included (deliberate)

Receipt-driven development (review transport, `review-*` agents). CodeBuddy
stays non-RDD until its immutable-reviewer boundary is validated.

## Testing

- `go build ./...`
- `go test ./internal/...` — green except two pre-existing, environment-only
  failures unrelated to this change:
  - `cli/TestEngramPathGuidanceDefault` (host GOBIN/GOPATH points at mise Go)
  - `reviewassets/TestOwnershipReuseRejectsLedgerWithoutMutation/hash_number`
    (Go 1.27 JSON error-message wording)
- Manual: `gentle-ai install --agent codebuddy …` writes the expected files
  (20/20 verification checks) and is idempotent on a second run.

## Notes

- Agent-count and native-agent-manifest ratchets were updated explicitly.
- Editing OpenCode plugin assets requires regenerating
  `released_digests.go`; done here for the two touched `skill-registry.ts`.
```

---

## Próximos pasos sugeridos

1. Revisar esta nota y el diff en `_sandbox/gentle-ai`.
2. (Opcional) Probar en vivo: abrir CodeBuddy con `HOME=_sandbox/home` y comprobar `/skills`, `/agents` y `codebuddy mcp list` cargando lo generado.
3. Si se aprueba, continuar con **Fase 5 (RDD)** y/o preparar la rama + PR real.
