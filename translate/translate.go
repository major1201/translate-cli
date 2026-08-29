package translate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LLMConfig holds configuration for the LLM translator.
type LLMConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// DefaultConfig reads configuration from environment variables.
func DefaultConfig() *LLMConfig {
	apiKey := os.Getenv("TX_LLM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	baseURL := os.Getenv("TX_LLM_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("OPENAI_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	model := os.Getenv("TX_LLM_MODEL")
	if model == "" {
		model = os.Getenv("OPENAI_MODEL")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &LLMConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
	}
}

// Translator translates text using an LLM.
type Translator struct {
	config *LLMConfig
	client *http.Client
}

// New creates a new Translator with the given config.
func New(cfg *LLMConfig) *Translator {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Translator{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Translate translates the given text from source language to target language.
// The translation is streamed to out as chunks arrive, and the full text is
// also returned.
func (t *Translator) Translate(text, sourceLang, targetLang string, out io.Writer) (string, error) {
	if t.config.APIKey == "" {
		return "", fmt.Errorf("API key not set. Set TX_LLM_API_KEY or OPENAI_API_KEY environment variable")
	}

	prompt := fmt.Sprintf(
		"Translate the following text from %s to %s. Return ONLY the translation, nothing else.\n\nText: %s",
		langName(sourceLang), langName(targetLang), text,
	)

	reqBody := map[string]any{
		"model": t.config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.3,
		"stream":      true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := t.config.BaseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.config.APIKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read error response: %w", err)
		}
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBytes))
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		const prefix = "data:"
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		d := chunk.Choices[0].Delta.Content
		sb.WriteString(d)
		if out != nil {
			if _, err := io.WriteString(out, d); err != nil {
				return sb.String(), fmt.Errorf("failed to write output: %w", err)
			}
			if f, ok := out.(interface{ Flush() }); ok {
				f.Flush()
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return sb.String(), fmt.Errorf("failed to read stream: %w", err)
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("no translation returned")
	}
	return sb.String(), nil
}

func langName(code string) string {
	switch code {
	case "zh", "zh-CN", "zh-cn":
		return "Chinese"
	case "en", "en-US", "en-us":
		return "English"
	case "ja", "ja-JP", "ja-jp":
		return "Japanese"
	case "ko", "ko-KR", "ko-kr":
		return "Korean"
	case "fr", "fr-FR", "fr-fr":
		return "French"
	case "de", "de-DE", "de-de":
		return "German"
	case "es", "es-ES", "es-es":
		return "Spanish"
	case "ru", "ru-RU", "ru-ru":
		return "Russian"
	case "pt", "pt-PT", "pt-pt":
		return "Portuguese"
	case "ar", "ar-SA", "ar-sa":
		return "Arabic"
	case "th", "th-TH", "th-th":
		return "Thai"
	case "vi", "vi-VN", "vi-vn":
		return "Vietnamese"
	default:
		return code
	}
}
