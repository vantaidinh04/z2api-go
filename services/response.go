package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/Tyler-Dinh/z2api-go/config"
	"github.com/Tyler-Dinh/z2api-go/types"
	"github.com/Tyler-Dinh/z2api-go/utils"
)

var (
	phaseBak = "thinking"
)

// ParseSSEStream parses Server-Sent Events stream from Z.ai API
func ParseSSEStream(resp *http.Response) <-chan *types.ZaiResponse {
	ch := make(chan *types.ZaiResponse)

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Bytes()

			// Skip empty lines or non-data lines
			if len(line) == 0 || !strings.HasPrefix(string(line), "data: ") {
				continue
			}

			// Extract JSON data
			jsonData := line[6:] // Skip "data: " prefix

			var zaiResp types.ZaiResponse
			if err := json.Unmarshal(jsonData, &zaiResp); err != nil {
				continue
			}

			ch <- &zaiResp
		}
	}()

	return ch
}

// FormatResponse formats Z.ai response to OpenAI/Anthropic format
func FormatResponse(data *types.ZaiResponse, responseType string) map[string]interface{} {
	if data == nil || data.Data == nil {
		return nil
	}

	phase := data.Data.Phase
	if phase == "" {
		phase = "other"
	}

	content := data.Data.DeltaContent
	if content == "" {
		content = data.Data.EditContent
	}
	if content == "" {
		return nil
	}

	_ = content // Keep original for potential logging

	// Handle tool_call phase
	if phase == "tool_call" {
		content = regexp.MustCompile(`\n*<glm_block[^>]*>\{"type": "mcp", "data": \{"metadata": \{`).ReplaceAllString(content, "{")
		content = regexp.MustCompile(`", "result": "".*</glm_block>`).ReplaceAllString(content, "")
	} else if phase == "other" && phaseBak == "tool_call" && strings.Contains(content, "glm_block") {
		phase = "tool_call"
		content = regexp.MustCompile(`null, "display_result": "".*</glm_block>`).ReplaceAllString(content, "\"}")
	}

	// Get config for thinking mode
	cfg := config.GetConfig()
	thinkMode := cfg.API.Think

	// Handle thinking/answer phase
	if phase == "thinking" || (phase == "answer" && strings.Contains(content, "summary>")) {
		// Remove details tags
		content = regexp.MustCompile(`(?s)<details[^>]*?>.*?</details>`).ReplaceAllString(content, "")
		content = strings.ReplaceAll(content, "</thinking>", "")
		content = strings.ReplaceAll(content, "<Full>", "")
		content = strings.ReplaceAll(content, "</Full>", "")

		if phase == "thinking" {
			content = regexp.MustCompile(`\n*<summary>.*?</summary>\n*`).ReplaceAllString(content, "\n\n")
		}

		// Convert to <reasoning> tags
		content = regexp.MustCompile(`<details[^>]*>\n*`).ReplaceAllString(content, "<reasoning>\n\n")
		content = regexp.MustCompile(`\n*</details>`).ReplaceAllString(content, "\n\n</reasoning>")

		if phase == "answer" {
			// Check if there's content after </reasoning>
			re := regexp.MustCompile(`(?s)^(.*?</reasoning>)(.*)$`)
			matches := re.FindStringSubmatch(content)
			if len(matches) == 3 {
				before := matches[1]
				after := matches[2]
				if strings.TrimSpace(after) != "" {
					// Has content after </reasoning>
					if phaseBak == "thinking" {
						// Thinking pause → end thinking, add answer
						content = fmt.Sprintf("\n\n</reasoning>\n\n%s", strings.TrimLeft(after, "\n"))
					} else if phaseBak == "answer" {
						// Answer pause → clear all
						content = ""
					}
				} else {
					// Thinking pause → no content after </reasoning>
					content = "\n\n</reasoning>"
				}
				_ = before // Keep before for potential use
			}
		}

		// Apply thinking mode transformations
		switch thinkMode {
		case "reasoning":
			if phase == "thinking" {
				content = regexp.MustCompile(`\n>\s?`).ReplaceAllString(content, "\n")
			}
			content = regexp.MustCompile(`\n*<summary>.*?</summary>\n*`).ReplaceAllString(content, "")
			content = regexp.MustCompile(`<reasoning>\n*`).ReplaceAllString(content, "")
			content = regexp.MustCompile(`\n*</reasoning>`).ReplaceAllString(content, "")

		case "think":
			if phase == "thinking" {
				content = regexp.MustCompile(`\n>\s?`).ReplaceAllString(content, "\n")
			}
			content = regexp.MustCompile(`\n*<summary>.*?</summary>\n*`).ReplaceAllString(content, "")
			content = strings.ReplaceAll(content, "<reasoning>", "<think>")
			content = strings.ReplaceAll(content, "</reasoning>", "</think>")

		case "strip":
			content = regexp.MustCompile(`\n*<summary>.*?</summary>\n*`).ReplaceAllString(content, "")
			content = regexp.MustCompile(`<reasoning>\n*`).ReplaceAllString(content, "")
			content = regexp.MustCompile(`</reasoning>`).ReplaceAllString(content, "")

		case "details":
			if phase == "thinking" {
				content = regexp.MustCompile(`\n>\s?`).ReplaceAllString(content, "\n")
			}
			content = strings.ReplaceAll(content, "<reasoning>", "<details type=\"reasoning\" open><div>")

			thoughts := ""
			if phase == "answer" {
				// Extract before part for summary/duration
				re := regexp.MustCompile(`(?s)^(.*?</reasoning>)`)
				matches := re.FindStringSubmatch(content)
				var before string
				if len(matches) >= 2 {
					before = matches[1]
				}

				// Check for summary
				summaryRe := regexp.MustCompile(`(?s)<summary>.*?</summary>`)
				summaryMatch := summaryRe.FindString(before)
				if summaryMatch != "" {
					thoughts = fmt.Sprintf("\n\n%s", summaryMatch)
				} else {
					// Check for duration
					durationRe := regexp.MustCompile(`duration="(\d+)"`)
					durationMatch := durationRe.FindStringSubmatch(before)
					if len(durationMatch) >= 2 {
						thoughts = fmt.Sprintf("\n\n<summary>Thought for %s seconds</summary>", durationMatch[1])
					}
				}
			}
			content = regexp.MustCompile(`</reasoning>`).ReplaceAllString(content, fmt.Sprintf("</div>%s</details>", thoughts))

		default:
			content = regexp.MustCompile(`</reasoning>`).ReplaceAllString(content, "</reasoning>\n\n")
		}
	}

	phaseBak = phase

	// Return formatted response based on type
	if phase == "thinking" && thinkMode == "reasoning" {
		if responseType == "Anthropic" {
			return map[string]interface{}{
				"type":     "thinking_delta",
				"thinking": content,
			}
		}
		return map[string]interface{}{
			"role":              "assistant",
			"reasoning_content": content,
		}
	}

	if phase == "tool_call" {
		return map[string]interface{}{
			"tool_call": content,
		}
	}

	if content != "" {
		if responseType == "Anthropic" {
			return map[string]interface{}{
				"type": "text_delta",
				"text": content,
			}
		}
		return map[string]interface{}{
			"role":    "assistant",
			"content": content,
		}
	}

	return nil
}

// CountTokens counts tokens in text using tiktoken
func CountTokens(text string) int {
	return utils.CountTokens(text)
}

// ExtractTextFromMessages extracts all text content from messages for token counting
func ExtractTextFromMessages(messages []map[string]interface{}) string {
	var texts []string

	for _, msg := range messages {
		content := msg["content"]

		// Handle string content
		if contentStr, ok := content.(string); ok {
			texts = append(texts, contentStr)
			continue
		}

		// Handle array content
		if contentArr, ok := content.([]interface{}); ok {
			for _, item := range contentArr {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemMap["type"] == "text" {
						if text, ok := itemMap["text"].(string); ok {
							texts = append(texts, text)
						}
					}
				}
			}
		}
	}

	return strings.Join(texts, "")
}
