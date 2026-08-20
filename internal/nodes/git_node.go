package nodes

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const maxGitOutputBytes = 1 << 20

var safeGitRefPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

type GitCommandExecutor struct{}

func NewGitCommandExecutor() *GitCommandExecutor { return &GitCommandExecutor{} }

func validateGitRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("Git branch is required")
	}
	if len(ref) > 255 || strings.HasPrefix(ref, "-") || strings.HasPrefix(ref, "/") || strings.HasSuffix(ref, "/") || strings.Contains(ref, "..") || strings.Contains(ref, "@{") || !safeGitRefPattern.MatchString(ref) {
		return fmt.Errorf("Git branch %q is invalid", ref)
	}
	return nil
}

func validateGitRepositoryURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("repository_url is required")
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "ext::") || strings.HasPrefix(lower, "file://") || strings.HasPrefix(lower, "git::") || strings.HasPrefix(raw, "-") {
		return fmt.Errorf("Git repository_url uses a disallowed transport")
	}
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "ssh://") {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("Git repository_url is invalid")
		}
		if parsed.User != nil {
			if _, hasPassword := parsed.User.Password(); hasPassword {
				return fmt.Errorf("Git repository_url must not embed a password or token")
			}
		}
		return nil
	}
	// SCP-style SSH URL: git@example.com:owner/repo.git
	at := strings.Index(raw, "@")
	colon := strings.Index(raw, ":")
	if at > 0 && colon > at+1 && !strings.ContainsAny(raw, " \t\r\n") {
		return nil
	}
	return fmt.Errorf("Git repository_url must use http(s), ssh, or SCP-style SSH syntax")
}

func validateGitNode(node *Node) error {
	action, _ := node.Params["action"].(string)
	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "CLONE"
	}
	if action != "CLONE" && action != "PULL" && action != "COMMIT_PUSH" {
		return fmt.Errorf("unsupported Git action: %s", action)
	}
	branch, _ := node.Params["branch"].(string)
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	if err := validateGitRef(branch); err != nil {
		return err
	}
	dir, _ := node.Params["target_directory"].(string)
	if strings.TrimSpace(dir) == "" || len(dir) > 4096 || strings.ContainsRune(dir, '\x00') {
		return fmt.Errorf("target_directory is required and must be valid")
	}
	if action == "CLONE" {
		repoURL, _ := node.Params["repository_url"].(string)
		if containsTemplateExpression(repoURL) {
			return nil
		}
		if err := validateGitRepositoryURL(repoURL); err != nil {
			return err
		}
	}
	message, _ := node.Params["commit_message"].(string)
	if len(message) > 4096 || strings.ContainsRune(message, '\x00') {
		return fmt.Errorf("Git commit message is invalid or too long")
	}
	return nil
}

func runGitCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output := newBoundedBuffer(maxGitOutputBytes)
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	if output.Exceeded() {
		return output.String(), output.Error("Git")
	}
	if err != nil {
		return output.String(), fmt.Errorf("%w: %s", err, boundedNodeErrorText(output.Bytes()))
	}
	return output.String(), nil
}

func gitHasStagedChanges(ctx context.Context, dir string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "diff", "--cached", "--quiet", "--exit-code")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("git diff --cached failed: %w", err)
}

func (e *GitCommandExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateGitNode(node); err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git CLI is not installed or not in system PATH: %w", err)
	}
	action, _ := node.Params["action"].(string)
	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "CLONE"
	}
	repoURL, _ := node.Params["repository_url"].(string)
	dir, _ := node.Params["target_directory"].(string)
	branch, _ := node.Params["branch"].(string)
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	message, _ := node.Params["commit_message"].(string)
	if strings.TrimSpace(message) == "" {
		message = "Update from Goflow automation"
	}

	switch action {
	case "CLONE":
		out, err := runGitCommand(ctx.Context, "clone", "--branch", branch, "--", repoURL, dir)
		if err != nil {
			return nil, fmt.Errorf("git clone failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "output": out}, nil
	case "PULL":
		out, err := runGitCommand(ctx.Context, "-C", dir, "pull", "--ff-only", "origin", branch)
		if err != nil {
			return nil, fmt.Errorf("git pull failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "output": out}, nil
	case "COMMIT_PUSH":
		if _, err := runGitCommand(ctx.Context, "-C", dir, "add", "--all"); err != nil {
			return nil, fmt.Errorf("git add failed: %w", err)
		}
		hasChanges, err := gitHasStagedChanges(ctx.Context, dir)
		if err != nil {
			return nil, err
		}
		commitOutput := "no staged changes"
		if hasChanges {
			commitOutput, err = runGitCommand(ctx.Context, "-C", dir, "commit", "-m", message)
			if err != nil {
				return nil, fmt.Errorf("git commit failed: %w", err)
			}
		}
		pushOutput, err := runGitCommand(ctx.Context, "-C", dir, "push", "origin", branch)
		if err != nil {
			return nil, fmt.Errorf("git push failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "commit": commitOutput, "push": pushOutput}, nil
	default:
		return nil, fmt.Errorf("unsupported Git action: %s", action)
	}
}

func (e *GitCommandExecutor) Validate(node *Node) error { return validateGitNode(node) }

func (e *GitCommandExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeGitCommand, Name: "Git Command", Description: "Runs bounded git clone, fast-forward pull, or commit and push operations", Icon: "GitBranch", Category: "DEVELOPER",
		Params: []ParamDefinition{
			{Name: "action", Label: "Git Action", Type: "select", Default: "CLONE", Options: []string{"CLONE", "PULL", "COMMIT_PUSH"}, Required: true, Description: "Choose clone, fast-forward pull, or commit and push"},
			{Name: "repository_url", Label: "Git Repository URL (For CLONE)", Type: "text", Required: false, Description: "HTTP(S), SSH, or SCP-style repository URL. Embedded passwords/tokens are rejected."},
			{Name: "target_directory", Label: "Target Directory", Type: "text", Required: true, Description: "Local target directory"},
			{Name: "branch", Label: "Git Branch", Type: "text", Default: "main", Required: true, Description: "Validated Git branch/ref name"},
			{Name: "commit_message", Label: "Commit Message (For COMMIT_PUSH)", Type: "text", Required: false, Description: "Commit message used for commit and push"},
		},
	}
}
