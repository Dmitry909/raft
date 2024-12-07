package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"raft/nodestate"
	"sync"
	"time"

	"github.com/jinzhu/copier"
)

var allNodes = [5]string{"localhost:8000", "localhost:8001", "localhost:8002", "localhost:8003", "localhost:8004"}
var nodesExceptMe = []string{}
var port string
var nodeId string
var suspectLeaderFailureTimeout time.Duration

func init() {
	port = os.Args[1]
	nodeId = "localhost:" + port
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

// 	startElectionTimer()
// }

// func OnElectionTimer() {
// }

func BecomeCandidateAndStartElection() { // mutex must be locked
	importantState.CurrentTerm += 1
	unimportantState.CurrentRole = nodestate.Candidate
	importantState.VotedFor = nodeId
	unimportantState.VotesRecieved = map[string]struct{}{}
	unimportantState.VotesRecieved[nodeId] = struct{}{}

	importantStateCopy := nodestate.ImportantState{}
	copier.Copy(&importantStateCopy, &importantState)

	mutex.Unlock()

	importantStateCopy.SaveToFile()
	// lastTerm := 0
	// if len(importantStateCopy.Log) > 0 {
	// 	lastTerm = importantStateCopy.Log[len(importantStateCopy.Log)-1].Term
	// }
	// for node := range allNodes {
	// 	// send http request here
	// }
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

	go CheckLeaderFailurePeriodically()

	http.HandleFunc("/read", readHandler)
	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/delete", deleteHandler)

	if err := http.ListenAndServe(nodeId, nil); err != nil {
		log.Fatalf("could not start server: %s\n", err)
	}
}
