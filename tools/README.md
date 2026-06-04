# tools/ — ops tooling (Engine 밖)

`tools/` is the **env-coupled ops tooling** area: Go utilities whose behaviour
or numbers depend on the deployment environment (machine, gnark version,
CUDA, instance catalog, prices). This is the `Scope Boundary` "Engine 밖"
line, expressed in the directory layout.

It is deliberately distinct from:

| dir | role | env-coupled? |
|---|---|---|
| `core/`, `pkg/` | **engine** — frozen contracts, services | no |
| `cmd/` | **engine** CLI shims (keygen/prover/verifier/witness/userproof/attest/audit) | no |
| **`tools/`** | **ops** utilities (capacity planning, calibration) | **yes** |
| `scripts/ec2/` | ops **shell** (provision/sync/smoke) | yes |

`tools/` is the Go-side counterpart to `scripts/ec2/`.

## Contents

- **`plan/`** — capacity planner. `go run ./tools/plan -model … -users … -capacity …`
  → predicts Setup/prove peak RAM, artifact sizes, Setup/prove time, GPU
  benefit, and recommends EC2 instance types. Builds on the engine's exact
  constraint estimate (`pkg/estimate`, density-free); the resource
  coefficients are environment-specific and live in `tools/plan/calibration.go`
  (dated, refreshable). Density only affects prove RAM (`docs/BENCHMARK.md §1.3`).
- **`sweep/`** — the calibration sweep that produces those coefficients on a
  GPU box. See `sweep/README.md`.

## Non-goals

Instance **bootstrap automation** (planner → auto-select + launch an EC2
type) is intentionally NOT here — it is implemented externally. `tools/plan`
emits the recommendation; consuming it to provision is the operator's /
external orchestration's job.
