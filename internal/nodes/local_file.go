package nodes

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"goflow/internal/fileref"
)

const maxLocalFileListEntries = 1000

type LocalFileExecutor struct {
	store *fileref.Store
}

func NewLocalFileExecutor() *LocalFileExecutor { return &LocalFileExecutor{store: fileref.DefaultStore()} }
func NewLocalFileExecutorWithStore(store *fileref.Store) *LocalFileExecutor {
	if store == nil {
		store = fileref.DefaultStore()
	}
	return &LocalFileExecutor{store: store}
}

func localFileAllowedRoots() ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("GOFLOW_FILE_ALLOWED_ROOTS"))
	var roots []string
	if raw != "" {
		roots = filepath.SplitList(raw)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		roots = []string{cwd}
	}
	result := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			abs = resolved
		}
		result = append(result, filepath.Clean(abs))
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no local file roots are configured")
	}
	return result, nil
}

func pathWithinRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	prefix := root + string(os.PathSeparator)
	return strings.HasPrefix(path, prefix)
}

func resolveLocalFilePath(raw string, forWrite bool) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("local file path is required")
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	candidate := abs
	if forWrite {
		parent := filepath.Dir(abs)
		if resolvedParent, err := filepath.EvalSymlinks(parent); err == nil {
			candidate = filepath.Join(resolvedParent, filepath.Base(abs))
		}
	} else if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		candidate = resolved
	}
	roots, err := localFileAllowedRoots()
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		if pathWithinRoot(candidate, root) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("local file path is outside GOFLOW_FILE_ALLOWED_ROOTS")
}

func normalizeLocalFileOperation(raw interface{}) (string, error) {
	op := strings.ToUpper(strings.TrimSpace(conditionValueString(raw)))
	if op == "" {
		op = "READ"
	}
	switch op {
	case "READ", "WRITE", "LIST":
		return op, nil
	default:
		return "", fmt.Errorf("local file operation must be READ, WRITE, or LIST")
	}
}

func (e *LocalFileExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	op, err := normalizeLocalFileOperation(node.Params["operation"])
	if err != nil {
		return nil, err
	}
	pathText := conditionValueString(node.Params["path"])

	switch op {
	case "READ":
		path, err := resolveLocalFilePath(pathText, false)
		if err != nil {
			return nil, err
		}
		ref, err := e.store.PutPath(path)
		if err != nil {
			return nil, err
		}
		return ref, nil
	case "WRITE":
		path, err := resolveLocalFilePath(pathText, true)
		if err != nil {
			return nil, err
		}
		if createDirs, _ := node.Params["create_directories"].(bool); createDirs {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return nil, err
			}
		}
		var data []byte
		if rawRef := node.Params["file_ref"]; rawRef != nil && strings.TrimSpace(conditionValueString(rawRef)) != "" {
			ref, err := fileref.Parse(rawRef)
			if err != nil {
				return nil, err
			}
			data, err = e.store.ReadAll(ref)
			if err != nil {
				return nil, err
			}
		} else {
			data = []byte(conditionValueString(node.Params["content"]))
		}
		if int64(len(data)) > e.store.MaxBytes && e.store.MaxBytes > 0 {
			return nil, fmt.Errorf("local file content exceeds %d byte limit", e.store.MaxBytes)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			return nil, err
		}
		return map[string]interface{}{"written": true, "path": path, "size": len(data)}, nil
	case "LIST":
		path, err := resolveLocalFilePath(pathText, false)
		if err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		pattern := strings.TrimSpace(conditionValueString(node.Params["pattern"]))
		result := make([]map[string]interface{}, 0, len(entries))
		for _, entry := range entries {
			if pattern != "" {
				matched, matchErr := filepath.Match(pattern, entry.Name())
				if matchErr != nil {
					return nil, fmt.Errorf("invalid local file pattern: %w", matchErr)
				}
				if !matched {
					continue
				}
			}
			if len(result) >= maxLocalFileListEntries {
				return nil, fmt.Errorf("local file listing exceeds %d entry limit", maxLocalFileListEntries)
			}
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			result = append(result, map[string]interface{}{
				"name": entry.Name(), "path": filepath.Join(path, entry.Name()), "is_dir": entry.IsDir(), "size": info.Size(), "modified_at": info.ModTime(),
			})
		}
		sort.Slice(result, func(i, j int) bool { return result[i]["name"].(string) < result[j]["name"].(string) })
		return map[string]interface{}{"entries": result, "count": len(result), "path": path}, nil
	}
	return nil, fmt.Errorf("unsupported local file operation")
}

func (e *LocalFileExecutor) Validate(node *Node) error {
	_, err := normalizeLocalFileOperation(node.Params["operation"])
	return err
}

func (e *LocalFileExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeLocalFile, Name: "Local File", Description: "Reads, writes, or lists files inside explicitly allowed local roots", Icon: "File", Category: "DATA", Retryable: false,
		Params: []ParamDefinition{
			{Name: "operation", Label: "Operation", Type: "select", Default: "READ", Options: []string{"READ", "WRITE", "LIST"}, Required: true},
			{Name: "path", Label: "Path", Type: "text", Default: "", Required: true, Description: "Absolute or relative path constrained by GOFLOW_FILE_ALLOWED_ROOTS"},
			{Name: "pattern", Label: "List Pattern", Type: "text", Default: "", Required: false, Description: "Optional glob such as *.csv for LIST"},
			{Name: "content", Label: "Text Content", Type: "textarea", Default: "", Required: false, Description: "Text content for WRITE when FileRef is not provided"},
			{Name: "file_ref", Label: "FileRef", Type: "json", Default: "", Required: false, Description: "Managed FileRef to copy for WRITE"},
			{Name: "create_directories", Label: "Create Parent Directories", Type: "boolean", Default: false, Required: false},
		},
	}
}
