package nodes

import "sync"

// NodeType identifies a supported node executor type.
type NodeType string

const (
	TypeWebhookTrigger NodeType = "webhookTrigger"
	TypeCronTrigger    NodeType = "cronTrigger"
	TypeManualTrigger  NodeType = "manualTrigger"
	TypeHTTPRequest    NodeType = "httpRequest"
	TypeTelegramBot    NodeType = "telegramBot"
	TypeJSONTransform  NodeType = "jsonTransform"
	TypeConditionIF    NodeType = "conditionIf"
	TypeEmailSMTP      NodeType = "emailSMTP"
	TypeDelaySleep     NodeType = "delaySleep"
	TypeOpenAIGPT      NodeType = "openAIGPT"
	TypeDeepSeekAI     NodeType = "deepseekAI"
	TypeDiscordBot     NodeType = "discordBot"
	TypeSlackBot       NodeType = "slackBot"
	TypeJSCodeRunner   NodeType = "jsCodeRunner"
	TypeSubWorkflow    NodeType = "subWorkflow"
	TypePostgresQuery  NodeType = "postgresQuery"
	TypeRedisCommand   NodeType = "redisCommand"
	TypeGoogleSheets   NodeType = "googleSheets"
	TypeMySQLQuery     NodeType = "mysqlQuery"
	TypeMongoDBCommand NodeType = "mongodbCommand"
	TypeGoogleDrive    NodeType = "googleDrive"
	TypeGmailREST      NodeType = "gmailREST"
	TypeNotionPage     NodeType = "notionPage"
	TypeSSHRunner      NodeType = "sshRunner"
	TypeGitCommand     NodeType = "gitCommand"
	TypeGithubWebhook  NodeType = "githubWebhook"
	TypeGoflowPlugin   NodeType = "goflowPlugin"
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

// ExecutionContext carries workflow state, node outputs, and decrypted credentials.
type ExecutionContext struct {
	WorkflowID  string
	ExecutionID string
	Outputs     map[string]interface{}
	Credentials map[string]string
	mu          sync.RWMutex

	// ExecuteWorkflow runs a child workflow without importing the engine package here.
	ExecuteWorkflow func(workflowID string, payload interface{}) (interface{}, error)

	// RefreshCredential refreshes an expired credential when the storage layer supports it.
	RefreshCredential func(id string) (string, error)
}

func NewExecutionContext(workflowID, executionID string) *ExecutionContext {
	return &ExecutionContext{
		WorkflowID:  workflowID,
		ExecutionID: executionID,
		Outputs:     make(map[string]interface{}),
		Credentials: make(map[string]string),
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
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
}

// NodeDefinition contains UI metadata for a node type.
type NodeDefinition struct {
	Type        NodeType          `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Category    string            `json:"category"`
	Retryable   bool              `json:"retryable"` // False disables retry for non-idempotent side effects.
	Params      []ParamDefinition `json:"params"`
}

// NodeExecutor is implemented by every built-in node and plugin node.
type NodeExecutor interface {
	Execute(ctx *ExecutionContext, node *Node) (interface{}, error)
	Validate(node *Node) error
	GetDefinition() NodeDefinition
}
