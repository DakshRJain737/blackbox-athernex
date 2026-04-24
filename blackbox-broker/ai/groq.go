package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/blackbox/broker/models"
)

const groqURL = "https://api.groq.com/openai/v1/chat/completions"

type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

func New(apiKey, model string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

// AnalysisResult is the structured response from the AI.
type AnalysisResult struct {
	RootCause  string  `json:"rootCause"`
	Suggestion string  `json:"suggestion"`
	Confidence float64 `json:"confidence"`
}

// AnalyzeCrash sends the last N readings to Groq and returns root cause + fix.
// recentData is a slice of sensor rows from the replay query.
func (c *Client) AnalyzeCrash(crash models.CrashEvent, recentData []map[string]interface{}) (*AnalysisResult, error) {
	if c.apiKey == "" || c.apiKey == "YOUR_GROQ_API_KEY_HERE" {
		return &AnalysisResult{
			RootCause:  "AI analysis unavailable (no API key configured)",
			Suggestion: "Set groqApiKey in config.json",
			Confidence: 0,
		}, nil
	}

	// Build a compact data summary
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("A crash event occurred at timestamp %d on topic %q.\n", crash.Timestamp, crash.Topic))
	sb.WriteString(fmt.Sprintf("Triggering value: %.4f, Threshold: %.4f, Severity: %s\n\n", crash.Value, crash.Threshold, crash.Severity))
	sb.WriteString("Last 30 seconds of sensor readings (timestamp, topic, value):\n")
	for _, row := range recentData {
		sb.WriteString(fmt.Sprintf("  %v | %v | %v\n", row["timestamp"], row["topic"], row["value"]))
	}

	prompt := sb.String() + `
Based on the above data, provide a JSON object with exactly these keys:
{
  "rootCause": "one-sentence explanation of what caused the crash",
  "suggestion": "one-sentence recommended fix or action",
  "confidence": 0.0 to 1.0
}
Respond with ONLY the JSON object, no other text.`

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a robotics sensor diagnostic AI. Be precise and technical."},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  300,
		"temperature": 0.2,
	})

	req, _ := http.NewRequest("POST", groqURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("groq status %d: %s", resp.StatusCode, string(body))
	}

	// Parse OpenAI-compatible response
	var groqResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &groqResp); err != nil {
		return nil, fmt.Errorf("parse groq response: %w", err)
	}
	if len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in groq response")
	}

	content := strings.TrimSpace(groqResp.Choices[0].Message.Content)
	// Strip markdown code fences if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result AnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse AI JSON: %w — raw: %s", err, content)
	}
	return &result, nil
}
