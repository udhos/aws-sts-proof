// Package main implements the tool.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/udhos/aws-sts-proof/awsstsproof"
)

func main() {

	headers := http.Header{}
	reqBody := []byte("test-request-body")

	input := awsstsproof.Param{
		Method:  "GET",
		URL:     "https://localhost:3000",
		Headers: headers,
		Body:    reqBody,
	}

	body, errBody := awsstsproof.NewBody(input)
	if errBody != nil {
		log.Fatalf("new body error: %v", errBody)
	}

	bodyBytes, errJSON := awsstsproof.Marshal(body)
	if errJSON != nil {
		log.Fatalf("json error: %v", errJSON)
	}

	log.Printf("raw request: body:%s body-to-string:%s", string(bodyBytes), string(reqBody))

	reader := bytes.NewBuffer(bodyBytes)

	req, errReq := http.NewRequestWithContext(context.TODO(), "POST", "http://localhost:8080/auth", reader)
	if errReq != nil {
		log.Fatalf("request error: %v", errReq)
	}

	client := http.DefaultClient

	resp, errDo := client.Do(req)
	if errDo != nil {
		log.Fatalf("http send error: %v", errDo)
	}

	log.Printf("response status: %d", resp.StatusCode)

	respBody, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Fatalf("read response error: %v", errRead)
	}

	log.Print("response:")

	fmt.Print(string(respBody))
}
