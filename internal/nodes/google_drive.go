package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

type GoogleDriveExecutor struct{}

func NewGoogleDriveExecutor() *GoogleDriveExecutor {
	return &GoogleDriveExecutor{}
}

func (e *GoogleDriveExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// 1. Resolve parameters
	credID, _ := node.Params["credential_id"].(string)
	directSA, _ := node.Params["service_account_json"].(string)
	action, _ := node.Params["action"].(string)
	folderID, _ := node.Params["folder_id"].(string)
	filename, _ := node.Params["filename"].(string)
	fileContent, _ := node.Params["content"].(string)

	action = strings.ToUpper(strings.TrimSpace(action))
	if action == "" {
		action = "LIST"
	}

	saJSON := directSA
	if credID != "" {
		ctx.mu.RLock()
		decrypted, ok := ctx.Credentials[credID]
		ctx.mu.RUnlock()
		if ok && decrypted != "" {
			saJSON = decrypted
		}
	}

	if strings.TrimSpace(saJSON) == "" {
		return nil, fmt.Errorf("service_account_json is empty (please set it directly or select a valid credential)")
	}

	// 2. Generate Google OAuth2 Token using JWT config
	jwtConfig, err := google.JWTConfigFromJSON([]byte(saJSON), "https://www.googleapis.com/auth/drive")
	if err != nil {
		return nil, fmt.Errorf("invalid service account JSON: %w", err)
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}
	ts := jwtConfig.TokenSource(context.Background())
	token, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to generate OAuth2 token: %w", err)
	}

	// 3. Perform REST requests
	if action == "LIST" {
		query := ""
		if folderID != "" {
			query = fmt.Sprintf("?q='%s'+in+parents+and+trashed=false", folderID)
		}
		url := "https://www.googleapis.com/drive/v3/files" + query

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Google Drive API error (status %d): %s", resp.StatusCode, string(respBytes))
		}

		var apiResult map[string]interface{}
		_ = json.Unmarshal(respBytes, &apiResult)
		return apiResult, nil

	} else if action == "UPLOAD" {
		if filename == "" {
			filename = fmt.Sprintf("goflow_upload_%d.txt", time.Now().Unix())
		}

		// Prepare multipart upload payload
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Metadata part
		metadata := map[string]interface{}{
			"name": filename,
		}
		if folderID != "" {
			metadata["parents"] = []string{folderID}
		}
		metadataJSON, _ := json.Marshal(metadata)

		metaHeader := make(textproto.MIMEHeader)
		metaHeader.Set("Content-Type", "application/json; charset=UTF-8")
		metaPart, err := writer.CreatePart(metaHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to create metadata part: %w", err)
		}
		_, _ = metaPart.Write(metadataJSON)

		// Content part
		mediaHeader := make(textproto.MIMEHeader)
		mediaHeader.Set("Content-Type", "text/plain")
		mediaPart, err := writer.CreatePart(mediaHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to create content part: %w", err)
		}
		_, _ = mediaPart.Write([]byte(fileContent))

		_ = writer.Close()

		url := "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart"

		req, err := http.NewRequest("POST", url, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create http request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		req.Header.Set("Content-Type", "multipart/related; boundary="+writer.Boundary())

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return nil, fmt.Errorf("Google Drive API upload error (status %d): %s", resp.StatusCode, string(respBytes))
		}

		var apiResult map[string]interface{}
		_ = json.Unmarshal(respBytes, &apiResult)
		return apiResult, nil
	}

	return nil, fmt.Errorf("unsupported Google Drive action: %s", action)
}

func (e *GoogleDriveExecutor) Validate(node *Node) error {
	return nil
}

func (e *GoogleDriveExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeGoogleDrive,
		Name:        "Google Drive",
		Description: "Tải tệp tin hoặc liệt kê các tệp trên Google Drive bằng Service Account",
		Icon:        "Folder",
		Category:    "COMMUNICATION",
		Params: []ParamDefinition{
			{
				Name:        "credential_id",
				Label:       "Select Encrypted Credential",
				Type:        "credential",
				Required:    false,
				Description: "Chọn tệp khóa Service Account JSON đã mã hóa từ Vault",
			},
			{
				Name:        "service_account_json",
				Label:       "Service Account JSON Key",
				Type:        "textarea",
				Required:    false,
				Description: "Dán nội dung khóa Service Account JSON trực tiếp (nếu không dùng Vault)",
			},
			{
				Name:        "action",
				Label:       "Action",
				Type:        "select",
				Default:     "LIST",
				Options:     []string{"LIST", "UPLOAD"},
				Required:    true,
				Description: "Chọn liệt kê tệp (LIST) hoặc tải lên tệp mới (UPLOAD)",
			},
			{
				Name:        "folder_id",
				Label:       "Folder ID (Optional)",
				Type:        "text",
				Required:    false,
				Description: "ID thư mục cha của Google Drive để liệt kê/tải lên",
			},
			{
				Name:        "filename",
				Label:       "Filename (For UPLOAD)",
				Type:        "text",
				Required:    false,
				Description: "Tên tệp tin sau khi được tải lên Google Drive",
			},
			{
				Name:        "content",
				Label:       "File Content (For UPLOAD)",
				Type:        "textarea",
				Required:    false,
				Description: "Nội dung văn bản bên trong tệp tin cần tải lên",
			},
		},
	}
}
