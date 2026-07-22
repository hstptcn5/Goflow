package nodes

import (
	"sync"
)

// NodeType đại diện cho mã định danh loại node
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
)

// Node biểu diễn một nút trên Canvas workflow
type Node struct {
	ID       string                 `json:"id"`
	Type     NodeType               `json:"type"`
	Name     string                 `json:"name"`
	Position map[string]float64     `json:"position,omitempty"`
	Params   map[string]interface{} `json:"params"`
}

// Edge biểu diễn đường nối giữa hai Node
type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"sourceHandle,omitempty"`
	Target       string `json:"target"`
	TargetHandle string `json:"targetHandle,omitempty"`
}

// ExecutionContext chứa dữ liệu luồng thực thi và truyền qua các node
type ExecutionContext struct {
	WorkflowID   string
	ExecutionID  string
	Outputs      map[string]interface{} // Outputs theo NodeID
	Credentials  map[string]string      // Credential ID -> decrypted secret
	mu           sync.RWMutex
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

// ParamDefinition định nghĩa tham số cấu hình trên UI
type ParamDefinition struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // 'text', 'textarea', 'select', 'json', 'credential'
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
}

// NodeDefinition chứa metadata của loại Node
type NodeDefinition struct {
	Type        NodeType          `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Category    string            `json:"category"` // 'TRIGGER', 'ACTION', 'LOGIC'
	Params      []ParamDefinition `json:"params"`
}

// NodeExecutor interface mà mọi Node Plugin phải implement
type NodeExecutor interface {
	Execute(ctx *ExecutionContext, node *Node) (interface{}, error)
	Validate(node *Node) error
	GetDefinition() NodeDefinition
}
