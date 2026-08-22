# Goflow App Builder Roadmap

Goal: turn a supported workflow into one same-platform executable with a focused input form, output view, local setup, and no Go toolchain on the destination machine.

| Checkpoint | Scope | Acceptance |
| :--- | :--- | :--- |
| `GF-APP-001` | Declarative app UI contract | Pack declares input fields, input envelope, output node/mode, and branding under `goflow.app.ui.v1`. |
| `GF-APP-002` | Portability analyzer | Every node is Green, Yellow, or Red; Red blocks build and Yellow explains destination dependencies. |
| `GF-APP-003` | One-file runtime | Same-platform runtime and verified Pack are emitted as one executable; tampering fails closed. |
| `GF-APP-004` | Generated run UI | Appliance renders typed inputs and cards/table/JSON output from the selected node. |
| `GF-APP-005` | Creator workflow | Editor provides Build App wizard, externalizes credential IDs, and downloads the artifact. |

Non-goals for this checkpoint: cross-compilation, code signing, native plugins, Python embedding, sub-workflow collection, and arbitrary custom frontend code.

## Portability policy

- Green: Goflow-native triggers, transforms, conditions, switches, state, delay, and embedded JS.
- Yellow: network, AI, database, SaaS, credential-backed nodes, or Python Code. Build is allowed, but the destination must provide the listed connection or runtime dependency; Python Code requires Python 3 because Python is not embedded.
- Red: native plugins, sub-workflows, SSH/Git commands, and local file/path dependencies. Build is blocked with the exact node reason.

The stopping point is a safe same-platform MVP. Cross-platform matrices and signed Windows installers require separate release engineering evidence.
