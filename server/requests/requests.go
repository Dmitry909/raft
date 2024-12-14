package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"raft/nodestate"
)

func sendRequest(address string, request map[string]interface{}, mutex *sync.Mutex) {
	payload, err := json.Marshal(request)
	if mutex != nil {
		(*mutex).Unlock()
	}
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
	SenderAddress string `json:"sender_address"`
	Term          int    `json:"term"`
	LogLength     int    `json:"log_length"`
	LogTerm       int    `json:"log_term"`
}

type VoteResponse struct {
	SenderAddress string `json:"sender_address"`
	Term          int    `json:"term"`
	Granted       bool   `json:"granted"`
}

type LogRequest struct {
	SenderAddress string               `json:"sender_address"`
	Term          int                  `json:"term"`
	LogLength     int                  `json:"logLength"`
	LogTerm       int                  `json:"logTerm"`
	LeaderCommit  int                  `json:"leaderCommit"`
	Entries       []nodestate.LogEntry `json:"entries"`
}

type LogResponse struct {
	SenderAddress string `json:"sender_address"`
	Term          int    `json:"term"`
	Ack           int    `json:"ack"`
	Success       bool   `json:"success"`
}

type Read struct {
	Value string `json:"value"`
}

type CurrentRole struct {
	Role string `json:"role"`
}

type UpdateRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func SendVoteRequest(sender_address string, recipient string, term int, logLength int, logTerm int) {
	voteRequest := map[string]interface{}{
		"sender_address": sender_address,
		"term":           term,
		"log_length":     logLength,
		"log_term":       logTerm,
	}
	sendRequest("http://"+recipient+"/vote_request", voteRequest, nil)
}

func SendVoteResponse(sender_address string, recipient string, term int, granted bool, mutex *sync.Mutex) {
	if mutex != nil {
		(*mutex).Unlock()
	}
	voteResponse := map[string]interface{}{
		"sender_address": sender_address,
		"term":           term,
		"granted":        granted,
	}
	sendRequest("http://"+recipient+"/vote_response", voteResponse, nil)
}

func SendLogRequest(sender_address string, recipient string, term, logLength, logTerm, leaderCommit int, entries []nodestate.LogEntry, mutex *sync.Mutex) {
	logRequest := map[string]interface{}{
		"sender_address": sender_address,
		"term":           term,
		"logLength":      logLength,
		"logTerm":        logTerm,
		"leaderCommit":   leaderCommit,
		"entries":        entries,
	}
	sendRequest("http://"+recipient+"/log_request", logRequest, mutex)
}

func SendLogResponse(sender_address string, recipient string, term int, ack int, success bool, mutex *sync.Mutex) {
	(*mutex).Unlock()
	logResponse := map[string]interface{}{
		"sender_address": sender_address,
		"term":           term,
		"ack":            ack,
		"success":        success,
	}
	sendRequest("http://"+recipient+"/log_response", logResponse, nil)
}
