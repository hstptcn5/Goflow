package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

const maxGoogleDriveUploadBytes = 5 << 20

type GoogleDriveExecutor struct{}

func NewGoogleDriveExecutor() *GoogleDriveExecutor { return &GoogleDriveExecutor{} }

func parseGoogleDriveAction(node *Node) (string, error) {
	action, _ := node.Params["action"].(string)
	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "LIST"
	}
	if action != "LIST" && action != "UPLOAD" {
		return "", fmt.Errorf("unsupported Google Drive action: %s", action)
	}
	return action, nil
}

func validateGoogleDriveNode(node *Node) error {
	action, err := parseGoogleDriveAction(node)
	if err != nil {
		return err
	}
	credentialID, _ := node.Params["credential_id"].(string)
	directSA, _ := node.Params["service_account_json"].(string)
	if strings.TrimSpace(credentialID) == "" && strings.TrimSpace(directSA) == "" {
		return fmt.Errorf("Google Drive requires an encrypted credential or service_account_json")
	}
	folderID, _ := node.Params["folder_id"].(string)
	if len(folderID) > 512 || strings.ContainsAny(folderID, "\r\n") {
		return fmt.Errorf("Google Drive folder_id is invalid")
	}
	if action == "UPLOAD" {
		filename, _ := node.Params["filename"].(string)
		if len([]rune(filename)) > 255 || strings.ContainsAny(filename, "\r\n") {
			return fmt.Errorf("Google Drive filename is invalid")
		}
		content, _ := node.Params["content"].(string)
		if len(content) > maxGoogleDriveUploadBytes {
			return fmt.Errorf("Google Drive upload content exceeds %d byte limit", maxGoogleDriveUploadBytes)
		}
	}
	return nil
}

func (e *GoogleDriveExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	if err := validateGoogleDriveNode(node); err != nil {
		return nil, err
	}
	action, _ := parseGoogleDriveAction(node)
	folderID, _ := node.Params["folder_id"].(string)
	filename, _ := node.Params["filename"].(string)
	fileContent, _ := node.Params["content"].(string)
	material, err := resolveGoogleAuth(ctx, node)
	if err != nil {
		return nil, err
	}
	accessToken, err := googleAccessToken(ctx.Context, material, "https://www.googleapis.com/auth/drive")
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 20 * time.Second}

	if action == "LIST" {
		endpoint, _ := url.Parse("https://www.googleapis.com/drive/v3/files")
		query := endpoint.Query()
		if strings.TrimSpace(folderID) != "" {
			safeFolder := strings.ReplaceAll(strings.TrimSpace(folderID), "'", "\\'")
			query.Set("q", fmt.Sprintf("'%s' in parents and trashed=false", safeFolder))
		}
		query.Set("pageSize", "100")
		endpoint.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(ctx.Context, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create Google Drive request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		return doGoogleJSONRequest(client, req, http.StatusOK)
	}

	if strings.TrimSpace(filename) == "" {
		filename = fmt.Sprintf("goflow_upload_%d.txt", time.Now().Unix())
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	metadata := map[string]interface{}{"name": filename}
	if strings.TrimSpace(folderID) != "" {
		metadata["parents"] = []string{strings.TrimSpace(folderID)}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Google Drive metadata: %w", err)
	}
	metaHeader := make(textproto.MIMEHeader)
	metaHeader.Set("Content-Type", "application/json; charset=UTF-8")
	metaPart, err := writer.CreatePart(metaHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata part: %w", err)
	}
	if _, err := metaPart.Write(metadataJSON); err != nil {
		return nil, fmt.Errorf("failed to write metadata part: %w", err)
	}
	mediaHeader := make(textproto.MIMEHeader)
	mediaHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	mediaPart, err := writer.CreatePart(mediaHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create content part: %w", err)
	}
	if _, err := mediaPart.Write([]byte(fileContent)); err != nil {
		return nil, fmt.Errorf("failed to write upload content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize Google Drive upload body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodPost, "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Drive upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "multipart/related; boundary="+writer.Boundary())
	return doGoogleJSONRequest(client, req, http.StatusOK, http.StatusCreated)
}

func (e *GoogleDriveExecutor) Validate(node *Node) error { return validateGoogleDriveNode(node) }

func (e *GoogleDriveExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeGoogleDrive, Name: "Google Drive", Description: "Uploads bounded text files or lists files in Google Drive", Icon: "Folder", Category: "COMMUNICATION", Retryable: true,
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted Google OAuth2 access token or service account JSON"},
			{Name: "service_account_json", Label: "Service Account JSON Key (legacy)", Type: "textarea", Required: false, Description: "Legacy inline service account JSON. Prefer an encrypted credential."},
			{Name: "action", Label: "Action", Type: "select", Default: "LIST", Options: []string{"LIST", "UPLOAD"}, Required: true, Description: "Choose LIST to list up to 100 files or UPLOAD to upload a text file"},
			{Name: "folder_id", Label: "Folder ID (Optional)", Type: "text", Required: false, Description: "Parent Google Drive folder ID"},
			{Name: "filename", Label: "Filename (For UPLOAD)", Type: "text", Required: false, Description: "Filename used for upload"},
			{Name: "content", Label: "File Content (For UPLOAD)", Type: "textarea", Required: false, Description: "Text content to upload, maximum 5 MiB"},
		},
	}
}
