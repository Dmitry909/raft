package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"raft/nodestate"
)

func sendRequest(address string, request map[string]interface{}) {
	payload, err := json.Marshal(request)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	resp, err := http.Post(address, "application/json", bytes.NewBuffer(payload))
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

type LogRequest struct {
	Term         int                  `json:"term"`
	LogLength    int                  `json:"logLength"`
	LogTerm      int                  `json:"logTerm"`
	LeaderCommit int                  `json:"leaderCommit"`
	Entries      []nodestate.LogEntry `json:"entries"`
}

type LogResponse struct {
	Term    int  `json:"term"`
	Ack     int  `json:"ack"`
	Success bool `json:"success"`
}

func SendVoteRequest(recipient string, term int, logLength int, logTerm int) {
	voteRequest := map[string]interface{}{
		"term":       term,
		"log_length": logLength,
		"log_term":   logTerm,
	}
	sendRequest(recipient+"vote_request", voteRequest)
}

func SendVoteResponse(recipient string, term int, granted bool) {
	voteResponse := map[string]interface{}{
		"term":    term,
		"granted": granted,
	}
	sendRequest(recipient+"vote_response", voteResponse)
}

func SendLogRequest(recipient string, term, logLength, logTerm, leaderCommit int, entries []nodestate.LogEntry) {
	logRequest := map[string]interface{}{
		"term":         term,
		"logLength":    logLength,
		"logTerm":      logTerm,
		"leaderCommit": leaderCommit,
		"entries":      entries,
	}
	sendRequest(recipient+"/log_request", logRequest)
}

func SendLogResponse(recipient string, term int, ack int, success bool) {
	logResponse := map[string]interface{}{
		"term":    term,
		"ack":     ack,
		"success": success,
	}
	sendRequest(recipient+"/log_response", logResponse)
}
