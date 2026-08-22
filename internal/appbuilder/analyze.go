package appbuilder

import (
	"encoding/json"
	"fmt"
	"strings"

	"goflow/internal/nodes"
	"goflow/internal/pack"
)

type Level string

const (
	Green  Level = "green"
	Yellow Level = "yellow"
	Red    Level = "red"
)

type NodeResult struct {
	NodeID string `json:"node_id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Level  Level  `json:"level"`
	Reason string `json:"reason"`
}

// ExternalizeCredentials removes machine-local credential IDs and replaces
// them with first-run Pack bindings. One slot is created per credential-using
// node so credentials are never copied into the generated app.
func ExternalizeCredentials(nodesJSON string) (string, []pack.CredentialRequirement, []pack.Binding, error) {
	var list []nodes.Node
	if err := json.Unmarshal([]byte(nodesJSON), &list); err != nil {
		return "", nil, nil, err
	}
	requirements := []pack.CredentialRequirement{}
	bindings := []pack.Binding{}
	for i := range list {
		if _, exists := list[i].Params["credential_id"]; !exists || !needsCredential(list[i]) {
			continue
		}
		slot := safeSlot(list[i].ID) + "_credential"
		list[i].Params["credential_id"] = ""
		for _, secretParam := range []string{"api_key", "token", "password", "webhook_url", "connection_string", "notion_token"} {
			if _, exists := list[i].Params[secretParam]; exists {
				list[i].Params[secretParam] = ""
			}
		}
		credentialType, kind, provider := credentialSpec(list[i])
		requirements = append(requirements, pack.CredentialRequirement{
			Key: slot, Label: displayName(list[i]), Description: "Thông tin xác thực cho " + displayName(list[i]),
			Type: credentialType, Kind: kind, Provider: provider, Required: true,
		})
		bindings = append(bindings, pack.Binding{Source: "credential." + slot, Target: pack.BindingTarget{NodeID: list[i].ID, Param: "credential_id"}})
	}
	encoded, err := json.Marshal(list)
	return string(encoded), requirements, bindings, err
}

func credentialSpec(node nodes.Node) (credentialType, kind, provider string) {
	switch node.Type {
	case nodes.TypeTelegramBot:
		return "TELEGRAM_BOT", "", ""
	case nodes.TypeOpenAIGPT:
		return "OPENAI_API_KEY", "", ""
	case nodes.TypeDeepSeekAI:
		return "DEEPSEEK_API_KEY", "", ""
	case nodes.TypeAIExtract:
		if strings.EqualFold(fmt.Sprint(node.Params["provider"]), "deepseek") {
			return "DEEPSEEK_API_KEY", "", ""
		}
		return "OPENAI_API_KEY", "", ""
	case nodes.TypeEmailSMTP:
		return "SMTP_ACCOUNT", "", ""
	case nodes.TypePostgresQuery, nodes.TypeMySQLQuery, nodes.TypeMongoDBCommand, nodes.TypeRedisCommand:
		return "DATABASE_URL", "", ""
	case nodes.TypeGoogleSheets, nodes.TypeGoogleDrive, nodes.TypeGmailREST:
		return "GOOGLE_SERVICE_ACCOUNT", "", ""
	case nodes.TypeSlackBot:
		return "API_KEY", "API_KEY", "slack"
	case nodes.TypeDiscordBot:
		return "API_KEY", "API_KEY", "discord"
	case nodes.TypeNotionPage:
		return "API_KEY", "API_KEY", "notion"
	case nodes.TypeZaloOA:
		return "BEARER_TOKEN", "BEARER_TOKEN", "zalo"
	default:
		return "API_KEY", "", ""
	}
}

func needsCredential(node nodes.Node) bool {
	for _, param := range []string{"credential_id", "api_key", "token", "password", "webhook_url", "connection_string", "notion_token"} {
		if strings.TrimSpace(fmt.Sprint(node.Params[param])) != "" && fmt.Sprint(node.Params[param]) != "<nil>" {
			return true
		}
	}
	switch node.Type {
	case nodes.TypeTelegramBot, nodes.TypeEmailSMTP, nodes.TypeOpenAIGPT, nodes.TypeDeepSeekAI, nodes.TypeAIExtract,
		nodes.TypeDiscordBot, nodes.TypeSlackBot, nodes.TypePostgresQuery, nodes.TypeRedisCommand,
		nodes.TypeGoogleSheets, nodes.TypeMySQLQuery, nodes.TypeMongoDBCommand, nodes.TypeGoogleDrive,
		nodes.TypeGmailREST, nodes.TypeNotionPage, nodes.TypeZaloOA:
		return true
	default:
		return false
	}
}

func safeSlot(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			out.WriteRune(r)
		} else {
			out.WriteByte('_')
		}
	}
	result := strings.Trim(out.String(), "_")
	if result == "" {
		return "node"
	}
	if len(result) > 48 {
		return result[:48]
	}
	return result
}

type Report struct {
	Level    Level        `json:"level"`
	CanBuild bool         `json:"can_build"`
	Summary  string       `json:"summary"`
	Nodes    []NodeResult `json:"nodes"`
	Warnings []string     `json:"warnings,omitempty"`
	Blockers []string     `json:"blockers,omitempty"`
}

func Analyze(nodesJSON string) (Report, error) {
	var list []nodes.Node
	if err := json.Unmarshal([]byte(nodesJSON), &list); err != nil {
		return Report{}, fmt.Errorf("nodes_json: %w", err)
	}
	report := Report{Level: Green, CanBuild: true, Nodes: make([]NodeResult, 0, len(list))}
	for _, node := range list {
		level, reason := classify(node.Type)
		result := NodeResult{NodeID: node.ID, Name: node.Name, Type: string(node.Type), Level: level, Reason: reason}
		report.Nodes = append(report.Nodes, result)
		switch level {
		case Red:
			report.Level = Red
			report.CanBuild = false
			report.Blockers = append(report.Blockers, fmt.Sprintf("%s (%s): %s", displayName(node), node.Type, reason))
		case Yellow:
			if report.Level != Red {
				report.Level = Yellow
			}
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s (%s): %s", displayName(node), node.Type, reason))
		}
	}
	switch report.Level {
	case Green:
		report.Summary = "Workflow dùng các thành phần có thể đóng gói cùng Goflow runtime."
	case Yellow:
		report.Summary = "Có thể build, nhưng ứng dụng cần mạng hoặc cấu hình kết nối ở máy đích."
	case Red:
		report.Summary = "Chưa thể build thành ứng dụng độc lập vì có phụ thuộc cục bộ hoặc runtime ngoài."
	}
	return report, nil
}

func classify(nodeType nodes.NodeType) (Level, string) {
	switch nodeType {
	case nodes.TypePythonCode:
		return Red, "cần Python được cài bên ngoài ứng dụng"
	case nodes.TypeSubWorkflow:
		return Red, "workflow con chưa được gom vào một file ứng dụng"
	case nodes.TypeGoflowPlugin:
		return Red, "plugin native chưa được hỗ trợ trong app một file"
	case nodes.TypeSSHRunner, nodes.TypeGitCommand:
		return Red, "phụ thuộc công cụ hoặc môi trường hệ thống bên ngoài"
	case nodes.TypeFileTrigger, nodes.TypeLocalFile, nodes.TypeTableFile:
		return Red, "phụ thuộc đường dẫn hoặc file cục bộ chưa portable"
	case nodes.TypeHTTPRequest, nodes.TypeNormalizedHTTPSource, nodes.TypeRSSFeedSource,
		nodes.TypeTelegramBot, nodes.TypeEmailSMTP, nodes.TypeOpenAIGPT, nodes.TypeDeepSeekAI,
		nodes.TypeAIExtract, nodes.TypeZaloOA,
		nodes.TypeDiscordBot, nodes.TypeSlackBot, nodes.TypePostgresQuery, nodes.TypeRedisCommand,
		nodes.TypeGoogleSheets, nodes.TypeMySQLQuery, nodes.TypeMongoDBCommand, nodes.TypeGoogleDrive,
		nodes.TypeGmailREST, nodes.TypeNotionPage, nodes.TypeGithubWebhook:
		return Yellow, "cần mạng, credential hoặc dịch vụ bên ngoài ở máy đích"
	default:
		return Green, "chạy bằng Goflow runtime được đóng gói"
	}
}

func displayName(node nodes.Node) string {
	if strings.TrimSpace(node.Name) != "" {
		return node.Name
	}
	return node.ID
}
