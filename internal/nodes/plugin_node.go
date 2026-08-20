package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	maxPluginInputBytes  = 4 << 20
	maxPluginOutputBytes = 2 << 20
	pluginTimeout        = 15 * time.Second
)

type GoflowPluginExecutor struct{}

func NewGoflowPluginExecutor() *GoflowPluginExecutor { return &GoflowPluginExecutor{} }

func (e *GoflowPluginExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	pluginName, _ := node.Params["plugin_name"].(string)
	pluginName = strings.TrimSpace(pluginName)
	if pluginName == "" {
		return nil, fmt.Errorf("plugin_name parameter is required")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("plugin workspace could not be resolved: %w", err)
	}
	pluginsDir := filepath.Join(cwd, "plugins")
	pluginPath, err := resolvePluginPath(pluginsDir, pluginName)
	if err != nil {
		return nil, err
	}
	pluginPath, err = resolvePluginRealPath(pluginsDir, pluginPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("plugin executable is unavailable: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("plugin path must be an executable file")
	}

	inputData := map[string]interface{}{
		"node_id": node.ID, "params": node.Params, "outputs": ctx.GetOutputs(),
		"workflow_id": ctx.WorkflowID, "execution_id": ctx.ExecutionID,
	}
	inputBytes, err := json.Marshal(inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin input data: %w", err)
	}
	if len(inputBytes) > maxPluginInputBytes {
		return nil, fmt.Errorf("plugin input exceeds %d byte limit", maxPluginInputBytes)
	}

	runCtx, cancel := context.WithTimeout(ctx.Context, pluginTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, pluginPath)
	cmd.Stdin = bytes.NewReader(inputBytes)
	stdout := newBoundedBuffer(maxPluginOutputBytes)
	stderr := newBoundedBuffer(maxPluginOutputBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return nil, runCtx.Err()
		}
		return nil, fmt.Errorf("plugin execution failed: %w; stderr: %s", err, boundedNodeErrorText(stderr.Bytes()))
	}
	if stdout.Exceeded() {
		return nil, stdout.Error("plugin stdout")
	}
	if stderr.Exceeded() {
		return nil, stderr.Error("plugin stderr")
	}
	stdoutBytes := stdout.Bytes()
	if len(bytes.TrimSpace(stdoutBytes)) == 0 {
		return nil, fmt.Errorf("plugin process completed but returned empty output")
	}
	var response struct {
		Result interface{} `json:"result"`
		Error  string      `json:"error"`
	}
	if err := json.Unmarshal(stdoutBytes, &response); err != nil {
		var rawResult interface{}
		if errRaw := json.Unmarshal(stdoutBytes, &rawResult); errRaw == nil {
			return rawResult, nil
		}
		return string(stdoutBytes), nil
	}
	if strings.TrimSpace(response.Error) != "" {
		return nil, fmt.Errorf("plugin returned error: %s", boundedNodeErrorText([]byte(response.Error)))
	}
	return response.Result, nil
}

func (e *GoflowPluginExecutor) Validate(node *Node) error {
	pluginName, _ := node.Params["plugin_name"].(string)
	pluginName = strings.TrimSpace(pluginName)
	if pluginName == "" {
		return fmt.Errorf("plugin_name is required")
	}
	if len(pluginName) > 255 {
		return fmt.Errorf("plugin_name exceeds 255 character limit")
	}
	_, err := resolvePluginPath("plugins", pluginName)
	return err
}

func resolvePluginPath(pluginsDir, pluginName string) (string, error) {
	pluginName = strings.TrimSpace(pluginName)
	if pluginName == "" {
		return "", fmt.Errorf("plugin_name is required")
	}
	if pluginName != filepath.Base(pluginName) || filepath.IsAbs(pluginName) || strings.ContainsAny(pluginName, `/\`) {
		return "", fmt.Errorf("plugin_name must be a file name in the plugins directory")
	}
	pluginPath := filepath.Join(pluginsDir, pluginName)
	if runtime.GOOS == "windows" && filepath.Ext(pluginPath) == "" {
		pluginPath += ".exe"
	}
	cleanDir, err := filepath.Abs(pluginsDir)
	if err != nil {
		return "", err
	}
	cleanPath, err := filepath.Abs(pluginPath)
	if err != nil {
		return "", err
	}
	if err := ensurePathWithin(cleanDir, cleanPath); err != nil {
		return "", err
	}
	return cleanPath, nil
}

func resolvePluginRealPath(pluginsDir, pluginPath string) (string, error) {
	realDir, err := filepath.EvalSymlinks(pluginsDir)
	if err != nil {
		return "", fmt.Errorf("plugins directory could not be resolved: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(pluginPath)
	if err != nil {
		return "", fmt.Errorf("plugin executable could not be resolved: %w", err)
	}
	realDir, _ = filepath.Abs(realDir)
	realPath, _ = filepath.Abs(realPath)
	if err := ensurePathWithin(realDir, realPath); err != nil {
		return "", fmt.Errorf("plugin symlink escapes plugins directory: %w", err)
	}
	return realPath, nil
}

func ensurePathWithin(root, candidate string) error {
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path resolves outside the plugins directory")
	}
	return nil
}

func (e *GoflowPluginExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeGoflowPlugin, Name: "Goflow Plugin", Description: "Runs a bounded plugin executable contained inside the ./plugins directory using JSON IPC", Icon: "Cpu", Category: "LOGIC & UTILITY",
		Params: []ParamDefinition{{Name: "plugin_name", Label: "Plugin Executable Name", Type: "text", Required: true, Description: "Executable filename inside ./plugins. Paths and symlink escapes are rejected."}},
	}
}
