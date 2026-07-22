package storage

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Workflow struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	NodesJSON   string    `json:"nodes_json"`
	EdgesJSON   string    `json:"edges_json"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkflowStore struct {
	db *DB
}

func NewWorkflowStore(db *DB) *WorkflowStore {
	return &WorkflowStore{db: db}
}

func (s *WorkflowStore) Create(wf *Workflow) error {
	if wf.ID == "" {
		wf.ID = uuid.New().String()
	}
	now := time.Now()
	wf.CreatedAt = now
	wf.UpdatedAt = now

	query := `
		INSERT INTO workflows (id, name, description, is_active, nodes_json, edges_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	isActiveInt := 0
	if wf.IsActive {
		isActiveInt = 1
	}

	_, err := s.db.WriteDB.Exec(query, wf.ID, wf.Name, wf.Description, isActiveInt, wf.NodesJSON, wf.EdgesJSON, wf.CreatedAt, wf.UpdatedAt)
	return err
}

func (s *WorkflowStore) Update(wf *Workflow) error {
	wf.UpdatedAt = time.Now()
	query := `
		UPDATE workflows
		SET name = ?, description = ?, is_active = ?, nodes_json = ?, edges_json = ?, updated_at = ?
		WHERE id = ?
	`
	isActiveInt := 0
	if wf.IsActive {
		isActiveInt = 1
	}

	res, err := s.db.WriteDB.Exec(query, wf.Name, wf.Description, isActiveInt, wf.NodesJSON, wf.EdgesJSON, wf.UpdatedAt, wf.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("workflow not found")
	}
	return nil
}

func (s *WorkflowStore) ToggleActive(id string, isActive bool) error {
	query := `UPDATE workflows SET is_active = ?, updated_at = ? WHERE id = ?`
	isActiveInt := 0
	if isActive {
		isActiveInt = 1
	}
	_, err := s.db.WriteDB.Exec(query, isActiveInt, time.Now(), id)
	return err
}

func (s *WorkflowStore) GetByID(id string) (*Workflow, error) {
	query := `
		SELECT id, name, description, is_active, nodes_json, edges_json, created_at, updated_at
		FROM workflows WHERE id = ?
	`
	row := s.db.ReadDB.QueryRow(query, id)

	var wf Workflow
	var isActiveInt int
	err := row.Scan(&wf.ID, &wf.Name, &wf.Description, &isActiveInt, &wf.NodesJSON, &wf.EdgesJSON, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("workflow not found")
		}
		return nil, err
	}
	wf.IsActive = isActiveInt == 1
	return &wf, nil
}

func (s *WorkflowStore) ListAll() ([]Workflow, error) {
	query := `
		SELECT id, name, description, is_active, nodes_json, edges_json, created_at, updated_at
		FROM workflows ORDER BY updated_at DESC
	`
	rows, err := s.db.ReadDB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Workflow
	for rows.Next() {
		var wf Workflow
		var isActiveInt int
		if err := rows.Scan(&wf.ID, &wf.Name, &wf.Description, &isActiveInt, &wf.NodesJSON, &wf.EdgesJSON, &wf.CreatedAt, &wf.UpdatedAt); err != nil {
			return nil, err
		}
		wf.IsActive = isActiveInt == 1
		result = append(result, wf)
	}
	return result, nil
}

func (s *WorkflowStore) Delete(id string) error {
	query := `DELETE FROM workflows WHERE id = ?`
	_, err := s.db.WriteDB.Exec(query, id)
	return err
}
