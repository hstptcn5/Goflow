package fileref

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	TypeMarker          = "file"
	DefaultMaxFileBytes = int64(100 << 20)
)

type Ref struct {
	Type   string `json:"$type"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	MIME   string `json:"mime,omitempty"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

type Store struct {
	Root     string
	MaxBytes int64
}

func DefaultRoot() string {
	if root := strings.TrimSpace(os.Getenv("GOFLOW_FILE_STORE_DIR")); root != "" {
		return root
	}
	dbPath := strings.TrimSpace(os.Getenv("GOFLOW_DB_PATH"))
	if dbPath == "" {
		dbPath = "goflow.db"
	}
	dir := filepath.Dir(dbPath)
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, ".goflow-files")
}

func NewStore(root string) *Store {
	if strings.TrimSpace(root) == "" {
		root = DefaultRoot()
	}
	return &Store{Root: root, MaxBytes: DefaultMaxFileBytes}
}

func DefaultStore() *Store { return NewStore("") }

func (s *Store) ensureRoot() error {
	if s.MaxBytes <= 0 {
		s.MaxBytes = DefaultMaxFileBytes
	}
	return os.MkdirAll(s.Root, 0700)
}

func (s *Store) pathForID(id string) (string, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return "", fmt.Errorf("invalid FileRef id")
	}
	if err := s.ensureRoot(); err != nil {
		return "", err
	}
	return filepath.Join(s.Root, parsed.String()+".bin"), nil
}

func (s *Store) PutBytes(name, mimeType string, data []byte) (Ref, error) {
	if int64(len(data)) > s.maxBytes() {
		return Ref{}, fmt.Errorf("file exceeds %d byte limit", s.maxBytes())
	}
	if err := s.ensureRoot(); err != nil {
		return Ref{}, err
	}
	id := uuid.New().String()
	path, _ := s.pathForID(id)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return Ref{}, err
	}
	return makeRef(id, name, mimeType, data), nil
}

func (s *Store) PutReader(name, mimeType string, reader io.Reader) (Ref, error) {
	if err := s.ensureRoot(); err != nil {
		return Ref{}, err
	}
	id := uuid.New().String()
	path, _ := s.pathForID(id)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return Ref{}, err
	}
	h := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, h), io.LimitReader(reader, s.maxBytes()+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return Ref{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return Ref{}, closeErr
	}
	if written > s.maxBytes() {
		_ = os.Remove(path)
		return Ref{}, fmt.Errorf("file exceeds %d byte limit", s.maxBytes())
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(name))
	}
	return Ref{Type: TypeMarker, ID: id, Name: safeName(name), MIME: mimeType, Size: written, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func (s *Store) PutPath(path string) (Ref, error) {
	file, err := os.Open(path)
	if err != nil {
		return Ref{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Ref{}, err
	}
	if !info.Mode().IsRegular() {
		return Ref{}, fmt.Errorf("FileRef source must be a regular file")
	}
	if info.Size() > s.maxBytes() {
		return Ref{}, fmt.Errorf("file exceeds %d byte limit", s.maxBytes())
	}
	return s.PutReader(filepath.Base(path), mime.TypeByExtension(filepath.Ext(path)), file)
}

func (s *Store) Resolve(ref Ref) (string, error) {
	if ref.Type != TypeMarker {
		return "", fmt.Errorf("value is not a FileRef")
	}
	path, err := s.pathForID(ref.ID)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("FileRef %s is unavailable: %w", ref.ID, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("FileRef %s is not a regular file", ref.ID)
	}
	return path, nil
}

func (s *Store) Open(ref Ref) (*os.File, error) {
	path, err := s.Resolve(ref)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *Store) ReadAll(ref Ref) ([]byte, error) {
	file, err := s.Open(ref)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, s.maxBytes()+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > s.maxBytes() {
		return nil, fmt.Errorf("FileRef exceeds %d byte limit", s.maxBytes())
	}
	return data, nil
}

func Parse(value interface{}) (Ref, error) {
	if ref, ok := value.(Ref); ok {
		if ref.Type == TypeMarker && ref.ID != "" {
			return ref, nil
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return Ref{}, fmt.Errorf("invalid FileRef: %w", err)
	}
	var ref Ref
	if err := json.Unmarshal(encoded, &ref); err != nil || ref.Type != TypeMarker || strings.TrimSpace(ref.ID) == "" {
		return Ref{}, fmt.Errorf("invalid FileRef")
	}
	return ref, nil
}

func (s *Store) maxBytes() int64 {
	if s.MaxBytes <= 0 {
		return DefaultMaxFileBytes
	}
	return s.MaxBytes
}

func makeRef(id, name, mimeType string, data []byte) Ref {
	if mimeType == "" && len(data) > 0 {
		mimeType = http.DetectContentType(data)
	}
	sum := sha256.Sum256(data)
	return Ref{Type: TypeMarker, ID: id, Name: safeName(name), MIME: mimeType, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

func safeName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "file"
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}
