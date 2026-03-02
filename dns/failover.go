package dns

import (
	"errors"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/dreams-money/failover/config"
	"github.com/dreams-money/failover/ha"
)

var client = mustMakeAWSClient()

func Failover(cfg config.Config, clusterStatus ha.ClusterStatus) error {
	leader, err := clusterStatus.GetLeaderName()
	if err != nil {
		return err
	} else if leader == "" {
		return errors.New("empty leader")
	} else if leader != cfg.NodeName { // Only primary node updates DNS
		return nil
	}

	publicIp, err := getPublicIP()
	if err != nil {
		return err
	}

	err = updatePrimary(cfg, client, publicIp)
	if err != nil {
		return err
	}

	return updateReplicas(cfg, client, clusterStatus)
}

func updatePrimary(cfg config.Config, client *route53.Client, ip string) error {
	_, err := upsertDNSRecord(client, cfg.DNSPrimary, ip, "")
	if err != nil {
		return err
	}

	log.Println("Successfully updated primary DNS: " + cfg.DNSPrimary)

	return nil
}

func updateReplicas(cfg config.Config, client *route53.Client, clusterStatus ha.ClusterStatus) error {
	// Delete existing records
	_, err := deleteExistingWeightedDNSRecords(client, cfg.DNSReplica)
	if err == errNoWeightedRecords {
		log.Println("Replica DNS:", errNoWeightedRecords)
	} else if err != nil {
		return err
	}

	// Add replica records
	replicas, err := clusterStatus.GetActiveReplicas()
	if err != nil {
		return err
	}

	var setErr error
	for _, nodeName := range replicas {
		peer, err := cfg.GetPeer(nodeName)
		if err != nil {
			setErr = errors.Join(setErr, err)
			continue
		}

		ip, err := resolveDNSRecord(peer.DDNSAddress)
		if err != nil {
			setErr = errors.Join(setErr, err)
			continue
		}

		_, err = upsertDNSWeightedRecord(client, DNSWeightedRecord{
			Name:          cfg.DNSReplica,
			SetIdentifier: nodeName,
			Weight:        peer.ReplicaWeight,
			IP:            ip,
		}, "DNS replica for: "+nodeName)
		if err != nil {
			setErr = errors.Join(setErr, err)
		}
	}

	if setErr != nil {
		return setErr
	}

	log.Println("Successfully updated replica DNS: " + cfg.DNSReplica)

	return nil
}
