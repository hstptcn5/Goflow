package nodes

import (
	"fmt"
	"net/http"
	"strings"

	"goflow/internal/adapter"
)

type NormalizedHTTPSourceExecutor struct{ source *adapter.HTTPSource }

func NewNormalizedHTTPSourceExecutor() *NormalizedHTTPSourceExecutor {
	return &NormalizedHTTPSourceExecutor{source: adapter.NewHTTPSource(nil, nil)}
}

func NewNormalizedHTTPSourceExecutorWithClient(client *http.Client, wait adapter.WaitFunc) *NormalizedHTTPSourceExecutor {
	return &NormalizedHTTPSourceExecutor{source: adapter.NewHTTPSource(client, wait)}
}

func (executor *NormalizedHTTPSourceExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	request, err := normalizedHTTPSourceRequest(node, ctx)
	if err != nil {
		return nil, err
	}
	return executor.source.Fetch(ctx.Context, request)
}

func (executor *NormalizedHTTPSourceExecutor) Validate(node *Node) error {
	request, err := normalizedHTTPSourceRequest(node, nil)
	if err != nil {
		return err
	}
	if request.AuthMode != "none" {
		request.Credential = "validation-placeholder"
	}
	return adapter.ValidateRequest(request)
}

func normalizedHTTPSourceRequest(node *Node, ctx *ExecutionContext) (adapter.Request, error) {
	stringParam := func(name, defaultValue string) string {
		value, _ := node.Params[name].(string)
		if strings.TrimSpace(value) == "" {
			return defaultValue
		}
		return strings.TrimSpace(value)
	}
	request := adapter.Request{
		URL:              nodeString(node, "url"),
		AuthMode:         stringParam("auth_mode", "none"),
		APIKeyHeader:     stringParam("api_key_header", "X-API-Key"),
		Pagination:       stringParam("pagination", "none"),
		CursorQueryParam: stringParam("cursor_query_param", "cursor"),
		ItemsField:       stringParam("items_field", "items"),
		NextCursorField:  stringParam("next_cursor_field", "next_cursor"),
	}
	var err error
	request.MaxPages, err = adapter.ParsePositiveInt(node.Params["max_pages"], 5)
	if err != nil {
		return request, fmt.Errorf("normalized source max_pages %w", err)
	}
	request.MaxItems, err = adapter.ParsePositiveInt(node.Params["max_items"], 1000)
	if err != nil {
		return request, fmt.Errorf("normalized source max_items %w", err)
	}
	request.Contract, err = adapter.ParseContract(node.Params["response_contract"])
	if err != nil {
		return request, err
	}
	credentialID := nodeString(node, "credential_id")
	if request.AuthMode != "none" {
		if credentialID == "" {
			return request, fmt.Errorf("normalized source credential_id is required for auth")
		}
		if ctx != nil {
			credential, ok := ctx.Credentials[credentialID]
			if !ok || credential == "" {
				return request, fmt.Errorf("normalized source credential is not available")
			}
			request.Credential = credential
		}
	} else if credentialID != "" {
		return request, fmt.Errorf("normalized source credential_id is allowed only with auth")
	}
	return request, nil
}

func nodeString(node *Node, name string) string {
	value, _ := node.Params[name].(string)
	return strings.TrimSpace(value)
}

func (executor *NormalizedHTTPSourceExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeNormalizedHTTPSource, Name: "Normalized HTTP Source", Icon: "DatabaseZap", Category: "ACTION", Retryable: false,
		Description: "Reads and validates a bounded normalized JSON source with optional cursor pagination",
		Params: []ParamDefinition{
			{Name: "url", Label: "Source URL", Type: "text", Required: true},
			{Name: "auth_mode", Label: "Authentication", Type: "select", Default: "none", Options: []string{"none", "bearer", "api_key"}, Required: true},
			{Name: "credential_id", Label: "Credential", Type: "credential", Required: false},
			{Name: "api_key_header", Label: "API key header", Type: "text", Default: "X-API-Key", Required: false},
			{Name: "pagination", Label: "Pagination", Type: "select", Default: "none", Options: []string{"none", "cursor"}, Required: true},
			{Name: "cursor_query_param", Label: "Cursor query parameter", Type: "text", Default: "cursor", Required: false},
			{Name: "items_field", Label: "Items field", Type: "text", Default: "items", Required: false},
			{Name: "next_cursor_field", Label: "Next cursor field", Type: "text", Default: "next_cursor", Required: false},
			{Name: "max_pages", Label: "Maximum pages", Type: "integer", Default: 5, Required: true},
			{Name: "max_items", Label: "Maximum items", Type: "integer", Default: 1000, Required: true},
			{Name: "response_contract", Label: "Normalized response contract", Type: "json", Required: false},
		},
	}
}
