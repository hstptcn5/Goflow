# Goflow App Builder Progress

Branch implementation status for `GF-APP-001` through `GF-APP-005`.

| Checkpoint | Status | Evidence |
| :--- | :--- | :--- |
| `GF-APP-001` | Implemented, pending CI | `RunUI`, `RunField`, `Branding`, capability and manifest validation. |
| `GF-APP-002` | Implemented, pending CI | Node-level Green/Yellow/Red analyzer plus unit coverage. |
| `GF-APP-003` | Implemented, pending CI | Appended ZIP executable, inventory hash verification, bounded extraction, startup detection. |
| `GF-APP-004` | Implemented, pending CI | Typed form coercion, webhook/direct input modes, selected-node output, cards/table/JSON views. |
| `GF-APP-005` | Implemented, pending CI | Editor wizard, schema-derived fields, portability report, credential externalization, download endpoint. |

## Verification gate

- Frontend utility tests cover schema-to-form conversion, typed input envelopes, and output view selection.
- Backend tests cover portability classes, credential externalization, app build/extract integrity, and manifest failure cases.
- Before merging: Go formatting/tests, frontend tests/build, and GitHub Actions must pass.
- Clean Windows acceptance remains required before calling the generated `.exe` production-ready or signed.
