package lib_email

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"goteams/internal/app/goteams/define"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

type senderConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

type sendRequest struct {
	To          []string      `json:"to"`
	Subject     string        `json:"subject,omitempty"`
	Body        string        `json:"body,omitempty"`
	ContentType string        `json:"content_type,omitempty"`
	From        *senderConfig `json:"from,omitempty"`
}

// SendEmail 通过配置的邮件服务发送邮件，参数与 PHP common_send_email 保持一致。
func SendEmail(to []string, subject string, body string, contentType string) error {
	validTo := make([]string, 0, len(to))
	for _, address := range to {
		address = strings.TrimSpace(address)
		parsed, err := mail.ParseAddress(address)
		if err == nil && strings.EqualFold(parsed.Address, address) {
			validTo = append(validTo, strings.ToLower(address))
		}
	}
	if len(validTo) == 0 {
		return errors.New(`email recipient is invalid`)
	}
	url := strings.TrimSpace(define.Config.Email[`url`])
	if url == `` {
		return errors.New(`email service url is not configured`)
	}
	if contentType == `` {
		contentType = `text/html`
	}
	reqData := sendRequest{To: validTo, Subject: subject, Body: body, ContentType: contentType}
	if username := strings.TrimSpace(define.Config.Email[`from_username`]); username != `` {
		reqData.From = &senderConfig{
			Username: username,
			Password: define.Config.Email[`from_password`],
			Host:     define.Config.Email[`from_host`],
			Port:     parseIntDefault(define.Config.Email[`from_port`], 465),
		}
	}
	bodyBytes, err := json.Marshal(reqData)
	if err != nil {
		return err
	}
	timeout := time.Duration(parseIntDefault(define.Config.Email[`timeout`], 30)) * time.Second
	client := &http.Client{Timeout: timeout}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	request.Header.Set(`Content-Type`, `application/json`)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("email service returned status %d", response.StatusCode)
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return err
	}
	if err := validateServiceResponse(responseBody); err != nil {
		return err
	}
	return nil
}

func validateServiceResponse(body []byte) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return errors.New(`email service returned an empty response`)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("email service returned invalid json: %w", err)
	}
	if value, ok := result[`res`]; ok && numericCode(value) != 0 {
		return fmt.Errorf("email service returned res %v", value)
	}
	if value, ok := result[`code`]; ok {
		code := numericCode(value)
		if code != 0 && code != http.StatusOK {
			return fmt.Errorf("email service returned code %v", value)
		}
	}
	return nil
}

func numericCode(value any) int {
	switch code := value.(type) {
	case float64:
		return int(code)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(code))
		return parsed
	default:
		return 0
	}
}

func parseIntDefault(value string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
