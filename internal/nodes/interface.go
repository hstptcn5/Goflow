package nodes

import (
	"context"
	"sync"
)

// NodeType identifies a supported node executor type.
type NodeType string

const (
	TypeWebhookTrigger       NodeType = "webhookTrigger"
	TypeCronTrigger          NodeType = "cronTrigger"
	TypeManualTrigger        NodeType = "manualTrigger"
	TypeFileTrigger          NodeType = "fileTrigger"
	TypeHTTPRequest          NodeType = "httpRequest"
	TypeNormalizedHTTPSource NodeType = "normalizedHttpSource"
	TypeRSSFeedSource        NodeType = "rssFeedSource"
	TypeSourcePolicy         NodeType = "sourcePolicy"
	TypeTelegramBot          NodeType = "telegramBot"
	TypeJSONTransform        NodeType = "jsonTransform"
	TypeConditionIF          NodeType = "conditionIf"
	TypeSwitch               NodeType = "switch"
	TypeWorkflowState        NodeType = "workflowState"
	TypeLocalFile            NodeType = "localFile"
	TypeTableFile            NodeType = "tableFile"
	TypeEmailSMTP            NodeType = "emailSMTP"
	TypeDelaySleep           NodeType = "delaySleep"
	TypeOpenAIGPT            NodeType = "openAIGPT"
	TypeDeepSeekAI           NodeType = "deepseekAI"
	TypeDiscordBot           NodeType = "discordBot"
	TypeSlackBot             NodeType = "slackBot"
	TypeJSCodeRunner         NodeType = "jsCodeRunner"
	TypePythonCode           NodeType = "pythonCode"
	TypeSubWorkflow          NodeType = "subWorkflow"
	TypePostgresQuery        NodeType = "postgresQuery"
	TypeRedisCommand         NodeType = "redisCommand"
	TypeGoogleSheets         NodeType = "googleSheets"
	TypeMySQLQuery           NodeType = "mysqlQuery"
	TypeMongoDBCommand       NodeType = "mongodbCommand"
	TypeGoogleDrive          NodeType = "googleDrive"
	TypeGmailREST            NodeType = "gmailREST"
	TypeNotionPage           NodeType = "notionPage"
	TypeSSHRunner            NodeType = "sshRunner"
	TypeGitCommand           NodeType = "gitCommand"
	TypeGithubWebhook        NodeType = "githubWebhook"
	TypeGoflowPlugin         NodeType = "goflowPlugin"
)

// Node represents one workflow canvas node.
type Node struct {
	ID       string                 `json:"id"`
	Type     NodeType               `json:"type"`
	Name     string                 `json:"name"`
	Position map[string]float64     `json:"position,omitempty"`
	Params   map[string]interface{} `json:"params"`
}

// Edge represents a connection between two workflow nodes.
type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"sourceHandle,omitempty"`
	Target       string `json:"target"`
	TargetHandle string `json:"targetHandle,omitempty"`
}

// CredentialMetadata carries non-secret credential classification data alongside
// decrypted credential values. Executors can use this to fail closed when a
// workflow references a credential for the wrong provider.
type CredentialMetadata struct {
	Kind     string
	Provider string
	Type     string
}

// ExecutionContext carries workflow state, node outputs, and decrypted credentials.
type ExecutionContext struct {
	Context            context.Context
	WorkflowID         string
	ExecutionID        string
	Outputs            map[string]interface{}
	Credentials        map[string]string
	CredentialMetadata map[string]CredentialMetadata
	mu                 sync.RWMutex

	// ExecuteWorkflow runs a child workflow without importing the engine package here.
	ExecuteWorkflow func(workflowID string, payload interface{}) (interface{}, error)

	// RefreshCredential refreshes an expired credential when the storage layer supports it.
	RefreshCredential func(id string) (string, error)

	// State callbacks expose persistent state without giving nodes raw database access.
	StateGet       func(scope, key string) (interface{}, bool, error)
	StateSet       func(scope, key string, value interface{}) error
	StateDelete    func(scope, key string) (bool, error)
	StateIncrement func(scope, key string, delta float64) (float64, error)
}

func NewExecutionContext(workflowID, executionID string) *ExecutionContext {
	return NewExecutionContextWithContext(context.Background(), workflowID, executionID)
}

func NewExecutionContextWithContext(parent context.Context, workflowID, executionID string) *ExecutionContext {
	if parent == nil {
		parent = context.Background()
	}
	return &ExecutionContext{
		Context:            parent,
		WorkflowID:         workflowID,
		ExecutionID:        executionID,
		Outputs:            make(map[string]interface{}),
		Credentials:        make(map[string]string),
		CredentialMetadata: make(map[string]CredentialMetadata),
	}
}

func (ctx *ExecutionContext) SetOutput(nodeID string, data interface{}) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.Outputs[nodeID] = data
}

func (ctx *ExecutionContext) GetOutput(nodeID string) (interface{}, bool) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	val, ok := ctx.Outputs[nodeID]
	return val, ok
}

func (ctx *ExecutionContext) GetOutputs() map[string]interface{} {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	res := make(map[string]interface{}, len(ctx.Outputs))
	for k, v := range ctx.Outputs {
		res[k] = v
	}
	return res
}

// ParamDefinition describes one configurable UI parameter.
type ParamDefinition struct {
	Name                string              `json:"name"`
	Label               string              `json:"label"`
	Type                string              `json:"type"`
	Default             any                 `json:"default,omitempty"`
	Options             []string            `json:"options,omitempty"`
	Required            bool                `json:"required"`
	Description         string              `json:"description,omitempty"`
	CredentialKinds     []string            `json:"credential_kinds,omitempty"`
	CredentialProviders []string            `json:"credential_providers,omitempty"`
	VisibleWhen         map[string][]string `json:"visible_when,omitempty"`
	Advanced            bool                `json:"advanced,omitempty"`
	Control             string              `json:"control,omitempty"`
	Language            string              `json:"language,omitempty"`
	Placeholder         string              `json:"placeholder,omitempty"`
}

// OutputReferenceContract describes how a node's runtime value is exposed to
// downstream template expressions. It exists so UI and AI clients do not have
// to guess whether executors add an extra envelope around their returned value.
type OutputReferenceContract struct {
	RootMode      string   `json:"root_mode"`
	Pattern       string   `json:"pattern"`
	DynamicFields bool     `json:"dynamic_fields,omitempty"`
	Description   string   `json:"description,omitempty"`
	Examples      []string `json:"examples,omitempty"`
}

// TriggerContract describes how a trigger node is invoked at runtime. In
// particular, it separates editor parameters from the actual public endpoint.
type TriggerContract struct {
	Invocation       string   `json:"invocation"`
	EndpointTemplate string   `json:"endpoint_template,omitempty"`
	RequiresActive   bool     `json:"requires_active,omitempty"`
	PayloadRoot      string   `json:"payload_root,omitempty"`
	Notes            []string `json:"notes,omitempty"`
}

// NodeDefinition contains UI metadata for a node type.
type NodeDefinition struct {
	Type            NodeType                 `json:"type"`
	Name            string                   `json:"name"`
	Description     string                   `json:"description"`
	Icon            string                   `json:"icon"`
	Category        string                   `json:"category"`
	Retryable       bool                     `json:"retryable"` // False disables retry for non-idempotent side effects.
	Version         string                   `json:"version,omitempty"`
	Capabilities    []string                 `json:"capabilities,omitempty"`
	Outputs         []PluginOutputDefinition `json:"outputs,omitempty"`
	OutputReference *OutputReferenceContract `json:"output_reference,omitempty"`
	TriggerContract *TriggerContract         `json:"trigger_contract,omitempty"`
	Params          []ParamDefinition        `json:"params"`
}

// NodeExecutor is implemented by every built-in node and plugin node.
type NodeExecutor interface {
	Execute(ctx *ExecutionContext, node *Node) (interface{}, error)
	Validate(node *Node) error
	GetDefinition() NodeDefinition
}
