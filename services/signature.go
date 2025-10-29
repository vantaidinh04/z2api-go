package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const secretKey = "key-@@@@)))()((9))-xxxx&&&%%%%%"

// SignatureResult holds the generated signature and timestamp
type SignatureResult struct {
	Signature string
	Timestamp int64
}

// GenerateSignature generates a two-level HMAC-SHA256 signature for Z.ai API
func GenerateSignature(params map[string]string, content string) (*SignatureResult, error) {
	// Validate required parameters
	required := []string{"timestamp", "requestId", "user_id"}
	for _, key := range required {
		if _, ok := params[key]; !ok {
			return nil, fmt.Errorf("missing required parameter: %s", key)
		}
	}

	// Parse timestamp
	requestTime, err := strconv.ParseInt(params["timestamp"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %v", err)
	}

	// Calculate signature expire time (5-minute window)
	signatureExpire := requestTime / (5 * 60 * 1000)

	// Level 1 signature
	plaintext1 := strconv.FormatInt(signatureExpire, 10)
	signature1 := hmacSHA256([]byte(secretKey), []byte(plaintext1))

	// Level 2 signature
	// Base64 encode content
	contentB64 := base64.StdEncoding.EncodeToString([]byte(content))

	// Sort parameters by key
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Format parameters: key1,value1,key2,value2,...
	var paramsStr strings.Builder
	for i, k := range keys {
		if i > 0 {
			paramsStr.WriteString(",")
		}
		paramsStr.WriteString(k)
		paramsStr.WriteString(",")
		paramsStr.WriteString(params[k])
	}

	// Combine for level 2: params|content|timestamp
	plaintext2 := fmt.Sprintf("%s|%s|%d", paramsStr.String(), contentB64, requestTime)
	signature2 := hmacSHA256([]byte(signature1), []byte(plaintext2))

	return &SignatureResult{
		Signature: signature2,
		Timestamp: requestTime,
	}, nil
}

// hmacSHA256 computes HMAC-SHA256 and returns hex-encoded string
func hmacSHA256(key, message []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(message)
	return hex.EncodeToString(h.Sum(nil))
}