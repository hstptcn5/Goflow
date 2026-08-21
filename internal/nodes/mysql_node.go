package nodes

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLQueryExecutor struct{}

func NewMySQLQueryExecutor() *MySQLQueryExecutor { return &MySQLQueryExecutor{} }

func (e *MySQLQueryExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	query, queryType, err := parseSQLNode(node, "MySQL")
	if err != nil {
		return nil, err
	}
	parameters, err := parseSQLParameters(node.Params["parameters"])
	if err != nil {
		return nil, err
	}
	connStr, err := resolveNodeCredential(ctx, node, "connection_string", "MySQL connection string")
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}
	defer db.Close()
	runCtx, cancel := boundedSQLContext(ctx.Context)
	defer cancel()
	if err := db.PingContext(runCtx); err != nil {
		return nil, fmt.Errorf("failed to ping mysql database: %w", err)
	}
	if queryType == "SELECT" {
		rows, err := db.QueryContext(runCtx, query, parameters...)
		if err != nil {
			return nil, fmt.Errorf("SQL query execution failed: %w", err)
		}
		defer rows.Close()
		return scanSQLRows(rows)
	}
	res, err := db.ExecContext(runCtx, query, parameters...)
	if err != nil {
		return nil, fmt.Errorf("SQL statement execution failed: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	return map[string]interface{}{"status": "success", "rows_affected": rowsAffected}, nil
}

func (e *MySQLQueryExecutor) Validate(node *Node) error {
	_, _, err := parseSQLNode(node, "MySQL")
	return err
}

func (e *MySQLQueryExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeMySQLQuery, Name: "MySQL Query", Description: "Runs bounded SELECT or EXECUTE SQL statements against MySQL", Icon: "Database", Category: "DATABASE", Retryable: true,
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted MySQL connection string"},
			{Name: "connection_string", Label: "MySQL Connection String (legacy)", Type: "password", Default: "", Required: false, Description: "Legacy direct connection string. Prefer an encrypted credential."},
			{Name: "query_type", Label: "Query Type", Type: "select", Default: "SELECT", Options: []string{"SELECT", "EXECUTE"}, Required: true, Description: "SELECT returns up to 5,000 rows; EXECUTE returns affected row count"},
			{Name: "query", Label: "SQL Statement", Type: "textarea", Default: "SELECT 1;", Required: true, Description: "SQL statement. Prefer ? placeholders for user-controlled values."},
			{Name: "parameters", Label: "Parameters", Type: "json", Default: "[]", Required: false, Description: "JSON array or mapped array bound separately to SQL placeholders; values are never concatenated into the query by this field."},
		},
	}
}
