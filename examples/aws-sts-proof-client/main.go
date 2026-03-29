// Package main implements the tool.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/udhos/aws-sts-proof/awsstsproof"
	"github.com/udhos/boilerplate/awsconfig"
)

func main() {

	//
	// aws config
	//

	options := awsconfig.Options{}
	awsCfg, errCfg := awsconfig.AwsConfig(options)
	if errCfg != nil {
		log.Fatalf("could not get aws config: %v", errCfg)
	}

	log.Printf("STS account ID: %s\n", awsCfg.StsAccountID)
	log.Printf("STS ARN: %s\n", awsCfg.StsArn)
	log.Printf("STS UserId: %s\n", awsCfg.StsUserID)

	//
	// sts client
	//

	clientSts := sts.NewFromConfig(awsCfg.AwsConfig)

	// create a presigned STS GetCallerIdentity request and use it
	// as the proof request target (URL + signed headers)
	presignClient := sts.NewPresignClient(clientSts)

	presigned, errPresign := presignClient.PresignGetCallerIdentity(context.TODO(),
		&sts.GetCallerIdentityInput{})
	if errPresign != nil {
		log.Fatalf("presign error: %v", errPresign)
	}

	input := awsstsproof.Param{
		Method:  presigned.Method,
		URL:     presigned.URL,
		Headers: presigned.SignedHeader,
	}

	body, errBody := awsstsproof.NewBody(input)
	if errBody != nil {
		log.Fatalf("new body error: %v", errBody)
	}

	bodyBytes, errJSON := awsstsproof.Marshal(body)
	if errJSON != nil {
		log.Fatalf("json error: %v", errJSON)
	}

	log.Printf("raw request: body:%s", string(bodyBytes))

	reader := bytes.NewBuffer(bodyBytes)

	req, errReq := http.NewRequestWithContext(context.TODO(),
		"POST", "http://localhost:8080/auth", reader)
	if errReq != nil {
		log.Fatalf("request error: %v", errReq)
	}

	client := http.DefaultClient

	resp, errDo := client.Do(req)
	if errDo != nil {
		log.Fatalf("http send error: %v", errDo)
	}

	defer resp.Body.Close()

	log.Printf("response status: %d", resp.StatusCode)

	respBody, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Fatalf("read response error: %v", errRead)
	}

	log.Print("response:")

	fmt.Print(string(respBody))
}
