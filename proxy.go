package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/auth"
)

type ProxyRequest struct {
	Method     string              `json:"method"`
	URL        string              `json:"url"`
	Headers    map[string][]string `json:"headers,omitempty"`
	BodyBase64 string              `json:"body_base64,omitempty"`
}

type ProxyResponse struct {
	Status     int                 `json:"status"`
	StatusText string              `json:"status_text"`
	Headers    map[string][]string `json:"headers"`
	BodyBase64 string              `json:"body_base64"`
}

type WorkerProxyTransport struct {
	WorkerURL    string
	WorkerSecret string
	Base         http.RoundTripper
}

func installWorkerProxy() {
	workerURL := "https://cloudflare-proxy.restartfu.workers.dev/proxy"
	workerSecret := os.Getenv("PROXY_SHARED_SECRET")

	baseTransport := http.DefaultTransport
	proxyTransport := &WorkerProxyTransport{
		WorkerURL:    workerURL,
		WorkerSecret: workerSecret,
		Base:         baseTransport,
	}
	proxyClient := &http.Client{
		Transport: proxyTransport,
	}

	http.DefaultClient = proxyClient
	http.DefaultTransport = proxyTransport
	auth.SetHTTPClient(proxyClient)
}

func (t *WorkerProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil {
		return nil, fmt.Errorf("nil request URL")
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	if strings.HasPrefix(req.URL.String(), t.WorkerURL) {
		return base.RoundTrip(req)
	}

	host := strings.ToLower(req.URL.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return base.RoundTrip(req)
	}

	if req.URL.Scheme != "https" {
		return nil, fmt.Errorf("only https URLs are allowed through the worker proxy: %s", req.URL.String())
	}

	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	headers := make(map[string][]string, len(req.Header))
	for k, v := range req.Header {
		vv := make([]string, len(v))
		copy(vv, v)
		headers[k] = vv
	}

	payload := ProxyRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: headers,
	}
	if len(bodyBytes) > 0 {
		payload.BodyBase64 = base64.StdEncoding.EncodeToString(bodyBytes)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	workerReq, err := http.NewRequest(http.MethodPost, t.WorkerURL, bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, err
	}
	workerReq.Header.Set("Content-Type", "application/json")
	if t.WorkerSecret != "" {
		workerReq.Header.Set("Authorization", "Bearer "+t.WorkerSecret)
	}

	workerResp, err := base.RoundTrip(workerReq)
	if err != nil {
		return nil, err
	}
	defer workerResp.Body.Close()

	raw, err := io.ReadAll(workerResp.Body)
	if err != nil {
		return nil, err
	}

	if workerResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worker returned %s: %s", workerResp.Status, string(raw))
	}

	var proxyResp ProxyResponse
	if err := json.Unmarshal(raw, &proxyResp); err != nil {
		return nil, err
	}

	respBody, err := base64.StdEncoding.DecodeString(proxyResp.BodyBase64)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode:    proxyResp.Status,
		Status:        fmt.Sprintf("%d %s", proxyResp.Status, proxyResp.StatusText),
		Header:        http.Header(proxyResp.Headers),
		Body:          io.NopCloser(bytes.NewReader(respBody)),
		ContentLength: int64(len(respBody)),
		Request:       req,
	}, nil
}
