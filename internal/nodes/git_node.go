package nodes

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitCommandExecutor struct{}

func NewGitCommandExecutor() *GitCommandExecutor {
	return &GitCommandExecutor{}
}

func (e *GitCommandExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	action, _ := node.Params["action"].(string)
	repoURL, _ := node.Params["repository_url"].(string)
	dir, _ := node.Params["target_directory"].(string)
	branch, _ := node.Params["branch"].(string)
	msg, _ := node.Params["commit_message"].(string)

	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "CLONE"
	}
	if branch == "" {
		branch = "main"
	}

	// Verify local git CLI is installed
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git CLI is not installed or not in system PATH: %w", err)
	}

	switch action {
	case "CLONE":
		if repoURL == "" || dir == "" {
			return nil, fmt.Errorf("repository_url and target_directory parameters are required for CLONE")
		}
		cmd := exec.Command("git", "clone", "-b", branch, repoURL, dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("git clone failed: %s (error: %w)", string(out), err)
		}
		return map[string]interface{}{"status": "success", "output": string(out)}, nil

	case "PULL":
		if dir == "" {
			return nil, fmt.Errorf("target_directory is required for PULL")
		}
		cmd := exec.Command("git", "-C", dir, "pull", "origin", branch)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("git pull failed: %s (error: %w)", string(out), err)
		}
		return map[string]interface{}{"status": "success", "output": string(out)}, nil

	case "COMMIT_PUSH":
		if dir == "" {
			return nil, fmt.Errorf("target_directory is required for COMMIT_PUSH")
		}
		if msg == "" {
			msg = "Update from Goflow automation flow"
		}

		// git add .
		cmdAdd := exec.Command("git", "-C", dir, "add", ".")
		if out, err := cmdAdd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git add failed: %s (error: %w)", string(out), err)
		}

		// git commit -m
		cmdCommit := exec.Command("git", "-C", dir, "commit", "-m", msg)
		outCommit, errCommit := cmdCommit.CombinedOutput()
		// Git commit returns non-zero code if there is nothing to commit, we check that
		if errCommit != nil && !strings.Contains(string(outCommit), "nothing to commit") {
			return nil, fmt.Errorf("git commit failed: %s (error: %w)", string(outCommit), errCommit)
		}

		// git push origin branch
		cmdPush := exec.Command("git", "-C", dir, "push", "origin", branch)
		outPush, errPush := cmdPush.CombinedOutput()
		if errPush != nil {
			return nil, fmt.Errorf("git push failed: %s (error: %w)", string(outPush), errPush)
		}

		return map[string]interface{}{
			"status": "success",
			"commit": string(outCommit),
			"push":   string(outPush),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported Git action: %s", action)
	}
}

func (e *GitCommandExecutor) Validate(node *Node) error {
	return nil
}

func (e *GitCommandExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeGitCommand,
		Name:        "Git Command",
		Description: "Thực hiện lệnh git clone, git pull, hoặc git commit/push tự động bằng Git CLI cục bộ",
		Icon:        "GitBranch",
		Category:    "DEVELOPER",
		Params: []ParamDefinition{
			{
				Name:        "action",
				Label:       "Git Action",
				Type:        "select",
				Default:     "CLONE",
				Options:     []string{"CLONE", "PULL", "COMMIT_PUSH"},
				Required:    true,
				Description: "Chọn hành động clone repo, pull cập nhật, hoặc commit & push",
			},
			{
				Name:        "repository_url",
				Label:       "Git Repository URL (For CLONE)",
				Type:        "text",
				Required:    false,
				Description: "Đường dẫn Git Repository cần clone (ví dụ: https://github.com/user/repo.git)",
			},
			{
				Name:        "target_directory",
				Label:       "Target Directory",
				Type:        "text",
				Required:    true,
				Description: "Thư mục đích lưu mã nguồn cục bộ (ví dụ: d:/my-projects/repo)",
			},
			{
				Name:        "branch",
				Label:       "Git Branch",
				Type:        "text",
				Default:     "main",
				Required:    true,
				Description: "Tên nhánh Git (ví dụ: main, master, develop)",
			},
			{
				Name:        "commit_message",
				Label:       "Commit Message (For COMMIT_PUSH)",
				Type:        "text",
				Required:    false,
				Description: "Nội dung thông điệp commit khi thực hiện commit/push",
			},
		},
	}
}
