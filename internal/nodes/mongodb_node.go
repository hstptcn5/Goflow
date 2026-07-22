package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBCommandExecutor struct{}

func NewMongoDBCommandExecutor() *MongoDBCommandExecutor {
	return &MongoDBCommandExecutor{}
}

func (e *MongoDBCommandExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// 1. Resolve configuration
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
		connStr = "mongodb://localhost:27017"
	}

	dbName, _ := node.Params["database"].(string)
	collectionName, _ := node.Params["collection"].(string)
	command, _ := node.Params["command"].(string)
	command = strings.ToUpper(strings.TrimSpace(command))
	if command == "" {
		command = "FIND_ONE"
	}

	filterJSON, _ := node.Params["filter_json"].(string)
	docJSON, _ := node.Params["document_json"].(string)

	if dbName == "" || collectionName == "" {
		return nil, fmt.Errorf("database and collection parameters are required")
	}

	// 2. Setup MongoDB connection client
	clientOpts := options.Client().ApplyURI(connStr)
	goctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(goctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	// Ping database to verify connection
	if err := client.Ping(goctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb server: %w", err)
	}

	coll := client.Database(dbName).Collection(collectionName)

	// Parse JSON filter to bson.M
	var filter bson.M = bson.M{}
	if strings.TrimSpace(filterJSON) != "" {
		if err := json.Unmarshal([]byte(filterJSON), &filter); err != nil {
			return nil, fmt.Errorf("invalid filter JSON: %w", err)
		}
	}

	// Parse JSON document to bson.M
	var doc bson.M = bson.M{}
	if strings.TrimSpace(docJSON) != "" {
		if err := json.Unmarshal([]byte(docJSON), &doc); err != nil {
			return nil, fmt.Errorf("invalid document JSON: %w", err)
		}
	}

	// 3. Execute command
	switch command {
	case "FIND_ONE":
		var result bson.M
		err := coll.FindOne(goctx, filter).Decode(&result)
		if err == mongo.ErrNoDocuments {
			return nil, nil
		} else if err != nil {
			return nil, fmt.Errorf("FindOne failed: %w", err)
		}
		return result, nil

	case "INSERT_ONE":
		if len(doc) == 0 {
			return nil, fmt.Errorf("document JSON is required for INSERT_ONE")
		}
		res, err := coll.InsertOne(goctx, doc)
		if err != nil {
			return nil, fmt.Errorf("InsertOne failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "inserted_id": res.InsertedID}, nil

	case "UPDATE_ONE":
		if len(doc) == 0 {
			return nil, fmt.Errorf("document JSON (update document/modifiers) is required for UPDATE_ONE")
		}
		res, err := coll.UpdateOne(goctx, filter, doc)
		if err != nil {
			return nil, fmt.Errorf("UpdateOne failed: %w", err)
		}
		return map[string]interface{}{
			"status":         "success",
			"matched_count":  res.MatchedCount,
			"modified_count": res.ModifiedCount,
			"upserted_id":    res.UpsertedID,
		}, nil

	case "DELETE_ONE":
		res, err := coll.DeleteOne(goctx, filter)
		if err != nil {
			return nil, fmt.Errorf("DeleteOne failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "deleted_count": res.DeletedCount}, nil

	default:
		return nil, fmt.Errorf("unsupported MongoDB command: %s", command)
	}
}

func (e *MongoDBCommandExecutor) Validate(node *Node) error {
	dbName, _ := node.Params["database"].(string)
	collectionName, _ := node.Params["collection"].(string)
	if strings.TrimSpace(dbName) == "" || strings.TrimSpace(collectionName) == "" {
		return fmt.Errorf("database and collection parameters are required")
	}
	return nil
}

func (e *MongoDBCommandExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeMongoDBCommand,
		Name:        "MongoDB Command",
		Description: "Thực thi truy vấn hoặc thao tác dữ liệu trên cơ sở dữ liệu MongoDB",
		Icon:        "Database",
		Category:    "DATABASE",
		Params: []ParamDefinition{
			{
				Name:        "credential_id",
				Label:       "Select Encrypted Credential",
				Type:        "credential",
				Required:    false,
				Description: "Chọn cấu hình lưu trữ URI kết nối đã được mã hóa",
			},
			{
				Name:        "connection_string",
				Label:       "MongoDB Connection URI",
				Type:        "text",
				Default:     "mongodb://localhost:27017",
				Required:    false,
				Description: "Chuỗi URI kết nối trực tiếp (ví dụ: mongodb://user:pass@host:port)",
			},
			{
				Name:        "database",
				Label:       "Database Name",
				Type:        "text",
				Required:    true,
				Description: "Tên cơ sở dữ liệu",
			},
			{
				Name:        "collection",
				Label:       "Collection Name",
				Type:        "text",
				Required:    true,
				Description: "Tên collection cần thao tác",
			},
			{
				Name:        "command",
				Label:       "MongoDB Command",
				Type:        "select",
				Default:     "FIND_ONE",
				Options:     []string{"FIND_ONE", "INSERT_ONE", "UPDATE_ONE", "DELETE_ONE"},
				Required:    true,
				Description: "Chọn thao tác cần thực hiện",
			},
			{
				Name:        "filter_json",
				Label:       "Filter JSON Object",
				Type:        "textarea",
				Default:     "{\n  \"status\": \"active\"\n}",
				Required:    false,
				Description: "Đối tượng JSON filter truy vấn (cho FIND_ONE, UPDATE_ONE, DELETE_ONE)",
			},
			{
				Name:        "document_json",
				Label:       "Document / Update JSON",
				Type:        "textarea",
				Default:     "{\n  \"$set\": { \"status\": \"processed\" }\n}",
				Required:    false,
				Description: "Nội dung Document để thêm mới hoặc Update modifiers (ví dụ: {\"$set\": {...}})",
			},
		},
	}
}
