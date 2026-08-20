package nodes

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const (
	maxSQLQueryBytes = 1 << 20
	maxSQLRows       = 5000
	maxSQLColumns    = 256
	maxSQLRunTime    = 30 * time.Second
)

type PostgresQueryExecutor struct{}

func NewPostgresQueryExecutor() *PostgresQueryExecutor { return &PostgresQueryExecutor{} }

func parseSQLNode(node *Node, engine string) (query, queryType string, err error) {
	query, _ = node.Params["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return "", "", fmt.Errorf("SQL query is empty")
	}
	if len(query) > maxSQLQueryBytes {
		return "", "", fmt.Errorf("SQL query exceeds %d byte limit", maxSQLQueryBytes)
	}
	queryType, _ = node.Params["query_type"].(string)
	queryType = strings.ToUpper(strings.TrimSpace(queryType))
	if queryType == "" {
		queryType = "SELECT"
	}
	if queryType != "SELECT" && queryType != "EXECUTE" {
		return "", "", fmt.Errorf("%s query_type must be SELECT or EXECUTE", engine)
	}
	connectionString, _ := node.Params["connection_string"].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	if strings.TrimSpace(connectionString) == "" && strings.TrimSpace(credentialID) == "" {
		return "", "", fmt.Errorf("%s connection string or encrypted credential is required", engine)
	}
	return query, queryType, nil
}

func boundedSQLContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, maxSQLRunTime)
}

func scanSQLRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get result columns: %w", err)
	}
	if len(columns) > maxSQLColumns {
		return nil, fmt.Errorf("SQL result has %d columns; maximum is %d", len(columns), maxSQLColumns)
	}
	resultList := make([]map[string]interface{}, 0)
	for rows.Next() {
		if len(resultList) >= maxSQLRows {
			return nil, fmt.Errorf("SQL result exceeds %d row limit", maxSQLRows)
		}
		rowValues := make([]interface{}, len(columns))
		rowPointers := make([]interface{}, len(columns))
		for i := range rowValues {
			rowPointers[i] = &rowValues[i]
		}
		if err := rows.Scan(rowPointers...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		rowMap := make(map[string]interface{}, len(columns))
		for i, column := range columns {
			if value, ok := rowValues[i].([]byte); ok {
				rowMap[column] = string(value)
			} else {
				rowMap[column] = rowValues[i]
			}
		}
		resultList = append(resultList, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}
	return resultList, nil
}

func (e *PostgresQueryExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	query, queryType, err := parseSQLNode(node, "PostgreSQL")
	if err != nil {
		return nil, err
	}
	connStr, err := resolveNodeCredential(ctx, node, "connection_string", "PostgreSQL connection string")
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}
	defer db.Close()
	runCtx, cancel := boundedSQLContext(ctx.Context)
	defer cancel()
	if err := db.PingContext(runCtx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}
	if queryType == "SELECT" {
		rows, err := db.QueryContext(runCtx, query)
		if err != nil {
			return nil, fmt.Errorf("SQL query execution failed: %w", err)
		}
		defer rows.Close()
		return scanSQLRows(rows)
	}
	res, err := db.ExecContext(runCtx, query)
	if err != nil {
		return nil, fmt.Errorf("SQL statement execution failed: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	return map[string]interface{}{"status": "success", "rows_affected": rowsAffected}, nil
}

func (e *PostgresQueryExecutor) Validate(node *Node) error {
	_, _, err := parseSQLNode(node, "PostgreSQL")
	return err
}

func (e *PostgresQueryExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypePostgresQuery, Name: "PostgreSQL Query", Description: "Runs bounded SELECT or EXECUTE SQL statements against PostgreSQL", Icon: "Database", Category: "DATABASE", Retryable: true,
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted PostgreSQL connection string"},
			{Name: "connection_string", Label: "Postgres Connection String (legacy)", Type: "password", Default: "", Required: false, Description: "Legacy direct connection string. Prefer an encrypted credential."},
			{Name: "query_type", Label: "Query Type", Type: "select", Default: "SELECT", Options: []string{"SELECT", "EXECUTE"}, Required: true, Description: "SELECT returns up to 5,000 rows; EXECUTE returns affected row count"},
			{Name: "query", Label: "SQL Statement", Type: "textarea", Default: "SELECT 1;", Required: true, Description: "SQL statement. Supports placeholders such as {{node.path}}"},
		},
	}
}
