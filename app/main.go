package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"raft/nodestate"
	"sync"
	"time"

	"github.com/jinzhu/copier"
)

var allNodes = [5]string{"localhost:8000", "localhost:8001", "localhost:8002", "localhost:8003", "localhost:8004"}
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
			break
		}
	}
	if !contains {
		log.Fatal("Wrong port " + port)
	}
}

var importantState nodestate.ImportantState
var unimportantState nodestate.UnimportantState
var mutex sync.Mutex

// type Server struct {
// 	role    Role
// 	listener net.Listener
// 	// timeout  time.Duration
// }

// const electionTime := 1s

// 	startElectionTimer()
// }

// func OnElectionTimer() {
// }

func BecomeCandidateAndStartElection() {
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
		suspectFailure := unimportantState.LastHeartbeat.Add(suspectLeaderFailureTimeout).Before(time.Now())
		if isReallyFollower && suspectFailure {
			fmt.Println("suspected leader failure at", time.Now())
			BecomeCandidateAndStartElection()
		} else {
			mutex.Unlock()
		}

		// fmt.Println("isReallyFollower:", isReallyFollower)
		if isReallyFollower {
			time.Sleep(time.Until(unimportantState.LastHeartbeat.Add(suspectLeaderFailureTimeout)))
		} else {
			time.Sleep(suspectLeaderFailureTimeout)
		}
	}
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

	time.Sleep(1000 * time.Second)

	// server := &Server{
	// 	state: Follower,
	// 	timeout:
	// }
}
