from pathlib import Path


def replace(path, old, new, count=1):
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    if old not in text:
        raise SystemExit(f"roadmap patch phase 2: expected text not found in {path}: {old[:140]!r}")
    p.write_text(text.replace(old, new, count), encoding="utf-8")


# GF-MCP-001: make the already-supported idempotency control discoverable in
# dynamic workflow tool schemas, without broadening workflow exposure.
replace(
    "internal/mcpserver/server.go",
    '''\t\t\tInputSchema:  schemaOrEmptyObject(workflow.InputSchemaJSON),''',
    '''\t\t\tInputSchema:  dynamicWorkflowInputSchema(workflow.InputSchemaJSON),''',
)

# GF-PACK-001: old packs remain bounded by default. Trusted external execution
# must be explicit in pack.json and capability-gated.
replace(
    "internal/pack/pack.go",
    '''\tRequiredCapabilities   []string                `json:"required_capabilities,omitempty"`\n\tOfflineTestFixture     string                  `json:"offline_test_fixture,omitempty"`''',
    '''\tRequiredCapabilities   []string                `json:"required_capabilities,omitempty"`\n\tExecutionTier          string                  `json:"execution_tier,omitempty"`\n\tOfflineTestFixture     string                  `json:"offline_test_fixture,omitempty"`''',
)
replace(
    "internal/pack/pack.go",
    '''\tif err := rejectPackEmbeddedSecrets(workflowDef.NodesJSON); err != nil {\n\t\treturn nil, err\n\t}\n\n\treturn &Pack{''',
    '''\tif err := rejectPackEmbeddedSecrets(workflowDef.NodesJSON); err != nil {\n\t\treturn nil, err\n\t}\n\tif err := validatePackExecutionTier(manifest, workflowDef.NodesJSON); err != nil {\n\t\treturn nil, err\n\t}\n\n\treturn &Pack{''',
)

print("roadmap phase 2 patch applied")
