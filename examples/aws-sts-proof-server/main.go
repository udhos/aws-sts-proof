// Package main implements the tool.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

	log.Printf("%s %s %s - raw body: %v", r.RemoteAddr, r.Method, r.RequestURI, string(reqBody))
	log.Printf("%s %s %s - body:%s", r.RemoteAddr, r.Method, r.RequestURI, string(toJSON(param)))

	//
	// Forward the presigned request to AWS using a plain HTTP client
	//

	resp, status, err := awsstsproof.VerifyPresignedGetCallerIdentity(r.Context(),
		http.DefaultClient, param)
	if err != nil {
		response(w, r, status, err.Error())
		return
	}

	iamARN := resp.Result.Arn
	log.Printf("STS gerCallerIdentity ARN: %s", iamARN)
	response(w, r, http.StatusOK, fmt.Sprintf("getCallerIdentity.ARN=%s", iamARN))
}

func toJSON(v any) []byte {
	s, _ := json.Marshal(v)
	return s
}
