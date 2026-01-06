// Package awsstsproof provides helpers.
package awsstsproof

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
)

// Body represents the request payload.
type Body struct {
	Method  string `json:"iam_http_request_method"`
	URL     string `json:"iam_request_url"`
	Headers string `json:"iam_request_headers"`
	Body    string `json:"iam_request_body"`
}

// Param represents body parameters.
type Param struct {
	Method  string
	URL     string
	Headers http.Header
	Body    []byte
}

// Marshal encodes the body as JSON.
func Marshal(body Body) ([]byte, error) {
	return json.Marshal(body)
}

// Unmarshal decodes JSON data into body.
func Unmarshal(data []byte, v *Body) error {
	return json.Unmarshal(data, v)
}

// NewBody creates a body.
func NewBody(input Param) (Body, error) {

	headersJSON, err := json.Marshal(input.Headers)
	if err != nil {
		return Body{}, err
	}

	return Body{
		Method:  input.Method,
		URL:     base64.StdEncoding.EncodeToString([]byte(input.URL)),
		Headers: base64.StdEncoding.EncodeToString(headersJSON),
		Body:    base64.StdEncoding.EncodeToString(input.Body),
	}, nil
}

// Decode extracts parameters from body.
func (b *Body) Decode() (out Param, err error) {
	u, errURL := base64.StdEncoding.DecodeString(b.URL)
	if errURL != nil {
		err = errURL
		return
	}

	h, errHeaders := base64.StdEncoding.DecodeString(b.Headers)
	if errHeaders != nil {
		err = errHeaders
		return
	}

	headers := http.Header{}
	if err = json.Unmarshal(h, &headers); err != nil {
		return
	}

	body, errBody := base64.StdEncoding.DecodeString(b.Body)
	if errBody != nil {
		err = errBody
		return
	}

	out.Method = b.Method
	out.URL = string(u)
	out.Headers = headers
	out.Body = body

	return
}
