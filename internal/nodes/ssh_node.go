package nodes

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	maxSSHOutputBytes      = 1 << 20
	defaultSSHTimeout      = 30
	maxSSHTimeout          = 300
	maxSSHCommandBytes     = 256 << 10
)

type SSHRunnerExecutor struct{}

func NewSSHRunnerExecutor() *SSHRunnerExecutor { return &SSHRunnerExecutor{} }

func parseSSHTimeout(raw interface{}) (int, error) {
	if raw == nil || strings.TrimSpace(fmt.Sprint(raw)) == "" {
		return defaultSSHTimeout, nil
	}
	var seconds int
	switch typed := raw.(type) {
	case int:
		seconds = typed
	case int64:
		seconds = int(typed)
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("SSH timeout must be an integer")
		}
		seconds = int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("SSH timeout must be an integer")
		}
		seconds = parsed
	default:
		return 0, fmt.Errorf("SSH timeout must be an integer")
	}
	if seconds < 1 || seconds > maxSSHTimeout {
		return 0, fmt.Errorf("SSH timeout must be between 1 and %d seconds", maxSSHTimeout)
	}
	return seconds, nil
}

func normalizeSSHAddress(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("SSH host address is required")
	}
	if strings.ContainsAny(raw, "\r\n\x00") {
		return "", fmt.Errorf("SSH host address is invalid")
	}
	if _, _, err := net.SplitHostPort(raw); err == nil {
		return raw, nil
	}
	if strings.Count(raw, ":") == 0 {
		return net.JoinHostPort(raw, "22"), nil
	}
	return "", fmt.Errorf("SSH address must be host:port; IPv6 addresses must include brackets and a port")
}

func validateSSHNode(node *Node) error {
	if _, err := normalizeSSHAddress(conditionValueString(node.Params["address"])); err != nil {
		return err
	}
	username := strings.TrimSpace(conditionValueString(node.Params["username"]))
	if username == "" || len(username) > 256 || strings.ContainsAny(username, "\r\n\x00") {
		return fmt.Errorf("SSH username is required and must be valid")
	}
	command := conditionValueString(node.Params["command"])
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("SSH command to run is empty")
	}
	if len(command) > maxSSHCommandBytes {
		return fmt.Errorf("SSH command exceeds %d byte limit", maxSSHCommandBytes)
	}
	fingerprint := strings.TrimSpace(conditionValueString(node.Params["host_key_sha256"]))
	if !strings.HasPrefix(fingerprint, "SHA256:") || len(fingerprint) < len("SHA256:")+8 || strings.ContainsAny(fingerprint, " \t\r\n") {
		return fmt.Errorf("SSH host_key_sha256 is required in SHA256:... format")
	}
	credentialID := strings.TrimSpace(conditionValueString(node.Params["credential_id"]))
	password := conditionValueString(node.Params["password"])
	privateKey := conditionValueString(node.Params["private_key"])
	if credentialID == "" && strings.TrimSpace(password) == "" && strings.TrimSpace(privateKey) == "" {
		return fmt.Errorf("SSH requires an encrypted credential, password, or private key")
	}
	if text, ok := node.Params["timeout"].(string); ok && containsTemplateExpression(text) {
		return nil
	}
	_, err := parseSSHTimeout(node.Params["timeout"])
	return err
}

func resolveSSHAuth(ctx *ExecutionContext, node *Node) ([]ssh.AuthMethod, error) {
	password := conditionValueString(node.Params["password"])
	privateKey := conditionValueString(node.Params["private_key"])
	credentialID := strings.TrimSpace(conditionValueString(node.Params["credential_id"]))
	if credentialID != "" {
		if ctx == nil {
			return nil, fmt.Errorf("SSH credential context is unavailable")
		}
		secret, ok := ctx.Credentials[credentialID]
		if !ok || strings.TrimSpace(secret) == "" {
			return nil, fmt.Errorf("SSH credential %q is not available", credentialID)
		}
		password = ""
		privateKey = ""
		if strings.Contains(secret, "PRIVATE KEY") {
			privateKey = secret
		} else {
			password = secret
		}
	}
	if strings.TrimSpace(privateKey) != "" {
		signer, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	}
	if password != "" {
		return []ssh.AuthMethod{ssh.Password(password)}, nil
	}
	return nil, fmt.Errorf("either SSH password or private key must be provided")
}

func (e *SSHRunnerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateSSHNode(node); err != nil {
		return nil, err
	}
	address, _ := normalizeSSHAddress(conditionValueString(node.Params["address"]))
	username := strings.TrimSpace(conditionValueString(node.Params["username"]))
	command := conditionValueString(node.Params["command"])
	fingerprint := strings.TrimSpace(conditionValueString(node.Params["host_key_sha256"]))
	timeoutSeconds, err := parseSSHTimeout(node.Params["timeout"])
	if err != nil {
		return nil, err
	}
	authMethods, err := resolveSSHAuth(ctx, node)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx.Context, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	hostKeyCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		actual := ssh.FingerprintSHA256(key)
		if actual != fingerprint {
			return fmt.Errorf("SSH host key mismatch: expected %s, got %s", fingerprint, actual)
		}
		return nil
	}
	config := &ssh.ClientConfig{User: username, Auth: authMethods, HostKeyCallback: hostKeyCallback, Timeout: 10 * time.Second}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	netConn, err := dialer.DialContext(runCtx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("SSH connection dial failed: %w", err)
	}
	defer netConn.Close()
	clientConn, channels, requests, err := ssh.NewClientConn(netConn, address, config)
	if err != nil {
		return nil, fmt.Errorf("SSH handshake failed: %w", err)
	}
	client := ssh.NewClient(clientConn, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()
	output := newBoundedBuffer(maxSSHOutputBytes)
	session.Stdout = output
	session.Stderr = output
	if err := session.Start(command); err != nil {
		return nil, fmt.Errorf("SSH command could not start: %w", err)
	}
	wait := make(chan error, 1)
	go func() { wait <- session.Wait() }()
	select {
	case err := <-wait:
		if output.Exceeded() {
			return nil, output.Error("SSH command")
		}
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				return nil, fmt.Errorf("SSH command exited with code %d: %s", exitErr.ExitStatus(), boundedNodeErrorText(output.Bytes()))
			}
			return nil, fmt.Errorf("SSH command execution failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "output": output.String(), "exit_code": 0}, nil
	case <-runCtx.Done():
		_ = session.Close()
		_ = client.Close()
		return nil, runCtx.Err()
	}
}

func (e *SSHRunnerExecutor) Validate(node *Node) error { return validateSSHNode(node) }

func (e *SSHRunnerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeSSHRunner, Name: "SSH Runner", Description: "Connects to a verified remote SSH host and runs a bounded command", Icon: "Terminal", Category: "DEVELOPER",
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted SSH password or private key"},
			{Name: "address", Label: "Host Address", Type: "text", Default: "127.0.0.1:22", Required: true, Description: "Remote SSH host and port"},
			{Name: "host_key_sha256", Label: "Host Key SHA256", Type: "text", Default: "", Required: true, Description: "Expected host-key fingerprint in SHA256:... format. GoFlow will refuse mismatches."},
			{Name: "username", Label: "SSH Username", Type: "text", Required: true, Description: "SSH username"},
			{Name: "password", Label: "SSH Password (legacy)", Type: "password", Required: false, Description: "Legacy direct password. Prefer an encrypted credential."},
			{Name: "private_key", Label: "SSH Private Key (legacy)", Type: "textarea", Required: false, Description: "Legacy PEM private key. Prefer an encrypted credential."},
			{Name: "command", Label: "Shell Command", Type: "textarea", Default: "uptime && df -h", Required: true, Description: "Command to run on the verified remote host"},
			{Name: "timeout", Label: "Timeout (Seconds)", Type: "text", Default: "30", Required: false, Description: "Whole SSH operation timeout, between 1 and 300 seconds"},
		},
	}
}
