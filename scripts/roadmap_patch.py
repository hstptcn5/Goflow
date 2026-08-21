from pathlib import Path


def replace(path, old, new, count=1):
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    if old not in text:
        raise SystemExit(f"roadmap patch: expected text not found in {path}: {old[:120]!r}")
    p.write_text(text.replace(old, new, count), encoding="utf-8")


def append_before(path, marker, addition):
    replace(path, marker, addition + marker)


# GF-NODE-001: additive rich parameter and node-definition metadata.
replace(
    "internal/nodes/interface.go",
    '''\tCredentialProviders []string `json:"credential_providers,omitempty"`\n}''',
    '''\tCredentialProviders []string            `json:"credential_providers,omitempty"`\n\tVisibleWhen         map[string][]string `json:"visible_when,omitempty"`\n\tAdvanced            bool                `json:"advanced,omitempty"`\n\tControl             string              `json:"control,omitempty"`\n\tLanguage            string              `json:"language,omitempty"`\n\tPlaceholder         string              `json:"placeholder,omitempty"`\n}''',
)
replace(
    "internal/nodes/interface.go",
    '''\tRetryable   bool              `json:"retryable"` // False disables retry for non-idempotent side effects.\n\tParams      []ParamDefinition `json:"params"`\n}''',
    '''\tRetryable    bool                     `json:"retryable"` // False disables retry for non-idempotent side effects.\n\tVersion      string                   `json:"version,omitempty"`\n\tCapabilities []string                 `json:"capabilities,omitempty"`\n\tOutputs      []PluginOutputDefinition `json:"outputs,omitempty"`\n\tParams       []ParamDefinition        `json:"params"`\n}''',
)

# Discover first-class plugin manifests and promoted reusable code nodes at startup.
replace(
    "internal/nodes/registry.go",
    '''\t_ = registry.Register(NewGoflowPluginExecutor())\n\treturn registry''',
    '''\t_ = registry.Register(NewGoflowPluginExecutor())\n\tif discovered, _ := DiscoverPluginNodeExecutors("plugins"); len(discovered) > 0 {\n\t\tfor _, executor := range discovered {\n\t\t\t_ = registry.RegisterOrReplaceCustom(executor)\n\t\t}\n\t}\n\tif discovered, _ := DiscoverReusableCodeExecutors(DefaultReusableCodeDir()); len(discovered) > 0 {\n\t\tfor _, executor := range discovered {\n\t\t\t_ = registry.RegisterOrReplaceCustom(executor)\n\t\t}\n\t}\n\treturn registry''',
)

# Promoted code keeps the original Code Node input contract when it declares one input named "input".
replace(
    "internal/nodes/reusable_code.go",
    '''\tinput := make(map[string]interface{}, len(e.manifest.Inputs))\n\tfor _, definition := range e.manifest.Inputs {\n\t\tinput[definition.Name] = node.Params[definition.Name]\n\t}''',
    '''\tvar input interface{}\n\tif len(e.manifest.Inputs) == 1 && e.manifest.Inputs[0].Name == "input" {\n\t\tinput = node.Params["input"]\n\t} else {\n\t\tmapped := make(map[string]interface{}, len(e.manifest.Inputs))\n\t\tfor _, definition := range e.manifest.Inputs {\n\t\t\tmapped[definition.Name] = node.Params[definition.Name]\n\t\t}\n\t\tinput = mapped\n\t}''',
)

# Register the safe cURL importer, code assistant, and reusable-code promotion APIs.
replace(
    "internal/api/router.go",
    '''\taiHandler := NewAIHandler(credStore, registry)''',
    '''\taiHandler := NewAIHandler(credStore, registry)\n\thttpImportHandler := NewHTTPImportHandler()\n\tcustomNodeHandler := NewCustomNodeHandler(registry)''',
)
replace(
    "internal/api/router.go",
    '''\t\tr.Get("/nodes/definitions", nodeHandler.ListDefinitions)\n\n\t\tr.Post("/ai/generate", aiHandler.GenerateWorkflow)\n\t\tr.Post("/ai/configure-node", aiHandler.ConfigureNode)\n\t\tr.Post("/ai/review", aiHandler.ReviewWorkflow)''',
    '''\t\tr.Get("/nodes/definitions", nodeHandler.ListDefinitions)\n\t\tr.Post("/http/import-curl", httpImportHandler.ImportCURL)\n\t\tr.Post("/custom-nodes/promote", customNodeHandler.PromoteCode)\n\n\t\tr.Post("/ai/generate", aiHandler.GenerateWorkflow)\n\t\tr.Post("/ai/configure-node", aiHandler.ConfigureNode)\n\t\tr.Post("/ai/code", aiHandler.AssistCode)\n\t\tr.Post("/ai/review", aiHandler.ReviewWorkflow)''',
)

# HTTP FileRef request/response modes now that the managed file store exists.
replace(
    "internal/nodes/http_request.go",
    '''\t"time"\n\n\t"goflow/internal/apperror"''',
    '''\t"time"\n\n\t"goflow/internal/apperror"\n\t"goflow/internal/fileref"''',
)
replace(
    "internal/nodes/http_request.go",
    '''type HTTPRequestExecutor struct {\n\tclient *http.Client\n}''',
    '''type HTTPRequestExecutor struct {\n\tclient *http.Client\n\tstore  *fileref.Store\n}''',
)
replace(
    "internal/nodes/http_request.go",
    '''func NewHTTPRequestExecutor() *HTTPRequestExecutor {\n\treturn &HTTPRequestExecutor{client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: sourceprobe.SafeRedirect}}\n}''',
    '''func NewHTTPRequestExecutor() *HTTPRequestExecutor {\n\treturn &HTTPRequestExecutor{client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: sourceprobe.SafeRedirect}, store: fileref.DefaultStore()}\n}''',
)
replace(
    "internal/nodes/http_request.go",
    '''\treturn &HTTPRequestExecutor{client: &bounded}\n}''',
    '''\treturn &HTTPRequestExecutor{client: &bounded, store: fileref.DefaultStore()}\n}\n\nfunc NewHTTPRequestExecutorWithClientAndStore(client *http.Client, store *fileref.Store) *HTTPRequestExecutor {\n\texecutor := NewHTTPRequestExecutorWithClient(client)\n\tif store != nil {\n\t\texecutor.store = store\n\t}\n\treturn executor\n}''',
)
replace(
    "internal/nodes/http_request.go",
    '''\tbody, contentType, err := buildHTTPRequestBody(node, method)''',
    '''\tbody, contentType, err := buildHTTPRequestBodyWithStore(node, method, e.store)''',
)
replace(
    "internal/nodes/http_request.go",
    '''\t\tcase "text":\n\t\t\tdata = string(respBytes)\n\t\tdefault:''',
    '''\t\tcase "text":\n\t\t\tdata = string(respBytes)\n\t\tcase "file":\n\t\t\tname := strings.TrimSpace(conditionValueString(node.Params["response_file_name"]))\n\t\t\tif name == "" {\n\t\t\t\tname = "response.bin"\n\t\t\t}\n\t\t\tref, err := e.store.PutBytes(name, resp.Header.Get("Content-Type"), respBytes)\n\t\t\tif err != nil {\n\t\t\t\treturn httpPageResult{}, fmt.Errorf("store HTTP response FileRef: %w", err)\n\t\t\t}\n\t\t\tdata = ref\n\t\tdefault:''',
)
replace(
    "internal/nodes/http_request.go",
    '''{Name: "query_params", Label: "Query Parameters", Type: "json", Default: "{}", Required: false, Description: "Structured query parameters as a JSON object; array values become repeated query keys"},\n\t\t\t{Name: "headers", Label: "Headers (JSON)", Type: "json", Default: "{}", Required: false, Description: "Custom non-secret headers as a JSON object"},''',
    '''{Name: "query_params", Label: "Query Parameters", Type: "json", Default: "{}", Required: false, Control: "key-value", Description: "Structured query parameters as a JSON object; array values become repeated query keys"},\n\t\t\t{Name: "headers", Label: "Headers (JSON)", Type: "json", Default: "{}", Required: false, Control: "key-value", Advanced: true, Description: "Custom non-secret headers as a JSON object"},''',
)
replace(
    "internal/nodes/http_request.go",
    '''{Name: "auth_header", Label: "Auth Header", Type: "text", Default: "X-API-Key", Required: false, Description: "Header name for API Key or Custom Header authentication"},\n\t\t\t{Name: "auth_prefix", Label: "Auth Prefix", Type: "text", Default: "", Required: false, Description: "Optional prefix for Custom Header authentication"},\n\t\t\t{Name: "credential_header", Label: "Legacy Credential Header", Type: "text", Default: "Authorization", Required: false, Description: "Legacy header that receives the encrypted credential"},\n\t\t\t{Name: "credential_prefix", Label: "Legacy Credential Prefix", Type: "text", Default: "Bearer ", Required: false, Description: "Legacy prefix placed before the encrypted credential"},''',
    '''{Name: "auth_header", Label: "Auth Header", Type: "text", Default: "X-API-Key", Required: false, VisibleWhen: map[string][]string{"auth_mode": {"api_key", "custom_header"}}, Description: "Header name for API Key or Custom Header authentication"},\n\t\t\t{Name: "auth_prefix", Label: "Auth Prefix", Type: "text", Default: "", Required: false, VisibleWhen: map[string][]string{"auth_mode": {"custom_header"}}, Description: "Optional prefix for Custom Header authentication"},\n\t\t\t{Name: "credential_header", Label: "Legacy Credential Header", Type: "text", Default: "Authorization", Required: false, Advanced: true, VisibleWhen: map[string][]string{"auth_mode": {"legacy"}}, Description: "Legacy header that receives the encrypted credential"},\n\t\t\t{Name: "credential_prefix", Label: "Legacy Credential Prefix", Type: "text", Default: "Bearer ", Required: false, Advanced: true, VisibleWhen: map[string][]string{"auth_mode": {"legacy"}}, Description: "Legacy prefix placed before the encrypted credential"},''',
)
replace(
    "internal/nodes/http_request.go",
    '''{Name: "body_mode", Label: "Request Body Mode", Type: "select", Default: "json", Options: []string{"json", "raw", "x-www-form-urlencoded", "multipart/form-data"}, Required: false, Description: "Body encoding. File mode is added when FileRef is available."},\n\t\t\t{Name: "body", Label: "Request Body", Type: "textarea", Default: "", Required: false, Description: "JSON or raw request body"},\n\t\t\t{Name: "content_type", Label: "Raw Content Type", Type: "text", Default: "text/plain; charset=utf-8", Required: false, Description: "Content-Type used for Raw body mode"},\n\t\t\t{Name: "form_fields", Label: "Form Fields", Type: "json", Default: "{}", Required: false, Description: "Fields for urlencoded or multipart form bodies"},\n\t\t\t{Name: "response_mode", Label: "Response Mode", Type: "select", Default: "auto", Options: []string{"auto", "json", "text"}, Required: false, Description: "Auto attempts JSON then falls back to text. File mode is added with FileRef."},''',
    '''{Name: "body_mode", Label: "Request Body Mode", Type: "select", Default: "json", Options: []string{"json", "raw", "x-www-form-urlencoded", "multipart/form-data", "file"}, Required: false, Description: "Body encoding, including managed FileRef bytes."},\n\t\t\t{Name: "body", Label: "Request Body", Type: "textarea", Default: "", Required: false, VisibleWhen: map[string][]string{"body_mode": {"json", "raw"}}, Description: "JSON or raw request body"},\n\t\t\t{Name: "file_ref", Label: "Request File", Type: "json", Default: "", Required: false, Control: "file-ref", VisibleWhen: map[string][]string{"body_mode": {"file"}}, Description: "Managed FileRef used as the request body"},\n\t\t\t{Name: "content_type", Label: "Raw Content Type", Type: "text", Default: "text/plain; charset=utf-8", Required: false, VisibleWhen: map[string][]string{"body_mode": {"raw"}}, Description: "Content-Type used for Raw body mode"},\n\t\t\t{Name: "form_fields", Label: "Form Fields", Type: "json", Default: "{}", Required: false, Control: "key-value", VisibleWhen: map[string][]string{"body_mode": {"x-www-form-urlencoded", "multipart/form-data"}}, Description: "Fields for urlencoded or multipart form bodies"},\n\t\t\t{Name: "response_mode", Label: "Response Mode", Type: "select", Default: "auto", Options: []string{"auto", "json", "text", "file"}, Required: false, Description: "Auto attempts JSON then falls back to text; File stores response bytes as FileRef."},\n\t\t\t{Name: "response_file_name", Label: "Response Filename", Type: "text", Default: "response.bin", Required: false, VisibleWhen: map[string][]string{"response_mode": {"file"}}, Description: "Managed filename for File response mode"},''',
)
replace(
    "internal/nodes/http_request.go",
    '''{Name: "items_field", Label: "Items Field", Type: "text", Default: "items", Required: false, Description: "Dot path to the response array; direct array responses are also accepted"},\n\t\t\t{Name: "max_pages", Label: "Maximum Pages", Type: "integer", Default: 5, Required: false, Description: "Maximum pagination requests, up to 100"},\n\t\t\t{Name: "cursor_query_param", Label: "Cursor Query Parameter", Type: "text", Default: "cursor", Required: false},\n\t\t\t{Name: "cursor_start", Label: "Initial Cursor", Type: "text", Default: "", Required: false},\n\t\t\t{Name: "next_cursor_field", Label: "Next Cursor Field", Type: "text", Default: "next_cursor", Required: false, Description: "Dot path to the next cursor in a cursor response"},\n\t\t\t{Name: "page_query_param", Label: "Page Query Parameter", Type: "text", Default: "page", Required: false},\n\t\t\t{Name: "start_page", Label: "Start Page", Type: "integer", Default: 1, Required: false},''',
    '''{Name: "items_field", Label: "Items Field", Type: "text", Default: "items", Required: false, Advanced: true, VisibleWhen: map[string][]string{"pagination_mode": {"cursor", "page_number"}}, Description: "Dot path to the response array; direct array responses are also accepted"},\n\t\t\t{Name: "max_pages", Label: "Maximum Pages", Type: "integer", Default: 5, Required: false, Advanced: true, VisibleWhen: map[string][]string{"pagination_mode": {"cursor", "page_number"}}, Description: "Maximum pagination requests, up to 100"},\n\t\t\t{Name: "cursor_query_param", Label: "Cursor Query Parameter", Type: "text", Default: "cursor", Required: false, Advanced: true, VisibleWhen: map[string][]string{"pagination_mode": {"cursor"}}},\n\t\t\t{Name: "cursor_start", Label: "Initial Cursor", Type: "text", Default: "", Required: false, Advanced: true, VisibleWhen: map[string][]string{"pagination_mode": {"cursor"}}},\n\t\t\t{Name: "next_cursor_field", Label: "Next Cursor Field", Type: "text", Default: "next_cursor", Required: false, Advanced: true, VisibleWhen: map[string][]string{"pagination_mode": {"cursor"}}, Description: "Dot path to the next cursor in a cursor response"},\n\t\t\t{Name: "page_query_param", Label: "Page Query Parameter", Type: "text", Default: "page", Required: false, Advanced: true, VisibleWhen: map[string][]string{"pagination_mode": {"page_number"}}},\n\t\t\t{Name: "start_page", Label: "Start Page", Type: "integer", Default: 1, Required: false, Advanced: true, VisibleWhen: map[string][]string{"pagination_mode": {"page_number"}}},''',
)

replace(
    "internal/nodes/http_request_options.go",
    '''\t"strings"\n)''',
    '''\t"strings"\n\n\t"goflow/internal/fileref"\n)''',
)
replace(
    "internal/nodes/http_request_options.go",
    '''func buildHTTPRequestBody(node *Node, method string) (io.Reader, string, error) {''',
    '''func buildHTTPRequestBody(node *Node, method string) (io.Reader, string, error) {\n\treturn buildHTTPRequestBodyWithStore(node, method, fileref.DefaultStore())\n}\n\nfunc buildHTTPRequestBodyWithStore(node *Node, method string, store *fileref.Store) (io.Reader, string, error) {''',
)
replace(
    "internal/nodes/http_request_options.go",
    '''\tcase "multipart/form-data", "multipart":''',
    '''\tcase "file":\n\t\tref, err := fileref.Parse(node.Params["file_ref"])\n\t\tif err != nil {\n\t\t\treturn nil, "", fmt.Errorf("HTTP file_ref: %w", err)\n\t\t}\n\t\tdata, err := store.ReadAll(ref)\n\t\tif err != nil {\n\t\t\treturn nil, "", err\n\t\t}\n\t\tif int64(len(data)) > maxHTTPRequestBytes {\n\t\t\treturn nil, "", fmt.Errorf("HTTP FileRef request body exceeds %d byte limit", maxHTTPRequestBytes)\n\t\t}\n\t\tcontentType := strings.TrimSpace(ref.MIME)\n\t\tif contentType == "" {\n\t\t\tcontentType = "application/octet-stream"\n\t\t}\n\t\treturn bytes.NewReader(data), contentType, nil\n\tcase "multipart/form-data", "multipart":''',
)

# Code-node UX metadata.
replace(
    "internal/nodes/python_code.go",
    '''{Name: "interpreter", Label: "Interpreter Path", Type: "text", Default: "", Required: false, Description: "Optional direct python/python.exe path overriding the environment profile"},\n\t\t\t{Name: "input", Label: "Input Value", Type: "json", Default: "null", Required: false, Description: "Value exposed to code as input"},\n\t\t\t{Name: "code", Label: "Python Code", Type: "textarea", Default: "output = {\\"status\\": \\"processed\\"}", Required: true, Description: "Set the variable output to a JSON-compatible result. Trusted code runs with the Goflow OS account permissions."},\n\t\t\t{Name: "timeout", Label: "Execution Timeout (Seconds)", Type: "number", Default: 10, Required: false, Description: "Maximum runtime, between 1 and 120 seconds"},\n\t\t\t{Name: "working_directory", Label: "Working Directory", Type: "text", Default: "", Required: false, Description: "Optional working directory. Python v1 is trusted local code, not a security sandbox."},''',
    '''{Name: "interpreter", Label: "Interpreter Path", Type: "text", Default: "", Required: false, Advanced: true, Description: "Optional direct python/python.exe path overriding the environment profile"},\n\t\t\t{Name: "input", Label: "Input Value", Type: "json", Default: "null", Required: false, Description: "Value exposed to code as input"},\n\t\t\t{Name: "code", Label: "Python Code", Type: "textarea", Default: "output = {\\"status\\": \\"processed\\"}", Required: true, Control: "code", Language: "python", Description: "Set the variable output to a JSON-compatible result. Trusted code runs with the Goflow OS account permissions."},\n\t\t\t{Name: "timeout", Label: "Execution Timeout (Seconds)", Type: "number", Default: 10, Required: false, Advanced: true, Description: "Maximum runtime, between 1 and 120 seconds"},\n\t\t\t{Name: "working_directory", Label: "Working Directory", Type: "text", Default: "", Required: false, Advanced: true, Description: "Optional working directory. Python v1 is trusted local code, not a security sandbox."},''',
)
replace(
    "internal/nodes/js_code_runner.go",
    '''{Name: "code", Label: "JavaScript Code / JSON Expression", Type: "textarea",''',
    '''{Name: "code", Label: "JavaScript Code / JSON Expression", Type: "textarea", Control: "code", Language: "javascript",''',
)
replace(
    "internal/nodes/js_code_runner.go",
    '''{Name: "timeout", Label: "Execution Timeout (Seconds)", Type: "text", Default: "5", Required: false,''',
    '''{Name: "timeout", Label: "Execution Timeout (Seconds)", Type: "text", Default: "5", Required: false, Advanced: true,''',
)

# Inspector utility: visibility rules are schema-driven, not hard-coded per node.
replace(
    "ui/src/utils/inspector.js",
    '''export function classifyParams(params = []) {''',
    '''export function paramIsVisible(param, values = {}) {\n  const rules = param?.visible_when;\n  if (!rules || typeof rules !== 'object') return true;\n  return Object.entries(rules).every(([key, allowed]) => {\n    const candidates = Array.isArray(allowed) ? allowed : [allowed];\n    return candidates.map((value) => String(value)).includes(String(values?.[key] ?? ''));\n  });\n}\n\nexport function classifyParams(params = [], values = {}) {''',
)
replace(
    "ui/src/utils/inspector.js",
    '''  params.forEach((param) => {\n    const name = String(param.name || '').toLowerCase();''',
    '''  params.forEach((param) => {\n    if (!paramIsVisible(param, values)) return;\n    const name = String(param.name || '').toLowerCase();''',
)

# API client support for cURL import, AI code assistance and promotion.
append_before(
    "ui/src/services/api.js",
    '''  async reviewAIWorkflow(mode, credentialId, workflow, execution = null, focus = '') {''',
    '''  async importCurl(command) {\n    const res = await customFetch(`${API_BASE}/http/import-curl`, {\n      method: 'POST',\n      headers: { 'Content-Type': 'application/json' },\n      body: JSON.stringify({ command }),\n    });\n    if (!res.ok) throw new Error((await res.text()) || 'Không import được cURL');\n    return res.json();\n  },\n\n  async assistCode(payload) {\n    const res = await customFetch(`${API_BASE}/ai/code`, {\n      method: 'POST',\n      headers: { 'Content-Type': 'application/json' },\n      body: JSON.stringify(payload),\n    });\n    if (!res.ok) throw new Error((await res.text()) || 'AI không tạo được code');\n    return res.json();\n  },\n\n  async promoteCodeNode(manifest) {\n    const res = await customFetch(`${API_BASE}/custom-nodes/promote`, {\n      method: 'POST',\n      headers: { 'Content-Type': 'application/json' },\n      body: JSON.stringify(manifest),\n    });\n    if (!res.ok) throw new Error((await res.text()) || 'Không tạo được reusable node');\n    return res.json();\n  },\n\n''',
)

# Properties panel controls and code/cURL actions.
replace(
    "ui/src/components/PropertiesPanel.vue",
    '''import { api } from '@/services/api';''',
    '''import { api } from '@/services/api';\nimport KeyValueEditor from './KeyValueEditor.vue';''',
)
replace(
    "ui/src/components/PropertiesPanel.vue",
    '''const copiedMessage = ref('');''',
    '''const copiedMessage = ref('');\nconst curlImportText = ref('');\nconst curlImportMessage = ref('');\nconst codeActionMessage = ref('');''',
)
replace(
    "ui/src/components/PropertiesPanel.vue",
    '''const paramGroups = computed(() => classifyParams(nodeDef.value?.params || []));''',
    '''const paramGroups = computed(() => classifyParams(nodeDef.value?.params || [], props.selectedNode?.params || {}));''',
)
append_before(
    "ui/src/components/PropertiesPanel.vue",
    '''</script>''',
    '''async function importCurlIntoHTTPNode() {\n  if (props.selectedNode?.type !== 'httpRequest' || !curlImportText.value.trim()) return;\n  curlImportMessage.value = '';\n  try {\n    const imported = await api.importCurl(curlImportText.value);\n    const params = { ...(props.selectedNode.params || {}), ...(imported.params || {}) };\n    if (imported.credential_secret) {\n      const credential = await api.createCredential({\n        name: `cURL import - ${imported.credential_hint || 'HTTP credential'}`,\n        type: 'api_key',\n        data: imported.credential_secret,\n      });\n      params.credential_id = credential.id;\n      curlImportMessage.value = 'Imported cURL and moved its secret into encrypted Credentials.';\n    } else {\n      curlImportMessage.value = 'Imported cURL. No inline secret was found.';\n    }\n    emit('updateNodeParams', props.selectedNode.id, params);\n    curlImportText.value = '';\n  } catch (err) {\n    curlImportMessage.value = `Import failed: ${err.message}`;\n  }\n}\n\nasync function assistCode(param, action) {\n  if (!aiCredentialId.value) {\n    codeActionMessage.value = 'Add an OpenAI or DeepSeek credential first.';\n    return;\n  }\n  codeActionMessage.value = '';\n  try {\n    const language = param.language === 'python' || props.selectedNode?.type === 'pythonCode' ? 'python' : 'js';\n    const prompt = aiHelperPrompt.value.trim() || (action === 'fix' ? 'Fix this code while preserving its intended behavior.' : 'Generate a small transformation for the sample input.');\n    const result = await api.assistCode({\n      credential_id: aiCredentialId.value,\n      language,\n      action,\n      prompt,\n      existing_code: currentValue(param),\n      sample_input: redactValue(activeSourceData.value || null),\n    });\n    handleParamChange(param.name, result.code);\n    codeActionMessage.value = 'AI returned code to the editor. It has not been executed.';\n  } catch (err) {\n    codeActionMessage.value = `AI code assist failed: ${err.message}`;\n  }\n}\n\nasync function promoteCurrentCode(param) {\n  const runtime = props.selectedNode?.type === 'pythonCode' ? 'python' : 'js';\n  const base = String(props.selectedNode?.name || `${runtime} helper`).toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '') || 'code_helper';\n  const typeName = `user.${base}`;\n  try {\n    const result = await api.promoteCodeNode({\n      schema_version: 1,\n      type: typeName,\n      name: props.selectedNode?.name || `${runtime.toUpperCase()} helper`,\n      version: '1.0.0',\n      description: `Reusable node promoted from ${props.selectedNode?.id || 'code node'}`,\n      runtime,\n      code: currentValue(param),\n      inputs: [{ name: 'input', label: 'Input', type: 'json', required: false, default: 'null' }],\n      outputs: [{ name: 'output', type: 'json' }],\n    });\n    codeActionMessage.value = `Registered ${result.type} v${result.version}. Refresh the node picker to use it.`;\n  } catch (err) {\n    codeActionMessage.value = `Promotion failed: ${err.message}`;\n  }\n}\n\n''',
)
replace(
    "ui/src/components/PropertiesPanel.vue",
    '''        </div>\n\n        <template v-if="nodeDef?.params?.length">''',
    '''        </div>\n\n        <div v-if="selectedNode.type === 'httpRequest'" class="form-group curl-import-box">\n          <label for="curl-import">Import cURL</label>\n          <textarea id="curl-import" v-model="curlImportText" class="form-textarea code-field" rows="3" placeholder="curl https://api.example.com ..."></textarea>\n          <div class="field-actions"><button type="button" class="mini-btn" :disabled="!curlImportText.trim()" @click="importCurlIntoHTTPNode">Import safely</button></div>\n          <p v-if="curlImportMessage" class="param-desc">{{ curlImportMessage }}</p>\n        </div>\n\n        <template v-if="nodeDef?.params?.length">''',
)
# Common parameter rendering: schema controls take precedence over generic fields.
replace(
    "ui/src/components/PropertiesPanel.vue",
    '''              <select\n                v-if="param.type === 'credential'"''',
    '''              <KeyValueEditor\n                v-if="param.control === 'key-value'"\n                :value="currentValue(param)"\n                :aria-label="param.label || param.name"\n                @change="handleParamChange(param.name, $event)"\n              />\n              <div v-else-if="param.control === 'code'" class="code-control">\n                <div class="code-language">{{ param.language || 'code' }}</div>\n                <textarea\n                  :id="`param-${param.name}`"\n                  :value="currentValue(param)"\n                  class="form-textarea code-field code-editor"\n                  rows="12"\n                  :aria-label="param.label || param.name"\n                  spellcheck="false"\n                  @input="handleParamChange(param.name, $event.target.value)"\n                ></textarea>\n              </div>\n              <textarea\n                v-else-if="param.control === 'file-ref'"\n                :id="`param-${param.name}`"\n                :value="currentValue(param)"\n                class="form-textarea code-field"\n                rows="3"\n                :aria-label="param.label || param.name"\n                spellcheck="false"\n                @input="handleParamChange(param.name, $event.target.value)"\n              ></textarea>\n              <select\n                v-else-if="param.type === 'credential'"''',
)
replace(
    "ui/src/components/PropertiesPanel.vue",
    '''                <button v-if="supportsExpression(param)" type="button" class="mini-btn" @click="mappingField = param.name">Pick data</button>\n              </div>''',
    '''                <button v-if="supportsExpression(param)" type="button" class="mini-btn" @click="mappingField = param.name">Pick data</button>\n                <button v-if="param.control === 'code'" type="button" class="mini-btn" @click="assistCode(param, 'generate')">AI generate</button>\n                <button v-if="param.control === 'code'" type="button" class="mini-btn" @click="assistCode(param, 'fix')">AI fix</button>\n                <button v-if="param.control === 'code' && ['jsCodeRunner', 'pythonCode'].includes(selectedNode.type)" type="button" class="mini-btn" @click="promoteCurrentCode(param)">Promote to node</button>\n              </div>\n              <p v-if="param.control === 'code' && codeActionMessage" class="param-desc">{{ codeActionMessage }}</p>''',
    count=1,
)
# Advanced controls need select + rich controls too; previously advanced select params fell back to text.
replace(
    "ui/src/components/PropertiesPanel.vue",
    '''                <textarea\n                  v-if="param.type === 'textarea' || param.type === 'json'"''',
    '''                <KeyValueEditor\n                  v-if="param.control === 'key-value'"\n                  :value="currentValue(param)"\n                  :aria-label="param.label || param.name"\n                  @change="handleParamChange(param.name, $event)"\n                />\n                <div v-else-if="param.control === 'code'" class="code-control">\n                  <div class="code-language">{{ param.language || 'code' }}</div>\n                  <textarea :id="`param-${param.name}`" :value="currentValue(param)" class="form-textarea code-field code-editor" rows="10" @input="handleParamChange(param.name, $event.target.value)"></textarea>\n                </div>\n                <textarea\n                  v-else-if="param.control === 'file-ref'"\n                  :id="`param-${param.name}`"\n                  :value="currentValue(param)"\n                  class="form-textarea code-field"\n                  rows="3"\n                  @input="handleParamChange(param.name, $event.target.value)"\n                ></textarea>\n                <select\n                  v-else-if="param.type === 'select'"\n                  :id="`param-${param.name}`"\n                  :value="currentValue(param)"\n                  class="form-select"\n                  @change="handleParamChange(param.name, $event.target.value)"\n                >\n                  <option v-for="opt in param.options" :key="opt" :value="opt">{{ opt }}</option>\n                </select>\n                <textarea\n                  v-else-if="param.type === 'textarea' || param.type === 'json'"''',
    count=1,
)

# Dynamic Switch handles and new node icons.
replace(
    "ui/src/components/WorkflowEditor.vue",
    '''    conditionIf: 'IF',\n    emailSMTP: 'SMTP',''',
    '''    conditionIf: 'IF',\n    switch: 'Switch',\n    workflowState: 'State',\n    localFile: 'File',\n    fileTrigger: 'File trigger',\n    tableFile: 'Table',\n    pythonCode: 'Python',\n    emailSMTP: 'SMTP',''',
)
append_before(
    "ui/src/components/WorkflowEditor.vue",
    '''function getNodeStatusClass(nodeId) {''',
    '''function switchHandles(data) {\n  let cases = data?.params?.cases_json ?? [];\n  if (typeof cases === 'string') {\n    try { cases = JSON.parse(cases || '[]'); } catch { cases = []; }\n  }\n  if (!Array.isArray(cases)) cases = [];\n  const seen = new Set();\n  const handles = cases.slice(0, 16).map((item, index) => {\n    const candidate = String(item?.handle || `case_${index + 1}`).trim();\n    const id = /^[A-Za-z][A-Za-z0-9_-]{0,63}$/.test(candidate) && !['default', 'error'].includes(candidate) && !seen.has(candidate) ? candidate : `case_${index + 1}`;\n    seen.add(id);\n    return { id, label: id };\n  });\n  handles.push({ id: 'default', label: 'default' });\n  return handles;\n}\n\n''',
)
replace(
    "ui/src/components/WorkflowEditor.vue",
    '''            </template>\n            <!-- Standard Node Handles -->\n            <template v-else>\n              <Handle type="target" :position="Position.Top" />\n              <Handle type="source" :position="Position.Bottom" />\n            </template>''',
    '''            </template>\n            <template v-else-if="data.type === 'switch'">\n              <Handle type="target" :position="Position.Top" />\n              <template v-for="(branch, branchIndex) in switchHandles(data)" :key="branch.id">\n                <Handle\n                  type="source"\n                  :id="branch.id"\n                  :position="Position.Bottom"\n                  :style="{ left: `${((branchIndex + 1) / (switchHandles(data).length + 1)) * 100}%` }"\n                />\n                <span class="handle-label" :style="{ left: `${((branchIndex + 1) / (switchHandles(data).length + 1)) * 100}%` }">{{ branch.label }}</span>\n              </template>\n            </template>\n            <!-- Standard Node Handles -->\n            <template v-else>\n              <Handle type="target" :position="Position.Top" />\n              <Handle type="source" :position="Position.Bottom" />\n            </template>''',
)

print("roadmap patch applied")
