---
name: config-dependency-review
description: Use for dependency, build, CI, Docker, environment variable, feature flag, permission, runtime config, release packaging, and supply-chain changes. This skill reviews reproducibility, secret safety, least privilege, unsafe scripts, rollout defaults, version pinning, and deployment drift.
license: Apache-2.0
compatibility: Go modules, runtime packaging, CI pipelines, environment files, provider SDKs, sidecar services
allowed-tools: filesystem scm-api diff-analyzer corpus-selector validator
metadata:
  version: "2.0.0"
  owner: 7review
  review-domain: config-dependency
  risk-tier: high
---

# Config Dependency Review Skill

## Activation Contract

Activate when a change affects how software is built, configured, deployed, or
authorized to run. This includes lockfiles, Dockerfiles, Compose/Kubernetes,
CI/CD, `.env.example`, runtime defaults, secrets, feature flags, package manager
scripts, and provider/client version changes.

## Review Algorithm

1. Identify dependency, build, CI, environment, or runtime behavior changed by the diff.
2. Check whether defaults are safe for local, staging, and production.
3. Verify secrets and tokens are not committed or logged.
4. Look for lockfile drift, broad permissions, and unsafe scripts.
5. Require migration notes or rollout guards for behavior-changing config.
6. Verify the documented setup path matches the actual runtime contract.

## Review Areas

- New dependencies, transitive risk, version pinning, and lockfile consistency
- CI permissions, tokens, caches, artifacts, and untrusted input execution
- Environment variables, defaults, validation, and missing examples
- Feature flags and rollout behavior
- Docker, process, filesystem, and network permissions
- Build scripts that execute remote or user-controlled content

## Technical Patterns To Check

### Dependency and Supply Chain

- Direct dependency added without lockfile update or version pin.
- Lockfile changes unrelated to manifest changes.
- New transitive dependency with install scripts, native code, network access, or
  broad runtime permissions.
- Package manager script executes remote content, untrusted paths, or environment
  variables without quoting.
- Major version bump without migration note, compatibility test, or rollback path.

### Build and Docker

- Runtime image omits files required by config (`instructions.md`, skills,
  orchestrator config, certificates).
- Container runs as root without a concrete need.
- Writable mounts expose secrets or host paths unnecessarily.
- Healthchecks test liveness but not required dependencies.
- Image pulls heavy optional extras by default when a smaller production set is
  sufficient.
- Build relies on unpinned base images where reproducibility matters.

### Environment Variables

- Required env var added without validation and `.env.example` update.
- Default points at localhost inside Docker when service DNS is required.
- Secret defaults are non-empty, real-looking, or logged.
- Config loader treats required production dependencies as optional.
- Timeout, queue, worker, or retry defaults are unsafe for production.

### CI/CD and Permissions

- Workflow permissions are broader than needed.
- Pull request workflow executes untrusted code with write tokens.
- Cache keys allow dependency poisoning.
- Artifacts include secrets, env files, private diffs, or generated credentials.
- Deployment job skips tests, migration checks, or Compose validation.

### Feature Flags and Rollout

- New behavior defaults on without staged rollout.
- Flag names or semantics drift between code, docs, and env examples.
- Removing a flag before all deployed versions stop reading it.
- Rollback path requires data/config that the change deletes.

### Runtime Configuration

- Required sidecar or external service URLs must be required in production config.
- Runtime packaging must wire all variables that the config loader requires.
- Setup/configuration output must align with packaged and local run modes.
- Model provider config must avoid accidentally enabling an unintended provider.
- Tool catalogs and prompts should be shipped in the runtime image if used at
  runtime.

## Execution Rules

Config/dependency findings should prove that one environment path disagrees with
another or that a dependency change can fail at runtime. File when local setup,
runtime packaging, CI, the setup wizard, docs, and the config loader no longer
form one executable contract. Suppress if the dependency is development-only and
cannot affect production startup, tests, or operator workflows.

Choose the fix class:

- config loader validation for required or malformed values
- sample/wizard/compose alignment for operator-facing env changes
- version pin or checksum update for supply-chain stability
- readiness/health check for required services
- test fixture update when production and tests intentionally diverge

## Tool Routing

| Step | Tool Surface | Required Use |
| --- | --- | --- |
| Config discovery | `filesystem`, `corpus-selector` | Load `.env.example`, Docker/Compose files, CI workflows, orchestrator config, package manifests, lockfiles, and deployment docs. |
| Dependency comparison | `diff-analyzer`, `scm-api` | Inspect version changes, new scripts, permissions, mounted paths, health checks, network bindings, and provider defaults. |
| Gate validation | `validator` | Keep only findings with a concrete broken runtime path, unsafe default, missing required config, or reproducibility gap. |

## Escalation Signals

- A required production dependency is introduced without environment documentation, health/readiness wiring, or Docker integration.
- A dependency is unpinned, lockfiles drift, or install scripts gain network/execution privileges.
- Defaults make production run without auth, durable state, required external
  services, SCM/API credentials, or model/tool credentials.

## Evidence Standard

A config/dependency finding must include:

- changed config/dependency file
- environment or deploy target affected
- failure/security/supply-chain mode
- mismatch between docs, defaults, validation, and runtime
- exact safer default, permission, pin, or validation change

## Runtime Integration Checks

- Compare config loader behavior, setup wizard output, `.env.example`, runtime
  packaging, docs, and readiness behavior as one contract.
- Required production services must have env wiring, health checks, network
  reachability, and operator-visible failure messages.
- Dependency changes must preserve reproducible local tests and container builds; do not rely on unstated host packages or live credentials.
- New runtime knobs must be visible through config validation and, when operator-facing, through status or setup flows.

## Review Output Contract

For every config finding, identify the broken environment path: local CLI, server, runtime package, CI, sidecar bridge, SCM provider, or model provider. Include the exact variable, image, version, permission, mount, or command that must change and the verification command that should prove it.

## False Positive Checks

Do not report if:

- the change is test-only and cannot affect production packaging
- the dependency is dev-only and the production build excludes it
- broad permission is required and documented with scope constraints
- config is intentionally optional and all callers handle absence safely

## Finding Template

```text
Title: <config/dependency risk>
Surface: <Docker/CI/env/dependency/feature flag>
Evidence: <changed file and setting>
Risk: <deploy break, privilege expansion, supply-chain, drift>
Fix: <pin, validate, document, narrow permission, or change default>
Verification: <test, docker compose config, build, or CI assertion>
```

## Finding Rules

Report configuration changes that can break deploys, weaken security posture, create non-reproducible builds, or make environments diverge silently.
