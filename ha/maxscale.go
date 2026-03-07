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
			log.Println(server.ID, err)
		}
		status = append(status, node)
	}

	return status, nil
}

func parseNode(server apiServer) (Node, error) {
	node := Node{}
	node.Name = server.ID

	column := strings.Split(server.Attributes.State, ", ")

	if len(column) < 1 {
		node.State = NODE_DOWN
		return node, nil
	}

	switch column[0] {
	case "Master":
		node.State = NODE_RUNNING
		node.Role = NODE_PRIMARY
	case "Slave":
		node.State = NODE_RUNNING
		node.Role = NODE_REPLICA
	case "Down":
		node.State = NODE_DOWN
		return node, nil
	default:
		return node, errors.New("unknown node state: " + column[0])
	}

	if len(column) > 1 && column[1] != "Synced" {
		log.Printf("Node %v state is: %v\n", node.Name, column[1])
	}

	return node, nil
}
