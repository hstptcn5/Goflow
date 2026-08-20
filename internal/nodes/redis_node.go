package nodes

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const maxRedisValueBytes = 1 << 20

type RedisCommandExecutor struct{}

func NewRedisCommandExecutor() *RedisCommandExecutor { return &RedisCommandExecutor{} }

func parseRedisDB(raw interface{}) (int, error) {
	if raw == nil || strings.TrimSpace(fmt.Sprint(raw)) == "" {
		return 0, nil
	}
	var value int
	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("Redis DB index must be an integer")
		}
		value = int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("Redis DB index must be an integer")
		}
		value = parsed
	default:
		return 0, fmt.Errorf("Redis DB index must be an integer")
	}
	if value < 0 || value > 1024 {
		return 0, fmt.Errorf("Redis DB index must be between 0 and 1024")
	}
	return value, nil
}

func parseRedisNode(node *Node) (command, key, field, value string, db int, err error) {
	command, _ = node.Params["command"].(string)
	command = strings.ToUpper(strings.TrimSpace(command))
	if command == "" {
		command = "GET"
	}
	switch command {
	case "GET", "SET", "DEL", "EXISTS", "LPUSH", "RPUSH", "HSET", "HGET":
	default:
		return "", "", "", "", 0, fmt.Errorf("unsupported Redis command: %s", command)
	}
	key, _ = node.Params["key"].(string)
	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", "", "", 0, fmt.Errorf("Redis key is required")
	}
	if len(key) > 4096 {
		return "", "", "", "", 0, fmt.Errorf("Redis key exceeds 4096 byte limit")
	}
	field, _ = node.Params["field"].(string)
	value, _ = node.Params["value"].(string)
	if (command == "HGET" || command == "HSET") && strings.TrimSpace(field) == "" {
		return "", "", "", "", 0, fmt.Errorf("Redis field is required for %s", command)
	}
	if (command == "SET" || command == "LPUSH" || command == "RPUSH" || command == "HSET") && len(value) > maxRedisValueBytes {
		return "", "", "", "", 0, fmt.Errorf("Redis value exceeds %d byte limit", maxRedisValueBytes)
	}
	db, err = parseRedisDB(node.Params["db"])
	return
}

func validateRedisAddress(address string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("Redis address is required")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("Redis address must be host:port")
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return fmt.Errorf("Redis port must be between 1 and 65535")
	}
	return nil
}

func (e *RedisCommandExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	command, key, field, value, dbIndex, err := parseRedisNode(node)
	if err != nil {
		return nil, err
	}
	addr, _ := node.Params["address"].(string)
	if strings.TrimSpace(addr) == "" {
		addr = "localhost:6379"
	}
	if err := validateRedisAddress(addr); err != nil {
		return nil, err
	}
	password, _ := node.Params["password"].(string)
	credentialID, _ := node.Params["credential_id"].(string)
	if strings.TrimSpace(credentialID) != "" {
		password, err = resolveNodeCredential(ctx, node, "password", "Redis password")
		if err != nil {
			return nil, err
		}
	}

	runCtx, cancel := context.WithTimeout(ctx.Context, 15*time.Second)
	defer cancel()
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: dbIndex, DialTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	defer rdb.Close()
	if err := rdb.Ping(runCtx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis server: %w", err)
	}
	switch command {
	case "GET":
		val, err := rdb.Get(runCtx, key).Result()
		if err == redis.Nil {
			return nil, nil
		}
		return val, err
	case "SET":
		val, err := rdb.Set(runCtx, key, value, 0).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "result": val}, nil
	case "DEL":
		val, err := rdb.Del(runCtx, key).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "deleted_keys": val}, nil
	case "EXISTS":
		val, err := rdb.Exists(runCtx, key).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"exists": val > 0}, nil
	case "LPUSH":
		val, err := rdb.LPush(runCtx, key, value).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "list_length": val}, nil
	case "RPUSH":
		val, err := rdb.RPush(runCtx, key, value).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "list_length": val}, nil
	case "HSET":
		val, err := rdb.HSet(runCtx, key, field, value).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "fields_added": val}, nil
	case "HGET":
		val, err := rdb.HGet(runCtx, key, field).Result()
		if err == redis.Nil {
			return nil, nil
		}
		return val, err
	default:
		return nil, fmt.Errorf("unsupported Redis command: %s", command)
	}
}

func (e *RedisCommandExecutor) Validate(node *Node) error {
	if _, _, _, _, _, err := parseRedisNode(node); err != nil {
		return err
	}
	address, _ := node.Params["address"].(string)
	if strings.TrimSpace(address) == "" {
		address = "localhost:6379"
	}
	if containsTemplateExpression(address) {
		return nil
	}
	return validateRedisAddress(address)
}

func (e *RedisCommandExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeRedisCommand, Name: "Redis Command", Description: "Runs bounded Redis commands such as GET, SET, DEL, HGET, and HSET", Icon: "Database", Category: "DATABASE", Retryable: true,
		Params: []ParamDefinition{
			{Name: "credential_id", Label: "Select Encrypted Credential", Type: "credential", Required: false, Description: "Encrypted Redis password"},
			{Name: "address", Label: "Redis Address", Type: "text", Default: "localhost:6379", Required: true, Description: "Redis server address as host:port"},
			{Name: "password", Label: "Redis Password (legacy)", Type: "password", Required: false, Description: "Legacy direct Redis password. Prefer an encrypted credential."},
			{Name: "db", Label: "Redis DB Index", Type: "text", Default: "0", Required: false, Description: "Redis database index, between 0 and 1024"},
			{Name: "command", Label: "Redis Command", Type: "select", Default: "GET", Options: []string{"GET", "SET", "DEL", "EXISTS", "LPUSH", "RPUSH", "HSET", "HGET"}, Required: true, Description: "Choose the Redis command to run"},
			{Name: "key", Label: "Redis Key", Type: "text", Required: true, Description: "Redis key to operate on"},
			{Name: "field", Label: "Redis Field (Only for HGET/HSET)", Type: "text", Required: false, Description: "Field name inside a Redis hash key"},
			{Name: "value", Label: "Value to Write", Type: "textarea", Required: false, Description: "Value to write or append"},
		},
	}
}
