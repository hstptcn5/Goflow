package nodes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	defaultAIExtractBaseURL  = "https://api.openai.com"
	maxAIExtractResponse     = int64(2 << 20)
	defaultAIExtractMediaMax = int64(20 << 20)
	maxAIExtractInlineChars  = 32 << 20
)

type AIExtractExecutor struct {
	client            *http.Client
	mediaClient       *http.Client
	baseURL           string
	allowPrivateMedia bool
}

func NewAIExtractExecutor() *AIExtractExecutor {
	return &AIExtractExecutor{
		client:      &http.Client{Timeout: 90 * time.Second},
		mediaClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:     defaultAIExtractBaseURL,
	}
}

func NewAIExtractExecutorWithClients(client, mediaClient *http.Client, baseURL string, allowPrivateMedia bool) *AIExtractExecutor {
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	if mediaClient == nil {
		mediaClient = &http.Client{Timeout: 60 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultAIExtractBaseURL
	}
	return &AIExtractExecutor{client: client, mediaClient: mediaClient, baseURL: strings.TrimRight(baseURL, "/"), allowPrivateMedia: allowPrivateMedia}
}

func (e *AIExtractExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	apiKey, err := aiExtractAPIKey(ctx, node)
	if err != nil {
		return nil, err
	}
	request, err := parseAIExtractRequest(node)
	if err != nil {
		return nil, err
	}
	policy, err := aiExtractSourcePolicy(ctx, request.SourcePolicyNodeID)
	if err != nil {
		return nil, err
	}

	inputValue := request.Input
	transcript := ""
	if request.InputType == "media_url" || request.InputType == "media_data" {
		mediaBytes, filename, err := e.resolveMedia(ctx.Context, request)
		if err != nil {
			return nil, err
		}
		transcript, err = e.transcribe(ctx.Context, apiKey, mediaBytes, filename, request.TranscriptionModel, request.Language)
		if err != nil {
			return nil, err
		}
		inputValue = transcript
	}

	payload := map[string]interface{}{
		"model":        request.Model,
		"store":        false,
		"instructions": "Extract only facts supported by the supplied input. Do not invent missing values. Return data that follows the requested JSON schema.",
		"input": []map[string]interface{}{
			{
				"role":    "user",
				"content": aiExtractContent(request.InputType, request.Instructions, inputValue, request.Filename),
			},
		},
		"text": map[string]interface{}{
			"format": map[string]interface{}{
				"type":   "json_schema",
				"name":   request.SchemaName,
				"schema": request.Schema,
				"strict": true,
			},
		},
	}

	result, rawText, err := e.callResponses(ctx.Context, apiKey, payload)
	if err != nil {
		return nil, err
	}
	var structured interface{}
	if err := json.Unmarshal([]byte(rawText), &structured); err != nil {
		return nil, fmt.Errorf("AI Extract returned non-JSON structured output: %w", err)
	}
	output := map[string]interface{}{
		"data":          structured,
		"raw_text":      rawText,
		"model_used":    request.Model,
		"input_type":    request.InputType,
		"response_id":   result["id"],
		"source_policy": policy,
	}
	if transcript != "" {
		output["transcript"] = transcript
	}
	return output, nil
}

func (e *AIExtractExecutor) Validate(node *Node) error {
	_, err := parseAIExtractRequest(node)
	return err
}

type aiExtractRequest struct {
	Model              string
	InputType          string
	Input              string
	Filename           string
	Instructions       string
	SchemaName         string
	Schema             map[string]interface{}
	SourcePolicyNodeID string
	TranscriptionModel string
	Language           string
	MaxMediaBytes      int64
}

func parseAIExtractRequest(node *Node) (aiExtractRequest, error) {
	stringParam := func(name, defaultValue string) string {
		value, _ := node.Params[name].(string)
		value = strings.TrimSpace(value)
		if value == "" {
			return defaultValue
		}
		return value
	}

	request := aiExtractRequest{
		Model:              stringParam("model", "gpt-5"),
		InputType:          stringParam("input_type", "text"),
		Input:              stringParam("input", ""),
		Filename:           stringParam("filename", ""),
		Instructions:       stringParam("instructions", "Extract the important factual information."),
		SchemaName:         stringParam("schema_name", "extracted_data"),
		SourcePolicyNodeID: stringParam("source_policy_node_id", ""),
		TranscriptionModel: stringParam("transcription_model", "gpt-4o-mini-transcribe"),
		Language:           stringParam("language", ""),
		MaxMediaBytes:      defaultAIExtractMediaMax,
	}
	if request.Input == "" {
		return request, fmt.Errorf("AI Extract requires input")
	}
	allowedTypes := map[string]bool{"text": true, "image_url": true, "image_data": true, "file_url": true, "file_data": true, "media_url": true, "media_data": true}
	if !allowedTypes[request.InputType] {
		return request, fmt.Errorf("AI Extract input_type must be text, image_url, image_data, file_url, file_data, media_url, or media_data")
	}
	if len(request.Input) > maxAIExtractInlineChars && request.InputType != "media_url" && request.InputType != "file_url" && request.InputType != "image_url" {
		return request, fmt.Errorf("AI Extract inline input exceeds %d character limit", maxAIExtractInlineChars)
	}
	if request.InputType == "image_url" || request.InputType == "file_url" || request.InputType == "media_url" {
		if err := validateHTTPURL(request.Input); err != nil {
			return request, fmt.Errorf("AI Extract %s: %w", request.InputType, err)
		}
	}
	if request.InputType == "image_data" {
		normalized, err := normalizeImageData(request.Input, request.Filename)
		if err != nil {
			return request, err
		}
		request.Input = normalized
	}
	if (request.InputType == "file_data" || request.InputType == "media_data") && request.Filename == "" {
		return request, fmt.Errorf("AI Extract filename is required for %s", request.InputType)
	}
	if request.InputType == "media_url" && request.Filename == "" {
		request.Filename = path.Base(mustParseURL(request.Input).Path)
	}
	if request.InputType == "media_url" || request.InputType == "media_data" {
		if !supportedTranscriptionFilename(request.Filename) {
			return request, fmt.Errorf("AI Extract media filename must use flac, mp3, mp4, mpeg, mpga, m4a, ogg, wav, or webm")
		}
	}
	maxMedia, err := positiveInt64Param(node.Params["max_media_bytes"], defaultAIExtractMediaMax)
	if err != nil || maxMedia > 25<<20 {
		return request, fmt.Errorf("AI Extract max_media_bytes must be between 1 and 26214400")
	}
	request.MaxMediaBytes = maxMedia

	schemaRaw, ok := node.Params["json_schema"]
	if !ok {
		return request, fmt.Errorf("AI Extract requires json_schema")
	}
	schema, err := parseJSONObject(schemaRaw)
	if err != nil {
		return request, fmt.Errorf("AI Extract json_schema: %w", err)
	}
	if schemaType, _ := schema["type"].(string); schemaType != "object" {
		return request, fmt.Errorf("AI Extract json_schema top-level type must be object")
	}
	request.Schema = schema
	if request.SchemaName == "" || len(request.SchemaName) > 64 {
		return request, fmt.Errorf("AI Extract schema_name must be 1-64 characters")
	}
	return request, nil
}

func aiExtractAPIKey(ctx *ExecutionContext, node *Node) (string, error) {
	apiKey, _ := node.Params["api_key"].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	if strings.TrimSpace(credentialID) != "" {
		secret, ok := ctx.Credentials[strings.TrimSpace(credentialID)]
		if !ok || strings.TrimSpace(secret) == "" {
			return "", fmt.Errorf("AI Extract credential is not available")
		}
		apiKey = secret
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("AI Extract requires an OpenAI API key or credential")
	}
	return strings.TrimSpace(apiKey), nil
}

func aiExtractSourcePolicy(ctx *ExecutionContext, nodeID string) (interface{}, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, nil
	}
	value, ok := ctx.GetOutput(strings.TrimSpace(nodeID))
	if !ok {
		return nil, fmt.Errorf("AI Extract source policy output %q is not available", nodeID)
	}
	policy, ok := value.(map[string]interface{})
	if !ok || policy["allowed"] != true || policy["policy_enforced"] != true {
		return nil, fmt.Errorf("AI Extract source policy output %q is not an allowed Source Policy result", nodeID)
	}
	return policy, nil
}

func aiExtractContent(inputType, instructions, input, filename string) []map[string]interface{} {
	content := []map[string]interface{}{{"type": "input_text", "text": instructions}}
	switch inputType {
	case "image_url", "image_data":
		content = append(content, map[string]interface{}{"type": "input_image", "image_url": input, "detail": "auto"})
	case "file_url":
		content = append(content, map[string]interface{}{"type": "input_file", "file_url": input})
	case "file_data":
		content = append(content, map[string]interface{}{"type": "input_file", "file_data": normalizeFileData(input), "filename": filename})
	default:
		content = append(content, map[string]interface{}{"type": "input_text", "text": input})
	}
	return content
}

func normalizeImageData(input, filename string) (string, error) {
	if strings.HasPrefix(input, "data:image/") {
		parts := strings.SplitN(input, ",", 2)
		if len(parts) != 2 || !strings.HasSuffix(parts[0], ";base64") {
			return "", fmt.Errorf("AI Extract image_data must be a base64 image data URL")
		}
		mimeType := strings.TrimPrefix(strings.TrimSuffix(parts[0], ";base64"), "data:")
		if !supportedImageMIME(mimeType) {
			return "", fmt.Errorf("AI Extract image_data supports PNG, JPEG, WEBP, or GIF")
		}
		if _, err := base64.StdEncoding.DecodeString(parts[1]); err != nil {
			return "", fmt.Errorf("AI Extract image_data is not valid base64")
		}
		return input, nil
	}
	if filename == "" {
		return "", fmt.Errorf("AI Extract filename is required for raw image_data")
	}
	mimeType := imageMIMEFromFilename(filename)
	if mimeType == "" {
		return "", fmt.Errorf("AI Extract image filename must use png, jpg, jpeg, webp, or gif")
	}
	if _, err := base64.StdEncoding.DecodeString(input); err != nil {
		return "", fmt.Errorf("AI Extract image_data is not valid base64")
	}
	return "data:" + mimeType + ";base64," + input, nil
}

func supportedImageMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func imageMIMEFromFilename(filename string) string {
	switch strings.ToLower(strings.TrimPrefix(path.Ext(filename), ".")) {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	default:
		return ""
	}
}

func normalizeFileData(input string) string {
	if strings.HasPrefix(input, "data:") {
		if comma := strings.IndexByte(input, ','); comma >= 0 {
			return input[comma+1:]
		}
	}
	return input
}

func (e *AIExtractExecutor) callResponses(ctx context.Context, apiKey string, payload map[string]interface{}) (map[string]interface{}, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("AI Extract response request could not be created")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("AI Extract could not connect to OpenAI: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := readBounded(resp.Body, maxAIExtractResponse)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("AI Extract OpenAI response failed with HTTP %d: %s", resp.StatusCode, boundedErrorText(respBody))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("AI Extract could not parse OpenAI response: %w", err)
	}
	text := responseOutputText(result)
	if strings.TrimSpace(text) == "" {
		return nil, "", fmt.Errorf("AI Extract OpenAI response did not contain output_text")
	}
	return result, text, nil
}

func (e *AIExtractExecutor) resolveMedia(ctx context.Context, request aiExtractRequest) ([]byte, string, error) {
	if request.InputType == "media_data" {
		data := normalizeFileData(request.Input)
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, "", fmt.Errorf("AI Extract media_data is not valid base64")
		}
		if int64(len(decoded)) > request.MaxMediaBytes {
			return nil, "", fmt.Errorf("AI Extract media exceeds %d byte limit", request.MaxMediaBytes)
		}
		return decoded, request.Filename, nil
	}
	if !e.allowPrivateMedia {
		if err := rejectPrivateLiteralURL(request.Input); err != nil {
			return nil, "", err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.Input, nil)
	if err != nil {
		return nil, "", fmt.Errorf("AI Extract media request could not be created")
	}
	resp, err := e.mediaClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("AI Extract could not download media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("AI Extract media download failed with HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > request.MaxMediaBytes {
		return nil, "", fmt.Errorf("AI Extract media exceeds %d byte limit", request.MaxMediaBytes)
	}
	data, err := readBounded(resp.Body, request.MaxMediaBytes)
	if err != nil {
		return nil, "", fmt.Errorf("AI Extract media download: %w", err)
	}
	return data, request.Filename, nil
}

func (e *AIExtractExecutor) transcribe(ctx context.Context, apiKey string, data []byte, filename, model, language string) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fileWriter.Write(data); err != nil {
		return "", err
	}
	if err := writer.WriteField("model", model); err != nil {
		return "", err
	}
	if language != "" {
		if err := writer.WriteField("language", language); err != nil {
			return "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/audio/transcriptions", &body)
	if err != nil {
		return "", fmt.Errorf("AI Extract transcription request could not be created")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI Extract transcription request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := readBounded(resp.Body, maxAIExtractResponse)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI Extract transcription failed with HTTP %d: %s", resp.StatusCode, boundedErrorText(respBody))
	}
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil || strings.TrimSpace(result.Text) == "" {
		return "", fmt.Errorf("AI Extract transcription response did not contain text")
	}
	return result.Text, nil
}

func responseOutputText(result map[string]interface{}) string {
	outputs, _ := result["output"].([]interface{})
	for _, output := range outputs {
		message, _ := output.(map[string]interface{})
		contents, _ := message["content"].([]interface{})
		for _, content := range contents {
			part, _ := content.(map[string]interface{})
			if part["type"] == "output_text" {
				if text, _ := part["text"].(string); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func parseJSONObject(value interface{}) (map[string]interface{}, error) {
	switch typed := value.(type) {
	case map[string]interface{}:
		return typed, nil
	case string:
		var object map[string]interface{}
		if err := json.Unmarshal([]byte(typed), &object); err != nil {
			return nil, err
		}
		return object, nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var object map[string]interface{}
		if err := json.Unmarshal(encoded, &object); err != nil {
			return nil, err
		}
		return object, nil
	}
}

func positiveInt64Param(value interface{}, defaultValue int64) (int64, error) {
	if value == nil {
		return defaultValue, nil
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return int64(typed), nil
		}
	case int64:
		if typed > 0 {
			return typed, nil
		}
	case float64:
		if typed > 0 && typed == float64(int64(typed)) {
			return int64(typed), nil
		}
	}
	return 0, fmt.Errorf("must be a positive integer")
}

func readBounded(reader io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("response exceeds %d byte limit", max)
	}
	return data, nil
}

func boundedErrorText(data []byte) string {
	text := strings.TrimSpace(string(data))
	if len(text) > 2048 {
		text = text[:2048]
	}
	return text
}

func validateHTTPURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("requires an absolute http/https URL")
	}
	return nil
}

func mustParseURL(raw string) *url.URL {
	parsed, _ := url.Parse(raw)
	return parsed
}

func rejectPrivateLiteralURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("AI Extract media URL is invalid")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("AI Extract media_url may not target localhost")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return fmt.Errorf("AI Extract media_url may not target a private or local address")
	}
	return nil
}

func supportedTranscriptionFilename(filename string) bool {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(filename), "."))
	switch ext {
	case "flac", "mp3", "mp4", "mpeg", "mpga", "m4a", "ogg", "wav", "webm":
		return true
	default:
		return false
	}
}

func (e *AIExtractExecutor) GetDefinition() NodeDefinition {
	defaultSchema := `{"type":"object","properties":{"summary":{"type":"string"},"facts":{"type":"array","items":{"type":"string"}}},"required":["summary","facts"],"additionalProperties":false}`
	return NodeDefinition{
		Type:        TypeAIExtract,
		Name:        "AI Extract",
		Description: "Extracts schema-bound structured data from text, images, documents, and audio/video speech",
		Icon:        "ScanSearch",
		Category:    "AI & LLM",
		Retryable:   true,
		Params: []ParamDefinition{
			{Name: "model", Label: "Extraction Model", Type: "text", Default: "gpt-5", Required: true},
			{Name: "input_type", Label: "Input Type", Type: "select", Default: "text", Options: []string{"text", "image_url", "image_data", "file_url", "file_data", "media_url", "media_data"}, Required: true, Description: "media_* transcribes the audio track first; video frames are not analyzed in this version"},
			{Name: "input", Label: "Input / URL / Base64", Type: "textarea", Required: true},
			{Name: "filename", Label: "Filename", Type: "text", Default: "", Required: false, Description: "Required for raw image_data, file_data, and media_data. Image data supports PNG/JPEG/WEBP/GIF; media supports flac/mp3/mp4/mpeg/mpga/m4a/ogg/wav/webm."},
			{Name: "instructions", Label: "Extraction Instructions", Type: "textarea", Default: "Extract the important factual information.", Required: true},
			{Name: "json_schema", Label: "Output JSON Schema", Type: "json", Default: defaultSchema, Required: true},
			{Name: "schema_name", Label: "Schema Name", Type: "text", Default: "extracted_data", Required: true},
			{Name: "source_policy_node_id", Label: "Source Policy Node ID", Type: "text", Default: "", Required: false, Description: "Optional upstream sourcePolicy node. When set, extraction fails unless that node produced an allowed policy result."},
			{Name: "transcription_model", Label: "Transcription Model", Type: "text", Default: "gpt-4o-mini-transcribe", Required: false},
			{Name: "language", Label: "Audio Language", Type: "text", Default: "", Required: false, Description: "Optional ISO-639-1 language code such as vi or en"},
			{Name: "max_media_bytes", Label: "Max Media Bytes", Type: "integer", Default: int(defaultAIExtractMediaMax), Required: true},
			{Name: "api_key", Label: "OpenAI API Key", Type: "text", Default: "", Required: false},
			{Name: "credential_id", Label: "OpenAI Credential", Type: "credential", Default: "", Required: false},
		},
	}
}
