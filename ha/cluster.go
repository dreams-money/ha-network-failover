package ha

import (
	"errors"
	"slices"
)

type Node struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	State string `json:"state"`
}

const (
	NODE_PRIMARY = "primary"
	NODE_REPLICA = "replica"
	NODE_RUNNING = "running"
	NODE_DOWN    = "down"
)

type ClusterStatus []Node

func (s ClusterStatus) GetLeaderName() (string, error) {
	for _, node := range s {
		if node.Role == NODE_PRIMARY && node.State == NODE_RUNNING {
			return node.Name, nil
		}
	}

	return "", errors.New("leader not found")
}

func (s ClusterStatus) GetActiveReplicas() ([]string, error) {
	var replicas []string

	for _, node := range s {
		if node.Role == NODE_REPLICA && node.State == NODE_RUNNING {
			replicas = append(replicas, node.Name)
		}
	}

	if len(replicas) < 1 {
		return nil, errors.New("no replicas")
	}

	return replicas, nil
}

func (a ClusterStatus) HasChanged(b ClusterStatus) bool {
	if len(a) != len(b) {
		return true
	}

	for _, aNode := range a {
		if !slices.Contains(b, aNode) {
			return true
		}
	}

	return false
}

func (original ClusterStatus) Copy() ClusterStatus {
	cpy := make(ClusterStatus, len(original))
	copy(cpy, original)

	return cpy
}
