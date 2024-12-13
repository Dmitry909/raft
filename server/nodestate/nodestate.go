package nodestate

import (
	"encoding/gob"
	"os"
	"time"
)

type OperationType int

const (
	Write OperationType = iota
	Delete
)

type LogEntry struct {
	Term       int
	OperatType OperationType
	K          string
	V          string
}

type ImportantState struct {
	CurrentTerm  int
	VotedFor     string
	Log          []LogEntry
	CommitLength int
}

var filepath string = "state"

func CheckStateOnDisk() bool {
	info, err := os.Stat(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return !info.IsDir()
}

func (n *ImportantState) SaveToFile() error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(n)
}

func (n *ImportantState) LoadFromFile() error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	return decoder.Decode(n)
}

type Role int

const (
	Leader Role = iota
	Candidate
	Follower
)

type UnimportantState struct {
	CurrentRole   Role
	CurrentLeader string
	VotesRecieved map[string]struct{}
	SentLength    map[string]int
	AckedLength   map[string]int

	LastHeartbeat     time.Time
	ElectionIteration int

	IsStopped bool
}
