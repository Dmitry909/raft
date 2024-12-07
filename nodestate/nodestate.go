package nodestate

import (
	"encoding/gob"
	"os"
)

type LogEntry struct {
	Term    int
	Message string
}

type NodeState struct {
	CurrentTerm  int64
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

func (n *NodeState) SaveToFile() error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(n)
}

func (n *NodeState) LoadFromFile() error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	return decoder.Decode(n)
}
