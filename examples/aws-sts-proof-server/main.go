// Package main implements the tool.
package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/udhos/aws-sts-proof/awsstsproof"
)

type application struct {
}

func main() {

	const addr = ":8080"
	const pathHealth = "/health"
	const pathAuth = "/auth"

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	app := &application{}

	const root = "/"

	register(mux, addr, root, handlerRoot)
	register(mux, addr, pathHealth, handlerHealth)
	register(mux, addr, pathAuth, func(w http.ResponseWriter, r *http.Request) { handlerToken(w, r, app) })

	go listenAndServe(server, addr)

	select {}
}

func register(mux *http.ServeMux, addr, path string, handler http.HandlerFunc) {
	mux.HandleFunc(path, handler)
	log.Printf("registered on port %s path %s", addr, path)
}

func listenAndServe(s *http.Server, addr string) {
	log.Printf("listening on port %s", addr)
	err := s.ListenAndServe()
	log.Fatalf("listening on port %s: %v", addr, err)
}

// httpJSON replies to the request with the specified error message and HTTP code.
// It does not otherwise end the request; the caller should ensure no further
// writes are done to w.
// The message should be JSON.
func httpJSON(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	fmt.Fprintln(w, message)
}

func response(w http.ResponseWriter, r *http.Request, status int, message string) {
	hostname, errHost := os.Hostname()
	if errHost != nil {
		log.Printf("hostname error: %v", errHost)
	}
	reply := fmt.Sprintf(`{"message":"%s","status":"%d","path":"%s","method":"%s","host":"%s","serverHostname":"%s"}`,
		message, status, r.RequestURI, r.Method, r.Host, hostname)
	httpJSON(w, reply, status)
}

func handlerRoot(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s - 404 not found", r.RemoteAddr, r.Method, r.RequestURI)
	response(w, r, http.StatusNotFound, "not found")
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	response(w, r, http.StatusOK, "health ok")
}

func handlerToken(w http.ResponseWriter, r *http.Request, _ *application) {

	reqBody, errBody := io.ReadAll(r.Body)
	if errBody != nil {
		response(w, r, http.StatusBadRequest, errBody.Error())
		return
	}

	var payload awsstsproof.Body

	if errJSON := awsstsproof.Unmarshal(reqBody, &payload); errJSON != nil {
		response(w, r, http.StatusBadRequest, errJSON.Error())
		return
	}

	param, errParam := payload.Decode()
	if errParam != nil {
		response(w, r, http.StatusBadRequest, errParam.Error())
		return
	}

	//log.Printf("FIXME %v", app.stsClient)

	// Forward the presigned request to AWS using a plain HTTP client
	log.Printf("%s %s %s - raw body: %v", r.RemoteAddr, r.Method, r.RequestURI, string(reqBody))
	log.Printf("%s %s %s - body:%s", r.RemoteAddr, r.Method, r.RequestURI, string(toJSON(param)))

	// validate presigned request looks like STS GetCallerIdentity
	if param.Method == "" {
		response(w, r, http.StatusBadRequest, "missing method in presigned request")
		return
	}
	if param.URL == "" {
		response(w, r, http.StatusBadRequest, "missing url in presigned request")
		return
	}

	if param.Method != "GET" {
		response(w, r, http.StatusBadRequest, "presigned request must be GET")
		return
	}

	u, errParse := url.Parse(param.URL)
	if errParse != nil {
		response(w, r, http.StatusBadRequest, "invalid url in presigned request")
		return
	}

	// require Query Action=GetCallerIdentity
	if u.Query().Get("Action") != "GetCallerIdentity" {
		response(w, r, http.StatusBadRequest, "presigned request Action is not GetCallerIdentity")
		return
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
		response(w, r, http.StatusBadRequest, "presigned request missing signature")
		return
	}

	reqToAws, errReq := http.NewRequestWithContext(r.Context(), param.Method, param.URL, nil)
	if errReq != nil {
		response(w, r, http.StatusBadRequest, errReq.Error())
		return
	}
	for k, vv := range param.Headers {
		for _, v := range vv {
			reqToAws.Header.Add(k, v)
		}
	}

	respAws, errDo := http.DefaultClient.Do(reqToAws)
	if errDo != nil {
		response(w, r, http.StatusBadGateway, errDo.Error())
		return
	}
	defer respAws.Body.Close()

	respData, errRead := io.ReadAll(respAws.Body)
	if errRead != nil {
		response(w, r, http.StatusBadGateway, errRead.Error())
		return
	}

	//log.Printf("sts response: %s", string(respData))

	// Parse STS GetCallerIdentity XML response
	var stsResp struct {
		Result struct {
			UserID  string `xml:"UserId"`
			Account string `xml:"Account"`
			Arn     string `xml:"Arn"`
		} `xml:"GetCallerIdentityResult"`
	}
	if err := xml.Unmarshal(respData, &stsResp); err != nil {
		log.Printf("xml unmarshal error: %v", err)
		// return raw AWS body for debugging
		response(w, r, http.StatusOK, string(respData))
		return
	}

	iamARN := stsResp.Result.Arn
	log.Printf("STS gerCallerIdentity ARN: %s", iamARN)
	response(w, r, http.StatusOK, fmt.Sprintf("getCallerIdentity.ARN=%s", iamARN))
}

func toJSON(v any) []byte {
	s, _ := json.Marshal(v)
	return s
}
