package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type GoflowPluginExecutor struct{}

func NewGoflowPluginExecutor() *GoflowPluginExecutor {
	return &GoflowPluginExecutor{}
}

func (e *GoflowPluginExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	pluginName, _ := node.Params["plugin_name"].(string)
	pluginName = strings.TrimSpace(pluginName)
	if pluginName == "" {
		return nil, fmt.Errorf("plugin_name parameter is required")
	}

	// 1. Resolve executable filepath
	// Plugins are stored in ./plugins directory in the workspace
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	pluginsDir := filepath.Join(cwd, "plugins")
	pluginPath := filepath.Join(pluginsDir, pluginName)
	if runtime.GOOS == "windows" && !strings.HasSuffix(pluginPath, ".exe") {
		pluginPath += ".exe"
	}

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugin executable not found at path: %s", pluginPath)
	}

	// 2. Prepare JSON stdin payload
	// Extract the actual outputs to feed to the plugin
	inputData := map[string]interface{}{
		"node_id":          node.ID,
		"params":           node.Params,
		"outputs":          ctx.GetOutputs(),
		"workflow_id":      ctx.WorkflowID,
		"execution_id":     ctx.ExecutionID,
	}
	inputBytes, err := json.Marshal(inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin input data: %w", err)
	}

	// 3. Execute process with 15s timeout
	cmdCtx := exec.Command(pluginPath)
	cmdCtx.Stdin = bytes.NewReader(inputBytes)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmdCtx.Stdout = &stdout
	cmdCtx.Stderr = &stderr

	// Timeout logic
	done := make(chan error, 1)
	if err := cmdCtx.Start(); err != nil {
		return nil, fmt.Errorf("failed to start plugin process: %w", err)
	}

	go func() {
		done <- cmdCtx.Wait()
	}()

	select {
	case <-time.After(15 * time.Second):
		_ = cmdCtx.Process.Kill()
		return nil, fmt.Errorf("plugin process execution timed out (15s)")
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("plugin execution failed (exit code %v, stderr: %s)", err, stderr.String())
		}
	}

	// 4. Parse JSON stdout response
	var response struct {
		Result interface{} `json:"result"`
		Error  string      `json:"error"`
	}

	stdoutBytes := stdout.Bytes()
	if len(stdoutBytes) == 0 {
		return nil, fmt.Errorf("plugin process completed but returned empty output")
	}

	// Parse fallback if the output is not a structured JSON response
	if err := json.Unmarshal(stdoutBytes, &response); err != nil {
		// If stdout is raw JSON directly without wrapping, try to unmarshal to result directly
		var rawResult interface{}
		if errRaw := json.Unmarshal(stdoutBytes, &rawResult); errRaw == nil {
			return rawResult, nil
		}
		return string(stdoutBytes), nil
	}

	if response.Error != "" {
		return nil, fmt.Errorf("plugin returned error: %s", response.Error)
	}

	return response.Result, nil
}

func (e *GoflowPluginExecutor) Validate(node *Node) error {
	pluginName, _ := node.Params["plugin_name"].(string)
	if strings.TrimSpace(pluginName) == "" {
		return fmt.Errorf("plugin_name is required")
	}
	return nil
}

func (e *GoflowPluginExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeGoflowPlugin,
		Name:        "Goflow Plugin",
		Description: "Thực thi các node tự định nghĩa viết bằng Go hoặc bất kỳ ngôn ngữ nào dưới dạng file binary tiến trình độc lập trong thư mục ./plugins/",
		Icon:        "Cpu",
		Category:    "LOGIC & UTILITY",
		Params: []ParamDefinition{
			{
				Name:        "plugin_name",
				Label:       "Plugin Executable Name",
				Type:        "text",
				Required:    true,
				Description: "Tên của file chạy plugin đặt trong thư mục ./plugins/ (ví dụ: my_custom_node hoặc my_custom_node.exe)",
			},
		},
	}
}
