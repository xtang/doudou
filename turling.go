package main

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
)

type TuringClient struct {
	APIBase    string
	APIKey     string
	APISecret  string
	httpClient *http.Client
}

func (c TuringClient) Do(text, user_id string) (string, bool) {
	body := make(map[string]string)
	body["key"] = c.APIKey
	body["info"] = text
	body["userid"] = user_id

	var buf io.ReadWriter
	buf = new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(body)
	if err != nil {
		return "", false
	}

	req, err := http.NewRequest("POST", c.APIBase, buf)
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false
	}

	defer resp.Body.Close()
	var response map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&response)

	code := response["code"]
	respText := response["text"]
	if diff := math.Abs(code.(float64) - 100000); diff < 0.0001 {
		return respText.(string), true
	}
	return "", false
}

func NewTuringClient(apiKey string) *TuringClient {
	return &TuringClient{
		APIBase:    "http://www.tuling123.com/openapi/api",
		APIKey:     apiKey,
		httpClient: http.DefaultClient,
	}
}
