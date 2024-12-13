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
	"raft/util"
	"strconv"
	"sync"
	"time"
)

var allNodes = []string{}
var nodesExceptMe = []string{}
var port string
var nodeId string
var suspectLeaderFailureTimeout time.Duration
var electionTimeout time.Duration
var replicateTimeout time.Duration

func init() {
	port = os.Args[1]
	nodeId = util.LocalIP + ":" + port

	allNodes = util.ConvertPortsToSlice(os.Args[2])

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

	num, _ := strconv.Atoi(os.Args[3])
	suspectLeaderFailureTimeout = time.Duration(num) * time.Millisecond
	fmt.Println("suspect timeout:", suspectLeaderFailureTimeout)
	electionTimeout = 2 * suspectLeaderFailureTimeout
	replicateTimeout = 2 * suspectLeaderFailureTimeout
}

var importantState nodestate.ImportantState
var unimportantState nodestate.UnimportantState
var mutex sync.Mutex

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
	termBeforeElection := importantState.CurrentTerm
	logLength := len(importantState.Log)
	importantState.SaveToFile()

	unimportantState.ElectionIteration++
	electionIterationBefore := unimportantState.ElectionIteration
	mutex.Unlock()

	for _, node := range allNodes {
		fmt.Println("Sending vote request to", node)
		requests.SendVoteRequest(node, termBeforeElection, logLength, lastTerm)
	}

	sleepUntil := time.Now().Add(electionTimeout)
	go func() {
		time.Sleep(time.Until(sleepUntil))
		mutex.Lock()
		if unimportantState.ElectionIteration == electionIterationBefore && importantState.CurrentTerm == termBeforeElection && unimportantState.CurrentRole == nodestate.Candidate {
			mutex.Unlock()
			BecomeCandidateAndStartElection()
		} else {
			mutex.Unlock()
		}
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

func ReplicateLog(leaderId string, followerId string) {
	mutex.Lock()
	i := unimportantState.SentLength[followerId]
	entries := importantState.Log[i:]
	prevLogTerm := 0
	if i > 0 {
		prevLogTerm = importantState.Log[i-1].Term
	}
	requests.SendLogRequest(followerId, importantState.CurrentTerm, i, prevLogTerm, importantState.CommitLength, entries) // TODO а здесь точно i, а не n-i?
	// TODO в этой функции и соседних анлочить мьютекс до http-вызова.
	mutex.Unlock()
}

func ReplicateLogPeriodically() {
	for {
		mutex.Lock()
		if unimportantState.CurrentRole == nodestate.Leader {
			for _, follower := range nodesExceptMe {
				ReplicateLog(nodeId, follower)
			}
		}
		mutex.Unlock()
		time.Sleep(replicateTimeout)
	}
}

func deliverMessage(i int) {
	fmt.Println("DELIVERING MESSAGE. key:", importantState.Log[i].K, ", value:", importantState.Log[i].V)
}

func AppendEntries(logLength, leaderCommit int, entries []nodestate.LogEntry) {
	if len(entries) > 0 && len(importantState.Log) > logLength {
		if importantState.Log[logLength].Term != entries[0].Term {
			importantState.Log = importantState.Log[:logLength]
		}
	}
	if logLength+len(entries) > len(importantState.Log) {
		for i := len(importantState.Log) - logLength; i < len(entries); i++ {
			importantState.Log = append(importantState.Log, entries[i])
		}
	}
	if leaderCommit > importantState.CommitLength {
		for i := importantState.CommitLength; i < leaderCommit; i++ {
			deliverMessage(i)
		}
		importantState.CommitLength = leaderCommit
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

	response := requests.Read{Value: value}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
	unimportantState.AckedLength[nodeId] = len(importantState.Log)
	for _, follower := range nodesExceptMe {
		ReplicateLog(nodeId, follower)
	}
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

// TODO вроде хартбиты не нужны, их заменяет log_request.
// func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
// 	heartbeatSender := r.RemoteAddr
// 	time := time.Now()
// 	fmt.Println("heartbeat recieved from", heartbeatSender, "at", time)
// 	mutex.Lock()
// 	if heartbeatSender == unimportantState.CurrentLeader {
// 		unimportantState.LastHeartbeat = time
// 	}
// 	mutex.Unlock()
// 	w.WriteHeader(http.StatusOK)
// }

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
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var voteResponse requests.VoteResponse
	err = json.Unmarshal(body, &voteResponse)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
	fmt.Printf("Received vote response: %+v\n", voteResponse)

	voterId := r.RemoteAddr
	mutex.Lock()
	if unimportantState.CurrentRole == nodestate.Candidate && voteResponse.Term == importantState.CurrentTerm && voteResponse.Granted {
		unimportantState.VotesRecieved[voterId] = struct{}{}
		if len(unimportantState.VotesRecieved) >= (len(allNodes)+1)/2 {
			unimportantState.CurrentRole = nodestate.Leader
			unimportantState.CurrentLeader = nodeId
			unimportantState.ElectionIteration++ // cancel election
			for _, follower := range nodesExceptMe {
				unimportantState.SentLength[follower] = len(importantState.Log)
				unimportantState.AckedLength[follower] = 0
				ReplicateLog(nodeId, follower)
			}
		} else if voteResponse.Term > importantState.CurrentTerm {
			importantState.CurrentTerm = voteResponse.Term
			unimportantState.CurrentRole = nodestate.Follower
			importantState.VotedFor = ""
			importantState.SaveToFile()
			unimportantState.ElectionIteration++ // cancel election
		}
	}

	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func logRequestHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var logRequest requests.LogRequest
	err = json.Unmarshal(body, &logRequest)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
	fmt.Printf("Received vote response: %+v\n", logRequest)

	leaderId := r.RemoteAddr
	mutex.Lock()
	if logRequest.Term > importantState.CurrentTerm {
		importantState.CurrentTerm = logRequest.Term
		importantState.VotedFor = ""
		unimportantState.CurrentRole = nodestate.Follower
		unimportantState.CurrentLeader = leaderId
	}
	if logRequest.Term == importantState.CurrentTerm && unimportantState.CurrentRole == nodestate.Candidate {
		unimportantState.CurrentRole = nodestate.Follower
		unimportantState.CurrentLeader = leaderId
	}
	logOk := (len(importantState.Log) >= logRequest.LogLength) &&
		(logRequest.LogLength == 0 || logRequest.LogTerm == importantState.Log[len(importantState.Log)-1].Term)
	if logRequest.Term == importantState.CurrentTerm && logOk {
		AppendEntries(logRequest.LogLength, logRequest.LeaderCommit, logRequest.Entries)
		ack := logRequest.LogLength + len(logRequest.Entries)
		requests.SendLogResponse(leaderId, importantState.CurrentTerm, ack, true)
	} else {
		requests.SendLogResponse(leaderId, importantState.CurrentTerm, 0, false)
	}

	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func acks(length int) int {
	result := 0
	for _, n := range allNodes {
		if unimportantState.AckedLength[n] >= length {
			result++
		}
	}
	return result
}

func findMaxReady(minAcks int) int {
	for i := len(importantState.Log); i >= 0; i-- {
		if acks(i) >= minAcks {
			return i
		}
	}
	return -1
}

func CommitLogEntries() {
	minAcks := (len(allNodes) + 2) / 2 // TODO точно +2? (+1 просто + округлуние вверх, поэтому еще +1). Короче зависит от того, учитываем ли себя
	maxReady := findMaxReady(minAcks)
	if maxReady > importantState.CommitLength && importantState.Log[maxReady-1].Term == importantState.CurrentTerm {
		for i := importantState.CommitLength; i < maxReady-1; i++ {
			deliverMessage(i)
		}
		importantState.CommitLength = maxReady
	}
}

func logResponseHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var logResponse requests.LogResponse
	err = json.Unmarshal(body, &logResponse)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
	fmt.Printf("Received vote response: %+v\n", logResponse)

	follower := r.RemoteAddr
	mutex.Lock()
	if logResponse.Term == importantState.CurrentTerm && unimportantState.CurrentRole == nodestate.Leader {
		if logResponse.Success && logResponse.Ack >= unimportantState.AckedLength[follower] {
			unimportantState.SentLength[follower] = logResponse.Ack
			unimportantState.AckedLength[follower] = logResponse.Ack
			CommitLogEntries()
		} else if unimportantState.SentLength[follower] > 0 { // TODO here maybe add "!logResponse.Success" (8 slide)
			unimportantState.SentLength[follower] = unimportantState.SentLength[follower] - 1 // TODO here maybe x2 increase
			ReplicateLog(nodeId, follower)
		}
	} else if logResponse.Term > importantState.CurrentTerm {
		importantState.CurrentTerm = logResponse.Term
		unimportantState.CurrentRole = nodestate.Follower
		importantState.VotedFor = ""
	}
	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func currentRoleHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	currentRole := unimportantState.CurrentRole
	mutex.Unlock()

	response := requests.CurrentRole{}
	if currentRole == nodestate.Leader {
		response.Role = "leader"
	} else if currentRole == nodestate.Candidate {
		response.Role = "candidate"
	} else {
		response.Role = "follower"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
	mutex.Unlock()
	w.WriteHeader(http.StatusOK)
}

func main() {
	fmt.Println("nodeId:", nodeId)
	importantState.CurrentTerm = 0
	importantState.VotedFor = ""
	importantState.Log = nil
	importantState.CommitLength = 0
	fmt.Println("initialized new state")
	importantState.SaveToFile()

	unimportantState.CurrentRole = nodestate.Follower
	unimportantState.CurrentLeader = ""
	unimportantState.VotesRecieved = nil
	unimportantState.SentLength = nil
	unimportantState.AckedLength = nil
	unimportantState.ElectionIteration = 0
	unimportantState.IsStopped = false

	go CheckLeaderFailurePeriodically()

	// user-called handlers
	http.HandleFunc("/read", readHandler)
	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/delete", deleteHandler)

	// handlers for node-to-node communication
	// http.HandleFunc("/heartbeat", heartbeatHandler)
	http.HandleFunc("/vote_request", voteRequestHandler)
	http.HandleFunc("/vote_response", voteResponseHandler)
	http.HandleFunc("/log_request", logRequestHandler)
	http.HandleFunc("/log_response", logResponseHandler)

	// handlers for testing
	http.HandleFunc("/current_role", currentRoleHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/recover", recoverHandler)

	if err := http.ListenAndServe(nodeId, nil); err != nil {
		log.Fatalf("could not start server: %s\n", err)
	}
}
