package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"raft/nodestate"
	"raft/requests"
	"sync"
	"time"
)

var localIP = "127.0.0.1"
var allNodes = [5]string{localIP + ":8000", localIP + ":8001", localIP + ":8002", localIP + ":8003", localIP + ":8004"}
var nodesExceptMe = []string{}
var port string
var nodeId string
var suspectLeaderFailureTimeout time.Duration
var electionTimeout time.Duration

func init() {
	port = os.Args[1]
	nodeId = localIP + ":" + port
	contains := false
	for _, address := range allNodes {
		if address == nodeId {
			contains = true
		} else {
			nodesExceptMe = append(nodesExceptMe, address)
		}
	}
	if !contains {
		log.Fatal("Wrong port " + port)
	}
}

var importantState nodestate.ImportantState
var unimportantState nodestate.UnimportantState
var mutex sync.Mutex

// const electionTime := 1s

func BecomeCandidateAndStartElection() { // mutex must be locked
	fmt.Println("Called BecomeCandidateAndStartElection")
	importantState.CurrentTerm += 1
	unimportantState.CurrentRole = nodestate.Candidate
	importantState.VotedFor = nodeId
	unimportantState.VotesRecieved = map[string]struct{}{}
	unimportantState.VotesRecieved[nodeId] = struct{}{}

	var lastTerm int = 0
	if len(importantState.Log) > 0 {
		lastTerm = importantState.Log[len(importantState.Log)-1].Term
	}
	currentTerm := importantState.CurrentTerm
	logLength := len(importantState.Log)
	importantState.SaveToFile()

	mutex.Unlock()

	for _, node := range allNodes {
		fmt.Println("Sending vote request to", node)
		requests.SendVoteRequest(node+"/vote_request", currentTerm, logLength, lastTerm)
	}

	sleepUntil := time.Now().Add(electionTimeout)
	go func() {
		time.Sleep(time.Until(sleepUntil))
		// TODO on election timeout
	}()
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func CheckLeaderFailurePeriodically() {
	for {
		mutex.Lock()
		isReallyFollower := unimportantState.CurrentRole == nodestate.Follower && unimportantState.CurrentLeader != ""
		lastHeartbeat := unimportantState.LastHeartbeat
		suspectFailure := lastHeartbeat.Add(suspectLeaderFailureTimeout).Before(time.Now())
		if isReallyFollower && suspectFailure {
			fmt.Println("suspected leader failure at", time.Now())
			BecomeCandidateAndStartElection()
		} else {
			mutex.Unlock()
		}

		// fmt.Println("isReallyFollower:", isReallyFollower)
		if isReallyFollower {
			time.Sleep(time.Until(lastHeartbeat.Add(suspectLeaderFailureTimeout)))
		} else {
			time.Sleep(suspectLeaderFailureTimeout)
		}
	}
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	if unimportantState.CurrentRole == nodestate.Leader {
		mutex.Unlock()
		randomNode := nodesExceptMe[rand.Intn(len(nodesExceptMe))]
		http.Redirect(w, r, "http://"+randomNode+"/read?key="+key, http.StatusFound)
		return
	}

	value := ""
	for i := len(importantState.Log) - 1; i >= 0; i-- {
		if importantState.Log[i].K == key {
			value = importantState.Log[i].V
			break
		}
	}
	mutex.Unlock()
	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}
	if value == "" {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, value)
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	value := r.URL.Query().Get("value")

	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}
	if unimportantState.CurrentRole != nodestate.Leader {
		http.Redirect(w, r, "http://"+unimportantState.CurrentLeader+"/read?key="+key, http.StatusFound)
		mutex.Unlock()
		return
	}

	newEntry := nodestate.LogEntry{Term: importantState.CurrentTerm, OperatType: nodestate.Write, K: key, V: value}
	importantState.Log = append(importantState.Log, newEntry)
	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}
	if unimportantState.CurrentRole != nodestate.Leader {
		http.Redirect(w, r, "http://"+unimportantState.CurrentLeader+"/read?key="+key, http.StatusFound)
		mutex.Unlock()
		return
	}

	newEntry := nodestate.LogEntry{Term: importantState.CurrentTerm, OperatType: nodestate.Delete, K: key, V: ""}
	importantState.Log = append(importantState.Log, newEntry)
	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	heartbeatSender := r.RemoteAddr
	time := time.Now()
	fmt.Println("heartbeat recieved from", heartbeatSender, "at", time)
	mutex.Lock()
	if heartbeatSender == unimportantState.CurrentLeader {
		unimportantState.LastHeartbeat = time
	}
	mutex.Unlock()
	w.WriteHeader(http.StatusOK)
}

func voteRequestHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var c requests.VoteRequest
	err = json.Unmarshal(body, &c)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
	fmt.Printf("Received vote request: %+v\n", c)

	cId := r.RemoteAddr
	mutex.Lock()
	myLogTerm := 0
	if n := len(importantState.Log); n > 0 {
		myLogTerm = importantState.Log[n-1].Term
	}
	logOk := (c.LogTerm > myLogTerm) ||
		(c.LogTerm == myLogTerm && c.LogLength >= len(importantState.Log))
	termOk := (c.Term > importantState.CurrentTerm) ||
		(c.Term == importantState.CurrentTerm && (importantState.VotedFor == cId || importantState.VotedFor == ""))

	if logOk && termOk {
		importantState.CurrentTerm = c.Term
		unimportantState.CurrentRole = nodestate.Follower
		importantState.VotedFor = cId
		requests.SendVoteResponse(cId, importantState.CurrentTerm, true)
		importantState.SaveToFile()
	} else {
		requests.SendVoteResponse(cId, importantState.CurrentTerm, false)
	}

	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func voteResponseHandler(w http.ResponseWriter, r *http.Request) {

	w.WriteHeader(http.StatusOK)
}

func logRequestHandler(w http.ResponseWriter, r *http.Request) {

	w.WriteHeader(http.StatusOK)
}

func logResponseHandler(w http.ResponseWriter, r *http.Request) {

	w.WriteHeader(http.StatusOK)
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	unimportantState.IsStopped = true
	mutex.Unlock()
	w.WriteHeader(http.StatusOK)
}

func recoverHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	unimportantState.IsStopped = false
	mutex.Unlock()
	w.WriteHeader(http.StatusOK)
}

func main() {
	fmt.Println("nodeId:", nodeId)
	if nodestate.CheckStateOnDisk() {
		importantState.LoadFromFile()
		fmt.Printf("state recovered after crash: %+v\n", importantState)
	} else {
		importantState.CurrentTerm = 0
		importantState.VotedFor = ""
		importantState.Log = nil
		importantState.CommitLength = 0
		fmt.Println("initialized new state")
	}
	importantState.SaveToFile()

	unimportantState.CurrentRole = nodestate.Follower
	unimportantState.CurrentLeader = ""
	unimportantState.VotesRecieved = nil
	unimportantState.SentLength = nil
	unimportantState.AckedLength = nil

	suspectLeaderFailureTimeout = time.Duration(rand.Intn(450)+50) * time.Millisecond
	fmt.Println("suspect timeout:", suspectLeaderFailureTimeout)
	electionTimeout = 2 * suspectLeaderFailureTimeout

	go CheckLeaderFailurePeriodically()

	// user-called handlers
	http.HandleFunc("/read", readHandler)
	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/delete", deleteHandler)

	// handlers for node-to-node communication
	http.HandleFunc("/heartbeat", heartbeatHandler)
	http.HandleFunc("/vote_request", voteRequestHandler)
	http.HandleFunc("/vote_response", voteResponseHandler)
	http.HandleFunc("/log_request", logRequestHandler)
	http.HandleFunc("/log_response", logResponseHandler)

	// handlers for testing
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/recover", recoverHandler)

	if err := http.ListenAndServe(nodeId, nil); err != nil {
		log.Fatalf("could not start server: %s\n", err)
	}
}
