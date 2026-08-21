package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"goflow/internal/fileref"
)

const maxGoogleDriveUploadBytes = 100 << 20

type GoogleDriveExecutor struct {
	store *fileref.Store
}

func NewGoogleDriveExecutor() *GoogleDriveExecutor { return &GoogleDriveExecutor{store: fileref.DefaultStore()} }
func NewGoogleDriveExecutorWithStore(store *fileref.Store) *GoogleDriveExecutor {
	if store == nil {
		store = fileref.DefaultStore()
	}
	return &GoogleDriveExecutor{store: store}
}

func parseGoogleDriveAction(node *Node) (string, error) {
	action, _ := node.Params["action"].(string)
	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "LIST"
	}
	switch action {
	case "LIST", "DOWNLOAD", "UPLOAD", "DELETE":
		return action, nil
	default:
		return "", fmt.Errorf("unsupported Google Drive action: %s", action)
	}
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
	if action == "DOWNLOAD" || action == "DELETE" {
		fileID := strings.TrimSpace(conditionValueString(node.Params["file_id"]))
		if fileID == "" || len(fileID) > 1024 || strings.ContainsAny(fileID, "\r\n") {
			return fmt.Errorf("Google Drive file_id is required and must be valid")
		}
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
	material, err := resolveGoogleAuth(ctx, node)
	if err != nil {
		return nil, err
	}
	accessToken, err := googleAccessToken(ctx.Context, material, "https://www.googleapis.com/auth/drive")
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}

	switch action {
	case "LIST":
		endpoint, _ := url.Parse("https://www.googleapis.com/drive/v3/files")
		query := endpoint.Query()
		if strings.TrimSpace(folderID) != "" {
			safeFolder := strings.ReplaceAll(strings.TrimSpace(folderID), "'", "\\'")
			query.Set("q", fmt.Sprintf("'%s' in parents and trashed=false", safeFolder))
		}
		query.Set("pageSize", "100")
		query.Set("fields", "files(id,name,mimeType,size,modifiedTime),nextPageToken")
		endpoint.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(ctx.Context, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create Google Drive request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		return doGoogleJSONRequest(client, req, http.StatusOK)
	case "DOWNLOAD":
		fileID := strings.TrimSpace(conditionValueString(node.Params["file_id"]))
		name := strings.TrimSpace(conditionValueString(node.Params["filename"]))
		if name == "" {
			name = fileID
		}
		requestURL := "https://www.googleapis.com/drive/v3/files/" + url.PathEscape(fileID) + "?alt=media"
		req, err := http.NewRequestWithContext(ctx.Context, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Google Drive download failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxGoogleAPIResponseBytes))
			return nil, fmt.Errorf("Google Drive download error (status %d): %s", resp.StatusCode, boundedNodeErrorText(body))
		}
		mimeType := strings.TrimSpace(resp.Header.Get("Content-Type"))
		ref, err := e.store.PutReader(name, mimeType, resp.Body)
		if err != nil {
			return nil, err
		}
		return ref, nil
	case "DELETE":
		fileID := strings.TrimSpace(conditionValueString(node.Params["file_id"]))
		requestURL := "https://www.googleapis.com/drive/v3/files/" + url.PathEscape(fileID)
		req, err := http.NewRequestWithContext(ctx.Context, http.MethodDelete, requestURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Google Drive delete failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxGoogleAPIResponseBytes))
			return nil, fmt.Errorf("Google Drive delete error (status %d): %s", resp.StatusCode, boundedNodeErrorText(body))
		}
		return map[string]interface{}{"deleted": true, "file_id": fileID}, nil
	case "UPLOAD":
		return e.upload(ctx, client, accessToken, folderID, node)
	}
	return nil, fmt.Errorf("unsupported Google Drive action")
}

func (e *GoogleDriveExecutor) upload(ctx *ExecutionContext, client *http.Client, accessToken, folderID string, node *Node) (interface{}, error) {
	filename := strings.TrimSpace(conditionValueString(node.Params["filename"]))
	mimeType := "text/plain; charset=UTF-8"
	var media []byte
	if rawRef := node.Params["file_ref"]; rawRef != nil {
		if ref, err := fileref.Parse(rawRef); err == nil {
			data, err := e.store.ReadAll(ref)
			if err != nil {
				return nil, err
			}
			media = data
			if filename == "" {
				filename = ref.Name
			}
			if strings.TrimSpace(ref.MIME) != "" {
				mimeType = ref.MIME
			}
		} else if strings.TrimSpace(conditionValueString(rawRef)) != "" {
			return nil, err
		}
	}
	if media == nil {
		media = []byte(conditionValueString(node.Params["content"]))
	}
	if len(media) > maxGoogleDriveUploadBytes {
		return nil, fmt.Errorf("Google Drive upload exceeds %d byte limit", maxGoogleDriveUploadBytes)
	}
	if filename == "" {
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
	mediaHeader.Set("Content-Type", mimeType)
	mediaPart, err := writer.CreatePart(mediaHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create content part: %w", err)
	}
	if _, err := mediaPart.Write(media); err != nil {
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
		Type: TypeGoogleDrive, Name: "Google Drive", Description: "Lists, downloads, uploads, or deletes Google Drive files using managed FileRefs", Icon: "Folder", Category: "COMMUNICATION", Retryable: true,
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted Google OAuth2 access token or service account JSON"},
			{Name: "service_account_json", Label: "Service Account JSON Key (legacy)", Type: "textarea", Required: false, Description: "Legacy inline service account JSON. Prefer an encrypted credential."},
			{Name: "action", Label: "Action", Type: "select", Default: "LIST", Options: []string{"LIST", "DOWNLOAD", "UPLOAD", "DELETE"}, Required: true},
			{Name: "folder_id", Label: "Folder ID", Type: "text", Required: false},
			{Name: "file_id", Label: "File ID", Type: "text", Required: false, Description: "Required for DOWNLOAD/DELETE"},
			{Name: "filename", Label: "Filename", Type: "text", Required: false, Description: "Optional download name or upload filename"},
			{Name: "file_ref", Label: "FileRef", Type: "json", Required: false, Description: "Managed FileRef for UPLOAD"},
			{Name: "content", Label: "Text Content (legacy upload)", Type: "textarea", Required: false, Description: "Legacy text upload fallback"},
		},
	}
}
