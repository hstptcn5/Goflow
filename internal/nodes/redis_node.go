package nodes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type RedisCommandExecutor struct{}

func NewRedisCommandExecutor() *RedisCommandExecutor {
	return &RedisCommandExecutor{}
}

func (e *RedisCommandExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	// 1. Resolve configuration
	addr, _ := node.Params["address"].(string)
	password, _ := node.Params["password"].(string)
	dbStr, _ := node.Params["db"].(string)
	credID, _ := node.Params["credential_id"].(string)

	if credID != "" {
		ctx.mu.RLock()
		decrypted, ok := ctx.Credentials[credID]
		ctx.mu.RUnlock()
		if ok && decrypted != "" {
			// Decrypted credential can contain a connection string or JSON
			// To keep it simple, we treat it as the password or connection info
			// If it has a colon, treat it as host:port, otherwise as password
			if strings.Contains(decrypted, ":") {
				addr = decrypted
			} else {
				password = decrypted
			}
		}
	}

	if strings.TrimSpace(addr) == "" {
		addr = "localhost:6379"
	}

	dbIndex := 0
	if dbStr != "" {
		if idx, err := strconv.Atoi(dbStr); err == nil {
			dbIndex = idx
		}
	}

	command, _ := node.Params["command"].(string)
	command = strings.ToUpper(strings.TrimSpace(command))
	if command == "" {
		command = "GET"
	}

	key, _ := node.Params["key"].(string)
	value, _ := node.Params["value"].(string)
	field, _ := node.Params["field"].(string)

	if key == "" {
		return nil, fmt.Errorf("Redis Key is required")
	}

	// 2. Open connection
	goctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       dbIndex,
	})
	defer rdb.Close()

	// Ping connection
	if err := rdb.Ping(goctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis server at %s: %w", addr, err)
	}

	// 3. Execute command
	switch command {
	case "GET":
		val, err := rdb.Get(goctx, key).Result()
		if err == redis.Nil {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		return val, nil

	case "SET":
		val, err := rdb.Set(goctx, key, value, 0).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "result": val}, nil

	case "DEL":
		val, err := rdb.Del(goctx, key).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "deleted_keys": val}, nil

	case "EXISTS":
		val, err := rdb.Exists(goctx, key).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"exists": val > 0}, nil

	case "LPUSH":
		val, err := rdb.LPush(goctx, key, value).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "list_length": val}, nil

	case "RPUSH":
		val, err := rdb.RPush(goctx, key, value).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "list_length": val}, nil

	case "HSET":
		if field == "" {
			return nil, fmt.Errorf("Redis Field is required for HSET command")
		}
		val, err := rdb.HSet(goctx, key, field, value).Result()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"status": "success", "fields_added": val}, nil

	case "HGET":
		if field == "" {
			return nil, fmt.Errorf("Redis Field is required for HGET command")
		}
		val, err := rdb.HGet(goctx, key, field).Result()
		if err == redis.Nil {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		return val, nil

	default:
		return nil, fmt.Errorf("unsupported Redis command: %s", command)
	}
}

func (e *RedisCommandExecutor) Validate(node *Node) error {
	key, _ := node.Params["key"].(string)
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("Redis key is required")
	}
	return nil
}

func (e *RedisCommandExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type:        TypeRedisCommand,
		Name:        "Redis Command",
		Description: "Tương tác với cơ sở dữ liệu Redis (GET, SET, DEL, HSET, v.v.)",
		Icon:        "Database",
		Category:    "DATABASE",
		Retryable:   true,
		Params: []ParamDefinition{
			{
				Name:        "credential_id",
				Label:       "Select Encrypted Credential",
				Type:        "credential",
				Required:    false,
				Description: "Chọn cấu hình thông tin bảo mật chứa mật khẩu kết nối đã mã hóa",
			},
			{
				Name:        "address",
				Label:       "Redis Address",
				Type:        "text",
				Default:     "localhost:6379",
				Required:    true,
				Description: "Địa chỉ Redis server (ví dụ: localhost:6379)",
			},
			{
				Name:        "password",
				Label:       "Redis Password",
				Type:        "password",
				Required:    false,
				Description: "Mật khẩu đăng nhập Redis server (nếu có)",
			},
			{
				Name:        "db",
				Label:       "Redis DB Index",
				Type:        "text",
				Default:     "0",
				Required:    false,
				Description: "Chỉ mục cơ sở dữ liệu Redis (thường là 0)",
			},
			{
				Name:        "command",
				Label:       "Redis Command",
				Type:        "select",
				Default:     "GET",
				Options:     []string{"GET", "SET", "DEL", "EXISTS", "LPUSH", "RPUSH", "HSET", "HGET"},
				Required:    true,
				Description: "Chọn câu lệnh Redis để thực thi",
			},
			{
				Name:        "key",
				Label:       "Redis Key",
				Type:        "text",
				Required:    true,
				Description: "Tên Key cần thao tác",
			},
			{
				Name:        "field",
				Label:       "Redis Field (Only for HGET/HSET)",
				Type:        "text",
				Required:    false,
				Description: "Tên Field bên trong Hash Key",
			},
			{
				Name:        "value",
				Label:       "Value to Write (For SET, HSET, LPUSH, etc.)",
				Type:        "textarea",
				Required:    false,
				Description: "Giá trị cần ghi đè hoặc thêm vào database",
			},
		},
	}
}
