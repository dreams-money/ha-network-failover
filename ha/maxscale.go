package ha

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dreams-money/failover/config"
)

type MaxScale struct{}

var (
	maxScaleHttpClient = &http.Client{
		Timeout: 5 * time.Second,
	}
)

type apiServerAttributes struct {
	State string `json:"state"`
}
type apiServer struct {
	ID         string              `json:"id"`
	Attributes apiServerAttributes `json:"attributes"`
}
type apiServerResponse struct {
	Data []apiServer `json:"data"`
}

func (MaxScale) GetClusterStatus(cfg config.Config) (ClusterStatus, error) {
	url := cfg.HighAvailabilityAPIAddress + "/v1/servers"
	status := ClusterStatus{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return status, err
	}

	resp, err := maxScaleHttpClient.Do(req)
	if err != nil {
		return status, err
	}

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return status, err
	}

	if resp.StatusCode != 200 {
		e := "maxscale: api server request failed. (%v) %v"
		return status, fmt.Errorf(e, resp.StatusCode, string(respBodyBytes))
	}

	asr := apiServerResponse{}
	err = json.Unmarshal(respBodyBytes, &asr)
	if err != nil {
		return status, err
	}

	for _, server := range asr.Data {
		node, err := parseNode(server)
		if err != nil {
			return status, err
		}
		status = append(status, node)
	}

	return status, nil
}

func parseNode(server apiServer) (Node, error) {
	node := Node{}
	node.Name = server.ID

	nodeStates := strings.Split(server.Attributes.State, ", ")

	if len(nodeStates) < 1 {
		node.State = NODE_DOWN
		return node, nil
	}

	nodeRole := nodeStates[0]
	switch nodeRole {
	case "Master":
		nodeRole = NODE_PRIMARY
	case "Slave":
		nodeRole = NODE_REPLICA
	default:
		return node, errors.New("unknown node role: " + nodeRole)
	}
	node.Role = nodeRole

	if len(nodeStates) < 2 && nodeStates[1] == "Running" {
		node.State = NODE_RUNNING
		log.Println("node possibily not synced: " + node.Name)
		return node, nil
	}

	nodeSynced := nodeStates[1]
	nodeStatus := nodeStates[2]

	node.State = NODE_DOWN
	if nodeStatus == "Running" && nodeSynced == "Synced" {
		node.State = NODE_RUNNING
	} else if nodeStatus == "Running" || nodeSynced == "Running" {
		log.Println("node not synced: " + node.Name)
		node.State = NODE_RUNNING
	}

	return node, nil
}
