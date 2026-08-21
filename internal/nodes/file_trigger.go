package nodes

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"goflow/internal/fileref"
)

const maxFileTriggerEvents = 100

type FileTriggerExecutor struct {
	store *fileref.Store
}

type fileTriggerStamp struct {
	Size    int64 `json:"size"`
	ModUnix int64 `json:"mod_unix"`
}

func NewFileTriggerExecutor() *FileTriggerExecutor {
	return &FileTriggerExecutor{store: fileref.DefaultStore()}
}
func NewFileTriggerExecutorWithStore(store *fileref.Store) *FileTriggerExecutor {
	if store == nil {
		store = fileref.DefaultStore()
	}
	return &FileTriggerExecutor{store: store}
}

func fileTriggerStateKey(path, pattern string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path) + "\x00" + pattern))
	return "file_trigger:" + hex.EncodeToString(sum[:16])
}

func decodeFileTriggerInt64(raw interface{}) (int64, bool) {
	switch typed := raw.(type) {
	case string:
		value, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return value, err == nil
	default:
		value, ok := conditionNumber(raw)
		return int64(value), ok
	}
}

func decodeFileTriggerSnapshot(raw interface{}) map[string]fileTriggerStamp {
	result := map[string]fileTriggerStamp{}
	if raw == nil {
		return result
	}
	if typed, ok := raw.(map[string]interface{}); ok {
		for name, value := range typed {
			item, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			size, sizeOK := decodeFileTriggerInt64(item["size"])
			mod, modOK := decodeFileTriggerInt64(item["mod_unix"])
			if !sizeOK || !modOK {
				continue
			}
			result[name] = fileTriggerStamp{Size: size, ModUnix: mod}
		}
	}
	return result
}

func encodeFileTriggerSnapshot(snapshot map[string]fileTriggerStamp) map[string]interface{} {
	result := make(map[string]interface{}, len(snapshot))
	for name, stamp := range snapshot {
		// Store int64 values as decimal strings so JSON round-trips do not lose
		// nanosecond timestamp precision through float64 decoding.
		result[name] = map[string]interface{}{
			"size":     strconv.FormatInt(stamp.Size, 10),
			"mod_unix": strconv.FormatInt(stamp.ModUnix, 10),
		}
	}
	return result
}

func scanFileTriggerDirectory(path, pattern string) (map[string]fileTriggerStamp, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := map[string]fileTriggerStamp{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if pattern != "" {
			matched, err := filepath.Match(pattern, entry.Name())
			if err != nil {
				return nil, fmt.Errorf("invalid file trigger pattern: %w", err)
			}
			if !matched {
				continue
			}
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		result[entry.Name()] = fileTriggerStamp{Size: info.Size(), ModUnix: info.ModTime().UnixNano()}
	}
	return result, nil
}

func (e *FileTriggerExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	path, err := resolveLocalFilePath(conditionValueString(node.Params["path"]), false)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("file trigger path must be a directory")
	}
	pattern := strings.TrimSpace(conditionValueString(node.Params["pattern"]))
	current, err := scanFileTriggerDirectory(path, pattern)
	if err != nil {
		return nil, err
	}
	key := fileTriggerStateKey(path, pattern)
	rawPrevious, found, err := workflowStateGet(ctx, "workflow", key)
	if err != nil {
		return nil, err
	}
	previous := decodeFileTriggerSnapshot(rawPrevious)
	emitExisting, _ := node.Params["emit_existing"].(bool)

	names := make([]string, 0, len(current))
	for name := range current {
		names = append(names, name)
	}
	sort.Strings(names)
	events := make([]map[string]interface{}, 0)
	for _, name := range names {
		stamp := current[name]
		old, existed := previous[name]
		eventType := ""
		if !found || !existed {
			if found || emitExisting {
				eventType = "CREATED"
			}
		} else if old.Size != stamp.Size || old.ModUnix != stamp.ModUnix {
			eventType = "MODIFIED"
		}
		if eventType == "" {
			continue
		}
		if len(events) >= maxFileTriggerEvents {
			return nil, fmt.Errorf("file trigger produced more than %d events; narrow the pattern", maxFileTriggerEvents)
		}
		fullPath := filepath.Join(path, name)
		ref, err := e.store.PutPath(fullPath)
		if err != nil {
			return nil, err
		}
		events = append(events, map[string]interface{}{
			"event": eventType, "name": name, "path": fullPath, "file": ref, "size": stamp.Size, "modified_at": time.Unix(0, stamp.ModUnix),
		})
	}
	if err := workflowStateSet(ctx, "workflow", key, encodeFileTriggerSnapshot(current)); err != nil {
		return nil, err
	}
	return map[string]interface{}{"events": events, "count": len(events), "path": path, "pattern": pattern}, nil
}

func (e *FileTriggerExecutor) Validate(node *Node) error {
	path, _ := node.Params["path"].(string)
	if strings.TrimSpace(path) == "" && !containsTemplateExpression(path) {
		return fmt.Errorf("file trigger path is required")
	}
	return nil
}

func (e *FileTriggerExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeFileTrigger, Name: "File Watch (Polling)", Description: "Detects created or modified local files by comparing a restart-safe snapshot; schedule it with Cron for periodic watching", Icon: "FolderSearch", Category: "TRIGGER", Retryable: false,
		Params: []ParamDefinition{
			{Name: "path", Label: "Folder", Type: "text", Default: "", Required: true, Description: "Folder constrained by GOFLOW_FILE_ALLOWED_ROOTS"},
			{Name: "pattern", Label: "Pattern", Type: "text", Default: "*", Required: false, Description: "Glob such as *.xlsx"},
			{Name: "emit_existing", Label: "Emit Existing Files on First Run", Type: "boolean", Default: false, Required: false},
		},
	}
}
