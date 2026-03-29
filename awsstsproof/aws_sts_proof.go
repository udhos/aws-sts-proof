// Package awsstsproof provides helpers.
package awsstsproof

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

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

// VerifyResponse represents the response from AWS STS GetCallerIdentity.
type VerifyResponse struct {
	Result VerifyResult `xml:"GetCallerIdentityResult"`
}

// VerifyResult represents the result from AWS STS GetCallerIdentity.
type VerifyResult struct {
	UserID  string `xml:"UserId"`
	Account string `xml:"Account"`
	Arn     string `xml:"Arn"`
}

// VerifyPresignedGetCallerIdentity forwards the presigned request to AWS and returns the response.
func VerifyPresignedGetCallerIdentity(ctx context.Context, client *http.Client,
	param Param) (VerifyResponse, int, error) {

	//
	// Forward the presigned request to AWS using a plain HTTP client
	//

	var resp VerifyResponse

	// validate presigned request looks like STS GetCallerIdentity
	if param.Method == "" {
		return resp, http.StatusBadRequest, fmt.Errorf("missing method in presigned request")
	}
	if param.URL == "" {
		return resp, http.StatusBadRequest, fmt.Errorf("missing url in presigned request")
	}

	if param.Method != "GET" {
		return resp, http.StatusBadRequest, fmt.Errorf("presigned request must be GET")
	}

	u, errParse := url.Parse(param.URL)
	if errParse != nil {
		return resp, http.StatusBadRequest, fmt.Errorf("invalid url in presigned request")
	}

	// require Query Action=GetCallerIdentity
	if u.Query().Get("Action") != "GetCallerIdentity" {
		return resp, http.StatusBadRequest, fmt.Errorf("presigned request Action is not GetCallerIdentity")
	}

	// require signature: either Authorization header or X-Amz-Signature in query or headers
	hasAuth := false
	if _, ok := param.Headers["Authorization"]; ok {
		hasAuth = true
	}
	if u.Query().Get("X-Amz-Signature") != "" {
		hasAuth = true
	}
	if _, ok := param.Headers["X-Amz-Signature"]; ok {
		hasAuth = true
	}
	if !hasAuth {
		return resp, http.StatusBadRequest, fmt.Errorf("presigned request missing signature")
	}

	reqToAws, errReq := http.NewRequestWithContext(ctx, param.Method, param.URL, nil)
	if errReq != nil {
		return resp, http.StatusBadRequest, fmt.Errorf("error creating request to AWS: %v", errReq)
	}
	for k, vv := range param.Headers {
		for _, v := range vv {
			reqToAws.Header.Add(k, v)
		}
	}

	respAws, errDo := client.Do(reqToAws)
	if errDo != nil {
		return resp, http.StatusBadGateway, fmt.Errorf("error forwarding request to AWS: %v", errDo)
	}
	defer respAws.Body.Close()

	respData, errRead := io.ReadAll(respAws.Body)
	if errRead != nil {
		return resp, http.StatusBadGateway, fmt.Errorf("error reading response from AWS: %v", errRead)
	}

	if respAws.StatusCode != 200 {
		return resp, http.StatusBadGateway, fmt.Errorf("bad status=%d body:%s", respAws.StatusCode, string(respData))
	}

	// Parse STS GetCallerIdentity XML response

	if err := xml.Unmarshal(respData, &resp); err != nil {
		log.Printf("xml unmarshal error: %v", err)
		// return raw AWS body for debugging
		return resp, http.StatusBadGateway, fmt.Errorf("error parsing AWS response: %v", err)
	}

	return resp, http.StatusOK, nil
}
