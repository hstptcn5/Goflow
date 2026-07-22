package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPRequestExecutor struct {
	client *http.Client
}

func NewHTTPRequestExecutor() *HTTPRequestExecutor {
	return &HTTPRequestExecutor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *HTTPRequestExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	urlStr, _ := node.Params["url"].(string)
	method, _ := node.Params["method"].(string)
	if method == "" {
		method = "GET"
	}

	bodyStr, _ := node.Params["body"].(string)
	headersMapStr, _ := node.Params["headers"].(string)

	var reqBody io.Reader
	if bodyStr != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		reqBody = bytes.NewBufferString(bodyStr)
	}

	req, err := http.NewRequest(method, urlStr, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Default Content-Type nếu có body
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Parse Custom Headers từ JSON string nếu có
	if headersMapStr != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersMapStr), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var jsonResult interface{}
	if err := json.Unmarshal(respBytes, &jsonResult); err != nil {
		// Trả về dạng string nếu không phải JSON
		jsonResult = string(respBytes)
	}

	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"data":        jsonResult,
	}, nil
}

func (e *HTTPRequestExecutor) Validate(node *Node) error {
	urlStr, _ := node.Params["url"].(string)
	if strings.TrimSpace(urlStr) == "" {
		return fmt.Errorf("HTTP Node requires a 'url' parameter")
	}
	return nil
}

func (e *HTTPRequestExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeHTTPRequest,
		Name:        "HTTP Request",
		Description: "Gửi REST API HTTP request (GET, POST, PUT, DELETE)",
		Icon:        "Globe",
		Category:    "ACTION",
		Params: []ParamDefinition{
			{
				Name:        "method",
				Label:       "HTTP Method",
				Type:        "select",
				Default:     "GET",
				Options:     []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
				Required:    true,
				Description: "Phương thức HTTP",
			},
			{
				Name:        "url",
				Label:       "Target URL",
				Type:        "text",
				Default:     "https://api.github.com",
				Required:    true,
				Description: "Địa chỉ URL nhận request",
			},
			{
				Name:        "headers",
				Label:       "Headers (JSON)",
				Type:        "json",
				Default:     "{}",
				Required:    false,
				Description: "Custom Headers dạng JSON Object",
			},
			{
				Name:        "body",
				Label:       "Request Body",
				Type:        "textarea",
				Default:     "",
				Required:    false,
				Description: "Nội dung Body (cho POST/PUT)",
			},
		},
	}
}
