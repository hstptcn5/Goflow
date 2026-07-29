package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type pluginInput struct {
	Outputs map[string]any `json:"outputs"`
}

func main() {
	var input pluginInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		writeError(err.Error())
		return
	}

	trigger, _ := input.Outputs["$trigger"].(map[string]any)
	body, _ := trigger["body"].(map[string]any)

	email, _ := body["email"].(string)
	company, _ := body["company"].(string)
	message, _ := body["message"].(string)

	score := 10
	if strings.HasSuffix(strings.ToLower(email), ".edu") {
		score += 10
	}
	if company != "" {
		score += 25
	}
	if strings.Contains(strings.ToLower(message), "enterprise") {
		score += 40
	}
	if strings.Contains(strings.ToLower(message), "urgent") {
		score += 15
	}

	tier := "low"
	if score >= 70 {
		tier = "high"
	} else if score >= 40 {
		tier = "medium"
	}

	writeResult(map[string]any{
		"score": score,
		"tier":  tier,
		"email": email,
	})
}

func writeResult(result any) {
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"result": result,
	})
}

func writeError(message string) {
	_, _ = fmt.Fprintf(os.Stdout, `{"error":%q}`, message)
}

