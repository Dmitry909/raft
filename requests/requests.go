package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type VoteRequest struct {
	SendersTerm int `json:"senders_term"`
	LogLength   int `json:"len_log"`
	LogLastTerm int `json:"log_last_term"`
}

func SendVoteRequest(url string, sendersTerm int64, lenLog int, logLastTerm int64) {
	voteRequest := map[string]interface{}{
		"senders_term":  sendersTerm,
		"len_log":       lenLog,
		"log_last_term": logLastTerm,
	}

	payload, err := json.Marshal(voteRequest)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()
}
