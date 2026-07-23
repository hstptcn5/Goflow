package nodes

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLQueryExecutor struct{}

func NewMySQLQueryExecutor() *MySQLQueryExecutor {
	return &MySQLQueryExecutor{}
}

func (e *MySQLQueryExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// 1. Resolve connection string
	connStr, _ := node.Params["connection_string"].(string)
	credID, _ := node.Params["credential_id"].(string)
	if credID != "" {
		ctx.mu.RLock()
		decrypted, ok := ctx.Credentials[credID]
		ctx.mu.RUnlock()
		if ok && decrypted != "" {
			connStr = decrypted
		}
	}

	if strings.TrimSpace(connStr) == "" {
		return nil, fmt.Errorf("connection_string is empty (either set it directly or select a valid credential)")
	}

	query, _ := node.Params["query"].(string)
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("SQL query is empty")
	}

	queryType, _ := node.Params["query_type"].(string)
	if queryType == "" {
		queryType = "SELECT"
	}

	// 2. Open connection
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}
	defer db.Close()

	// Ping database to verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql database: %w", err)
	}

	// 3. Execute query
	if strings.ToUpper(queryType) == "SELECT" {
		rows, err := db.Query(query)
		if err != nil {
			return nil, fmt.Errorf("SQL query execution failed: %w", err)
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return nil, fmt.Errorf("failed to get result columns: %w", err)
		}

		var resultList []map[string]interface{}

		for rows.Next() {
			rowValues := make([]interface{}, len(columns))
			rowValPointers := make([]interface{}, len(columns))
			for i := range rowValues {
				rowValPointers[i] = &rowValues[i]
			}

			if err := rows.Scan(rowValPointers...); err != nil {
				return nil, fmt.Errorf("failed to scan row: %w", err)
			}

			rowMap := make(map[string]interface{})
			for i, colName := range columns {
				val := rowValues[i]
				switch v := val.(type) {
				case []byte:
					// Convert string/varchar returned as bytes back to string
					rowMap[colName] = string(v)
				default:
					rowMap[colName] = v
				}
			}
			resultList = append(resultList, rowMap)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error during row iteration: %w", err)
		}

		if resultList == nil {
			resultList = []map[string]interface{}{}
		}

		return resultList, nil
	} else {
		// EXECUTE mode (INSERT/UPDATE/DELETE/CREATE/DROP/etc)
		res, err := db.Exec(query)
		if err != nil {
			return nil, fmt.Errorf("SQL statement execution failed: %w", err)
		}

		rowsAffected, _ := res.RowsAffected()
		return map[string]interface{}{
			"status":        "success",
			"rows_affected": rowsAffected,
		}, nil
	}
}

func (e *MySQLQueryExecutor) Validate(node *Node) error {
	return nil
}

func (e *MySQLQueryExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeMySQLQuery,
		Name:        "MySQL Query",
		Description: "Thực thi câu lệnh SQL SELECT/EXECUTE trên cơ sở dữ liệu MySQL",
		Icon:        "Database",
		Category:    "DATABASE",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "credential_id",
				Label:       "Select Encrypted Credential",
				Type:        "credential",
				Required:    false,
				Description: "Chọn cấu hình lưu trữ chuỗi kết nối đã được mã hóa",
			},
			{
				Name:        "connection_string",
				Label:       "MySQL Connection String",
				Type:        "text",
				Default:     "root:password@tcp(localhost:3306)/mysql",
				Required:    false,
				Description: "Chuỗi kết nối trực tiếp (ví dụ: user:pass@tcp(host:port)/db)",
			},
			{
				Name:        "query_type",
				Label:       "Query Type",
				Type:        "select",
				Default:     "SELECT",
				Options:     []string{"SELECT", "EXECUTE"},
				Required:    true,
				Description: "SELECT trả về danh sách dữ liệu, EXECUTE trả về số dòng bị tác động",
			},
			{
				Name:        "query",
				Label:       "SQL Statement",
				Type:        "textarea",
				Default:     "SELECT * FROM users LIMIT 5;",
				Required:    true,
				Description: "Viết câu lệnh SQL của bạn ở đây, hỗ trợ chèn biến động {{node.path}}",
			},
		},
	}
}
