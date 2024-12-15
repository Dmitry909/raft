package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	fmt.Println(nodeId, ":", "suspect timeout:", suspectLeaderFailureTimeout)
	electionTimeout = suspectLeaderFailureTimeout / 4
	replicateTimeout = suspectLeaderFailureTimeout / 4
}

var importantState nodestate.ImportantState
var unimportantState nodestate.UnimportantState
var mutex sync.Mutex

func BecomeCandidateAndStartElection() { // mutex must be locked
	fmt.Println(nodeId, ":", "Called BecomeCandidateAndStartElection")
	importantState.CurrentTerm += 1
	unimportantState.CurrentRole = nodestate.Candidate
	importantState.VotedFor = nodeId
	unimportantState.VotesRecieved = make(map[string]struct{})
	unimportantState.VotesRecieved[nodeId] = struct{}{}
	unimportantState.SentLength = make(map[string]int)
	unimportantState.AckedLength = make(map[string]int)

	var lastTerm int = 0
	if len(importantState.Log) > 0 {
		lastTerm = importantState.Log[len(importantState.Log)-1].Term
	}
	termBeforeElection := importantState.CurrentTerm
	logLength := len(importantState.Log)
	importantState.SaveToFile(port)

	unimportantState.ElectionIteration++
	electionIterationBefore := unimportantState.ElectionIteration
	mutex.Unlock()

	for _, node := range allNodes {
		fmt.Println(nodeId, ":", "\tSending vote request to", node)
		requests.SendVoteRequest(nodeId, node, termBeforeElection, logLength, lastTerm)
	}

	sleepUntil := time.Now().Add(electionTimeout)
	go func() {
		time.Sleep(time.Until(sleepUntil))
		mutex.Lock()
		if unimportantState.ElectionIteration == electionIterationBefore && importantState.CurrentTerm == termBeforeElection && unimportantState.CurrentRole == nodestate.Candidate {
			// mutex must be locked before call
			BecomeCandidateAndStartElection()
		} else {
			mutex.Unlock()
		}
	}()
}

func CheckLeaderFailurePeriodically() {
	time.Sleep(suspectLeaderFailureTimeout)
	for {
		fmt.Println(nodeId, ":", "CheckLeaderFailurePeriodically")
		mutex.Lock()
		isReallyFollower := unimportantState.CurrentRole == nodestate.Follower // && unimportantState.CurrentLeader != ""
		lastHeartbeat := unimportantState.LastHeartbeat
		// fmt.Println(nodeId, ":", "\t", isReallyFollower, lastHeartbeat)
		suspectFailure := lastHeartbeat.Add(suspectLeaderFailureTimeout).Before(time.Now())

		if isReallyFollower && suspectFailure {
			fmt.Println(nodeId, ":", "\tsuspected leader failure at", time.Now())
			BecomeCandidateAndStartElection()
		} else {
			mutex.Unlock()
		}

		// fmt.Println(nodeId, ":", "\tisReallyFollower:", isReallyFollower)
		if isReallyFollower {
			time.Sleep(time.Until(lastHeartbeat.Add(suspectLeaderFailureTimeout)))
		} else {
			time.Sleep(suspectLeaderFailureTimeout)
		}
	}
}

func ReplicateLog(leaderId string, followerId string) { // mutex must be UNlocked
	mutex.Lock()
	i := unimportantState.SentLength[followerId]
	entries := importantState.Log[i:]
	prevLogTerm := 0
	if i > 0 {
		prevLogTerm = importantState.Log[i-1].Term
	}
	requests.SendLogRequest(nodeId, followerId, importantState.CurrentTerm, i, prevLogTerm, importantState.CommitLength, entries, &mutex) // TODO а здесь точно i, а не n-i?
}

func ReplicateLogPeriodically() {
	for {
		mutex.Lock()
		if unimportantState.CurrentRole == nodestate.Leader {
			mutex.Unlock()
			for _, follower := range nodesExceptMe {
				ReplicateLog(nodeId, follower)
			}
		} else {
			mutex.Unlock()
		}
		time.Sleep(replicateTimeout)
	}
}

func deliverMessage(i int) {
	fmt.Println(nodeId, ":", "\tDELIVERING MESSAGE. key:", importantState.Log[i].K, ", value:", importantState.Log[i].V)
}

func AppendEntries(logLength, leaderCommit int, entries []nodestate.LogEntry) { // mutex must be locked
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
	importantState.SaveToFile(port)
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(nodeId, ":", "readHandler called")
	key := r.URL.Query().Get("key")
	fmt.Println(nodeId, ":", "\tkey=", key)
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
	// if unimportantState.CurrentRole == nodestate.Leader {
	// 	mutex.Unlock()
	// 	randomNode := nodesExceptMe[rand.Intn(len(nodesExceptMe))]
	// 	http.Redirect(w, r, "http://"+randomNode+"/read?key="+key, http.StatusFound)
	// 	return
	// }

	value := ""
	for i := len(importantState.Log) - 1; i >= 0; i-- {
		if importantState.Log[i].K == key {
			value = importantState.Log[i].V
			break
		}
	}
	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}
	mutex.Unlock()
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
	fmt.Println(nodeId, ":", "updateHandler called")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var request requests.UpdateRequest
	err = json.Unmarshal(body, &request)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
	fmt.Printf("%s : \tReceived update request: %+v\n", nodeId, request)

	if request.Key == "" {
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
		http.Redirect(w, r, "http://"+unimportantState.CurrentLeader+"/read?key="+request.Key, http.StatusFound)
		mutex.Unlock()
		return
	}

	newEntry := nodestate.LogEntry{Term: importantState.CurrentTerm, OperatType: nodestate.Write, K: request.Key, V: request.Value}
	importantState.Log = append(importantState.Log, newEntry)
	importantState.SaveToFile(port)
	unimportantState.AckedLength[nodeId] = len(importantState.Log)
	mutex.Unlock()
	for _, follower := range nodesExceptMe {
		ReplicateLog(nodeId, follower)
	}

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
	importantState.SaveToFile(port)
	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func voteRequestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(nodeId, ":", "voteRequestHandler called")
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
	fmt.Printf("%s : \tReceived vote request: %+v\n", nodeId, c)

	cId := c.SenderAddress
	mutex.Lock()
	if cId == nodeId {
		fmt.Println(nodeId, ":", "\tcId == nodeId, vote for myself")
		currentTerm := importantState.CurrentTerm
		mutex.Unlock()
		requests.SendVoteResponse(nodeId, cId, currentTerm, true, nil)
		w.WriteHeader(http.StatusOK)
		return
	}

	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}

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
		unimportantState.LastHeartbeat = time.Now()
		importantState.VotedFor = cId
		importantState.SaveToFile(port)
		requests.SendVoteResponse(nodeId, cId, importantState.CurrentTerm, true, &mutex)
	} else {
		requests.SendVoteResponse(nodeId, cId, importantState.CurrentTerm, false, &mutex)
	}

	w.WriteHeader(http.StatusOK)
}

func voteResponseHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(nodeId, ":", "voteResponseHandler called")
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
	fmt.Printf("%s : \tReceived vote response: %+v\n", nodeId, voteResponse)

	voterId := voteResponse.SenderAddress
	fmt.Println(nodeId, ":", "\tvoterId: ", voterId)
	mutex.Lock()
	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}

	fmt.Println(nodeId, ":", "\tunimportantState.CurrentRole:", unimportantState.CurrentRole)
	fmt.Println(nodeId, ":", "\t", voteResponse.Term, importantState.CurrentTerm)
	if unimportantState.CurrentRole == nodestate.Candidate && voteResponse.Term == importantState.CurrentTerm && voteResponse.Granted {
		unimportantState.VotesRecieved[voterId] = struct{}{}
		fmt.Println(nodeId, ":", "\tnew len(unimportantState.VotesRecieved):", len(unimportantState.VotesRecieved))
		if len(unimportantState.VotesRecieved) >= (len(allNodes)+1)/2 {
			unimportantState.CurrentRole = nodestate.Leader
			unimportantState.CurrentLeader = nodeId
			unimportantState.ElectionIteration++ // cancel election
			for _, follower := range nodesExceptMe {
				unimportantState.SentLength[follower] = len(importantState.Log)
				unimportantState.AckedLength[follower] = 0
			}
			mutex.Unlock()
			for _, follower := range nodesExceptMe {
				ReplicateLog(nodeId, follower)
			}
			w.WriteHeader(http.StatusOK)
			return
		} else if voteResponse.Term > importantState.CurrentTerm {
			importantState.CurrentTerm = voteResponse.Term
			unimportantState.CurrentRole = nodestate.Follower
			importantState.VotedFor = ""
			importantState.SaveToFile(port)
			unimportantState.ElectionIteration++ // cancel election
		}
	}

	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func logRequestHandler(w http.ResponseWriter, r *http.Request) {
	// fmt.Println(nodeId, ":", "logRequestHandler called")
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
	// fmt.Printf("%s : \tReceived log request: %+v\n", nodeId, logRequest)

	leaderId := logRequest.SenderAddress
	mutex.Lock()
	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}

	unimportantState.LastHeartbeat = time.Now()
	if logRequest.Term > importantState.CurrentTerm {
		importantState.CurrentTerm = logRequest.Term
		importantState.VotedFor = ""
		unimportantState.CurrentRole = nodestate.Follower
		unimportantState.CurrentLeader = leaderId
		importantState.SaveToFile(port)
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
		requests.SendLogResponse(nodeId, leaderId, importantState.CurrentTerm, ack, true, &mutex)
	} else {
		requests.SendLogResponse(nodeId, leaderId, importantState.CurrentTerm, 0, false, &mutex)
	}

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

func CommitLogEntries() { // mutex must be locked
	minAcks := (len(allNodes) + 2) / 2 // TODO точно +2? (+1 просто + округлуние вверх, поэтому еще +1). Короче зависит от того, учитываем ли себя
	maxReady := findMaxReady(minAcks)
	if maxReady > importantState.CommitLength && importantState.Log[maxReady-1].Term == importantState.CurrentTerm {
		for i := importantState.CommitLength; i < maxReady-1; i++ {
			deliverMessage(i)
		}
		importantState.CommitLength = maxReady
		importantState.SaveToFile(port)
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
	// fmt.Printf("%s : Received log response: %+v\n", nodeId, logResponse)

	follower := logResponse.SenderAddress
	mutex.Lock()
	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}

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
		importantState.SaveToFile(port)
	}
	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func currentRoleHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	if unimportantState.IsStopped {
		mutex.Unlock()
		http.Error(w, "node is stopped", http.StatusForbidden)
		return
	}
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
	fmt.Println(nodeId, ":", "recoverHandler called")
	mutex.Lock()
	unimportantState.IsStopped = false
	if nodestate.CheckStateOnDisk(port) {
		importantState.LoadFromFile(port)
		fmt.Printf("%s : \tstate recovered after crash: %+v\n", nodeId, importantState)
	} else {
		importantState.CurrentTerm = 0
		importantState.VotedFor = ""
		importantState.Log = nil
		importantState.CommitLength = 0
		fmt.Println(nodeId, ":", "\tinitialized new state")
	}
	mutex.Unlock()
	w.WriteHeader(http.StatusOK)
}

func main() {
	fmt.Println(nodeId, ":", "nodeId:", nodeId)
	importantState.CurrentTerm = 0
	importantState.VotedFor = ""
	importantState.Log = nil
	importantState.CommitLength = 0
	fmt.Println(nodeId, ":", "initialized new state")

	unimportantState.CurrentRole = nodestate.Follower
	unimportantState.CurrentLeader = ""
	unimportantState.VotesRecieved = nil
	unimportantState.SentLength = nil
	unimportantState.AckedLength = nil
	unimportantState.ElectionIteration = 0
	unimportantState.IsStopped = false

	go CheckLeaderFailurePeriodically()
	go ReplicateLogPeriodically()

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
