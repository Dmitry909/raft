package main

import (
	"fmt"
	"log"
	"os"
	"raft/nodestate"
)

var allNodes = [5]string{"localhost:8000", "localhost:8001", "localhost:8002", "localhost:8003", "localhost:8004"}
var port string
var nodeId string

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

var state nodestate.NodeState

// type Role int

// const (
// 	Leader Role = iota
// 	Candidate
// 	Follower
// )

// type Server struct {
// 	role    Role
// 	listener net.Listener
// 	// timeout  time.Duration
// }

// var nodes = {"1.2.3.4:6000", "2.3.4.5:6000", "3.4.5.6:6000"}

// type LogMessage struct {
// 	term int64
// 	message string
// }

// type NodeState struct {
// 	CurrentTerm int
// 	VotedFor string
// 	Log []string
// 	CommitLength int
// }

// func ReadFromDisk() NodeState {
// 	// TODO
// 	return NodeState{}
// }

// func WriteToDisk(NodeState state) {
// 	// TODO
// }

// var currentRole Role := Follower
// var currentLeader string := node
// var votesReceived = {} // set
// var sentLength := 0
// var ackedLength := 0

// var isElectionNow := false;

// const electionTime := 1s
// const nodeId := "1.2.3.4:6000"

// func BecomeCandidateAndStartElection() {
// 	CurrentTerm++
// 	currentRole = Candidate
// 	VotedFor = nodeId
// 	votesReceived = {nodeId}

// 	lastTerm := len(Log) > 0 : Log.back().term ? 0
// 	msg := (VoteRequest, nodeId, CurrentTerm, len(Log), lastTerm)
// 	for (node : nodes) {
// 		node.send(msg)
// 	}

// 	startElectionTimer()
// }

// func OnElectionTimer() {

// }

// func onStartUp() {
// 	num_messages := 0;
// 	wait_messages_timeout := rand(1s, 5s);
// 	wait_messages(wait_messages_timeout, &num_messages);
// 	if (num_messages == 0) {
// 		becomeCandidate();
// 	}
// }

func main() {
	fmt.Println("nodeId:", nodeId)
	if nodestate.CheckStateOnDisk() {
		state.LoadFromFile()
		fmt.Printf("state recovered after crash: %+v\n", state)
	} else {
		state.CurrentTerm = 0
		state.VotedFor = ""
		state.Log = nil
		state.CommitLength = 0
		fmt.Println("initialized new state")
	}
	state.SaveToFile()

	// server := &Server{
	// 	state: Follower,
	// 	timeout:
	// }

	// state := nodestate.NodeState{
	// 	CurrentTerm:  0,
	// 	VotedFor:     "dSADSFGDG",
	// 	Log:          []nodestate.LogEntry{{1, "sgdg"}, {4, "fadafeaf"}},
	// 	CommitLength: 10,
	// }

	// err := state.SaveToFile("node_state.gob")
	// if err != nil {
	// 	log.Fatalf("Ошибка при сохранении: %v", err)
	// }

	// var loadedState nodestate.NodeState
	// err = loadedState.LoadFromFile("node_state.gob")
	// if err != nil {
	// 	log.Fatalf("Ошибка при загрузке: %v", err)
	// }
	// fmt.Printf("Загруженное состояние: %+v\n", loadedState)
}
