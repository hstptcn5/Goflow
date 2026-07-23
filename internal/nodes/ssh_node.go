package nodes

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHRunnerExecutor struct{}

func NewSSHRunnerExecutor() *SSHRunnerExecutor {
	return &SSHRunnerExecutor{}
}

func (e *SSHRunnerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// 1. Resolve configuration
	addr, _ := node.Params["address"].(string)
	username, _ := node.Params["username"].(string)
	password, _ := node.Params["password"].(string)
	privateKey, _ := node.Params["private_key"].(string)
	command, _ := node.Params["command"].(string)
	credID, _ := node.Params["credential_id"].(string)

	if credID != "" {
		ctx.mu.RLock()
		decrypted, ok := ctx.Credentials[credID]
		ctx.mu.RUnlock()
		if ok && decrypted != "" {
			// If decrypted credential contains BEGIN RSA PRIVATE KEY or similar, it's a private key.
			// Otherwise treat it as the password.
			if strings.Contains(decrypted, "PRIVATE KEY") {
				privateKey = decrypted
			} else {
				password = decrypted
			}
		}
	}

	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("SSH Host address is required")
	}
	if !strings.Contains(addr, ":") {
		addr = addr + ":22"
	}
	if username == "" {
		return nil, fmt.Errorf("SSH username is required")
	}
	if command == "" {
		return nil, fmt.Errorf("SSH command to run is empty")
	}

	// 2. Build authentication methods
	var auths []ssh.AuthMethod

	if strings.TrimSpace(privateKey) != "" {
		signer, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	} else if password != "" {
		auths = append(auths, ssh.Password(password))
	} else {
		return nil, fmt.Errorf("either SSH password or private key must be provided")
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Since Goflow is local-first
		Timeout:         10 * time.Second,
	}

	// 3. Connect to SSH Server
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH connection dial failed to %s: %w", addr, err)
	}
	defer client.Close()

	// 4. Start SSH Session
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Capture combined stdout and stderr
	out, err := session.CombinedOutput(command)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("command execution failed: %w", err)
		}
	}

	return map[string]interface{}{
		"status":    "success",
		"output":    string(out),
		"exit_code": exitCode,
	}, nil
}

func (e *SSHRunnerExecutor) Validate(node *Node) error {
	addr, _ := node.Params["address"].(string)
	if strings.TrimSpace(addr) == "" {
		return fmt.Errorf("address is required")
	}
	return nil
}

func (e *SSHRunnerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeSSHRunner,
		Name:        "SSH Runner",
		Description: "Connects to a remote host over SSH and runs shell commands",
		Icon:        "Terminal",
		Category:    "DEVELOPER",
		Params: []ParamDefinition{
			{
				Name:        "credential_id",
				Label:       "Select Encrypted Credential",
				Type:        "credential",
				Required:    false,
				Description: "Select an encrypted SSH password or private key credential from the vault",
			},
			{
				Name:        "address",
				Label:       "Host Address",
				Type:        "text",
				Default:     "127.0.0.1:22",
				Required:    true,
				Description: "Remote SSH host and port, for example 127.0.0.1:22",
			},
			{
				Name:        "username",
				Label:       "SSH Username",
				Type:        "text",
				Required:    true,
				Description: "SSH username, for example root or ubuntu",
			},
			{
				Name:        "password",
				Label:       "SSH Password (Password Auth)",
				Type:        "password",
				Required:    false,
				Description: "SSH password, only required when not using a private key or vault credential",
			},
			{
				Name:        "private_key",
				Label:       "SSH Private Key (Key Auth)",
				Type:        "textarea",
				Required:    false,
				Description: "PEM private key content, used for key-based authentication",
			},
			{
				Name:        "command",
				Label:       "Shell Command",
				Type:        "textarea",
				Default:     "uptime && df -h",
				Required:    true,
				Description: "Shell command to run on the remote server. Supports placeholders such as {{node.path}}",
			},
		},
	}
}
