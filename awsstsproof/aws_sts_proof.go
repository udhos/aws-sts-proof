// Package awsstsproof provides helpers.
package awsstsproof

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Body represents the request payload.
type Body struct {
	Method  string `json:"iam_http_request_method"`
	URL     string `json:"iam_request_url"`
	Headers string `json:"iam_request_headers"`
}

// Param represents body parameters.
type Param struct {
	Method  string
	URL     string
	Headers http.Header
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

	out.Method = b.Method
	out.URL = string(u)
	out.Headers = headers

	return
}

// PresignGetCallerIdentity creates a presigned STS GetCallerIdentity request and returns the parameters.
func PresignGetCallerIdentity(awsConfig aws.Config) (Param, error) {
	//
	// sts client
	//

	clientSts := sts.NewFromConfig(awsConfig)

	// create a presigned STS GetCallerIdentity request and use it
	// as the proof request target (URL + signed headers)
	presignClient := sts.NewPresignClient(clientSts)

	presigned, errPresign := presignClient.PresignGetCallerIdentity(context.TODO(),
		&sts.GetCallerIdentityInput{})
	if errPresign != nil {
		return Param{}, fmt.Errorf("presign error: %v", errPresign)
	}

	input := Param{
		Method:  presigned.Method,
		URL:     presigned.URL,
		Headers: presigned.SignedHeader,
	}

	return input, nil
}
