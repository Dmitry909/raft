package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func sendRequest(recipient string, request map[string]interface{}) {
	payload, err := json.Marshal(request)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	resp, err := http.Post(recipient, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()
}

type VoteRequest struct {
	Term      int `json:"term"`
	LogLength int `json:"log_length"`
	LogTerm   int `json:"log_term"`
}

type VoteResponse struct {
	Term    int  `json:"term"`
	Granted bool `json:"granted"`
}

func SendVoteRequest(recipient string, term int, logLength int, logTerm int) {
	voteRequest := map[string]interface{}{
		"term":       term,
		"log_length": logLength,
		"log_term":   logTerm,
	}
	sendRequest(recipient, voteRequest)
}

func SendVoteResponse(recipient string, term int, granted bool) {
	voteResponse := map[string]interface{}{
		"term":    term,
		"granted": granted,
	}
	sendRequest(recipient, voteResponse)
}
