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

const maxMongoJSONBytes = 1 << 20

type MongoDBCommandExecutor struct{}

func NewMongoDBCommandExecutor() *MongoDBCommandExecutor { return &MongoDBCommandExecutor{} }

func mongoCommandName(node *Node) (string, error) {
	command, _ := node.Params["command"].(string)
	command = strings.ToUpper(strings.TrimSpace(command))
	if command == "" {
		command = "FIND_ONE"
	}
	switch command {
	case "FIND_ONE", "INSERT_ONE", "UPDATE_ONE", "DELETE_ONE":
		return command, nil
	default:
		return "", fmt.Errorf("unsupported MongoDB command: %s", command)
	}
}

func validateMongoIdentity(node *Node) error {
	dbName, _ := node.Params["database"].(string)
	collectionName, _ := node.Params["collection"].(string)
	if strings.TrimSpace(dbName) == "" || strings.TrimSpace(collectionName) == "" {
		return fmt.Errorf("database and collection parameters are required")
	}
	if len(dbName) > 255 || len(collectionName) > 255 || strings.ContainsAny(dbName+collectionName, "\r\n\x00") {
		return fmt.Errorf("database and collection names are invalid or too long")
	}
	_, err := mongoCommandName(node)
	return err
}

func parseMongoJSONParam(raw interface{}, name string) (bson.M, error) {
	if raw == nil {
		return bson.M{}, nil
	}
	if structured, ok := raw.(map[string]interface{}); ok {
		encoded, err := json.Marshal(structured)
		if err != nil {
			return nil, fmt.Errorf("MongoDB %s could not be encoded: %w", name, err)
		}
		if len(encoded) > maxMongoJSONBytes {
			return nil, fmt.Errorf("MongoDB %s exceeds %d byte limit", name, maxMongoJSONBytes)
		}
		var parsed bson.M
		if err := json.Unmarshal(encoded, &parsed); err != nil {
			return nil, fmt.Errorf("invalid %s: %w", name, err)
		}
		return parsed, nil
	}
	text, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("MongoDB %s must resolve to a JSON object", name)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return bson.M{}, nil
	}
	if len(text) > maxMongoJSONBytes {
		return nil, fmt.Errorf("MongoDB %s exceeds %d byte limit", name, maxMongoJSONBytes)
	}
	var parsed bson.M
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", name, err)
	}
	return parsed, nil
}

func parseMongoCommandNode(node *Node) (command string, filter, document bson.M, err error) {
	if err := validateMongoIdentity(node); err != nil {
		return "", nil, nil, err
	}
	command, _ = mongoCommandName(node)
	filter, err = parseMongoJSONParam(node.Params["filter_json"], "filter_json")
	if err != nil {
		return "", nil, nil, err
	}
	document, err = parseMongoJSONParam(node.Params["document_json"], "document_json")
	if err != nil {
		return "", nil, nil, err
	}
	if (command == "INSERT_ONE" || command == "UPDATE_ONE") && len(document) == 0 {
		return "", nil, nil, fmt.Errorf("document_json is required for %s", command)
	}
	return command, filter, document, nil
}

func (e *MongoDBCommandExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	command, filter, doc, err := parseMongoCommandNode(node)
	if err != nil {
		return nil, err
	}
	connStr, _ := node.Params["connection_string"].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	if strings.TrimSpace(credentialID) != "" {
		connStr, err = resolveNodeCredential(ctx, node, "connection_string", "MongoDB connection URI")
		if err != nil {
			return nil, err
		}
	} else if strings.TrimSpace(connStr) == "" {
		connStr = "mongodb://localhost:27017"
	}
	if !strings.HasPrefix(connStr, "mongodb://") && !strings.HasPrefix(connStr, "mongodb+srv://") {
		return nil, fmt.Errorf("MongoDB connection URI must start with mongodb:// or mongodb+srv://")
	}
	dbName, _ := node.Params["database"].(string)
	collectionName, _ := node.Params["collection"].(string)

	runCtx, cancel := context.WithTimeout(ctx.Context, 15*time.Second)
	defer cancel()
	client, err := mongo.Connect(runCtx, options.Client().ApplyURI(connStr))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}
	defer func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx)
	}()
	if err := client.Ping(runCtx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb server: %w", err)
	}
	coll := client.Database(strings.TrimSpace(dbName)).Collection(strings.TrimSpace(collectionName))
	switch command {
	case "FIND_ONE":
		var result bson.M
		err := coll.FindOne(runCtx, filter).Decode(&result)
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("FindOne failed: %w", err)
		}
		return result, nil
	case "INSERT_ONE":
		res, err := coll.InsertOne(runCtx, doc)
		if err != nil {
			return nil, fmt.Errorf("InsertOne failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "inserted_id": res.InsertedID}, nil
	case "UPDATE_ONE":
		res, err := coll.UpdateOne(runCtx, filter, doc)
		if err != nil {
			return nil, fmt.Errorf("UpdateOne failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "matched_count": res.MatchedCount, "modified_count": res.ModifiedCount, "upserted_id": res.UpsertedID}, nil
	case "DELETE_ONE":
		res, err := coll.DeleteOne(runCtx, filter)
		if err != nil {
			return nil, fmt.Errorf("DeleteOne failed: %w", err)
		}
		return map[string]interface{}{"status": "success", "deleted_count": res.DeletedCount}, nil
	default:
		return nil, fmt.Errorf("unsupported MongoDB command: %s", command)
	}
}

func (e *MongoDBCommandExecutor) Validate(node *Node) error {
	if err := validateMongoIdentity(node); err != nil {
		return err
	}
	command, _ := mongoCommandName(node)
	for _, name := range []string{"filter_json", "document_json"} {
		raw := node.Params[name]
		if text, ok := raw.(string); ok && containsTemplateExpression(text) {
			if name == "document_json" && (command == "INSERT_ONE" || command == "UPDATE_ONE") && strings.TrimSpace(text) == "" {
				return fmt.Errorf("document_json is required for %s", command)
			}
			continue
		}
		parsed, err := parseMongoJSONParam(raw, name)
		if err != nil {
			return err
		}
		if name == "document_json" && (command == "INSERT_ONE" || command == "UPDATE_ONE") && len(parsed) == 0 {
			return fmt.Errorf("document_json is required for %s", command)
		}
	}
	return nil
}

func (e *MongoDBCommandExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeMongoDBCommand, Name: "MongoDB Command", Description: "Runs bounded queries or data operations against MongoDB", Icon: "Database", Category: "DATABASE", Retryable: true,
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted MongoDB connection URI"},
			{Name: "connection_string", Label: "MongoDB Connection URI (legacy)", Type: "password", Default: "mongodb://localhost:27017", Required: false, Description: "Legacy direct connection URI. Prefer an encrypted credential."},
			{Name: "database", Label: "Database Name", Type: "text", Required: true, Description: "Database name"},
			{Name: "collection", Label: "Collection Name", Type: "text", Required: true, Description: "Collection name to operate on"},
			{Name: "command", Label: "MongoDB Command", Type: "select", Default: "FIND_ONE", Options: []string{"FIND_ONE", "INSERT_ONE", "UPDATE_ONE", "DELETE_ONE"}, Required: true, Description: "Choose the MongoDB operation to run"},
			{Name: "filter_json", Label: "Filter JSON Object", Type: "textarea", Default: "{}", Required: false, Description: "JSON filter object for FIND_ONE, UPDATE_ONE, or DELETE_ONE"},
			{Name: "document_json", Label: "Document / Update JSON", Type: "textarea", Default: "", Required: false, Description: "Document to insert, or update modifiers for UPDATE_ONE"},
		},
	}
}
