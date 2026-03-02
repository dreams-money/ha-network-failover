package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dreams-money/failover/config"
	"github.com/dreams-money/failover/dns"
	"github.com/dreams-money/failover/ha"
	"github.com/dreams-money/failover/health"
	"github.com/dreams-money/failover/routers"
)

var CurrentStatus ha.ClusterStatus

func main() {
	// Load config
	configuration, err := config.LoadProgramConfiguration()
	if err != nil {
		log.Println(err)
		os.Exit(1)
		return
	}

	router, err := routers.Make(configuration)
	if err != nil {
		log.Println(err)
		os.Exit(1)
		return
	}

	// Set Router Auth
	router.SetAuthorization(configuration)
	err = router.SimpleCall(configuration)
	if err != nil {
		log.Println("Authorization failed", err)
		os.Exit(1)
		return
	}

	// Get High Avaliability Provider
	provider, err := ha.MakeProvider(configuration.HighAvailabilityProvider)
	if err != nil {
		log.Println("Failed to create high avaliability provider", err)
		os.Exit(1)
		return
	}

	// Failover endpoint
	http.HandleFunc("/failover", func(w http.ResponseWriter, r *http.Request) {
		clusterStatus, err := provider.GetClusterStatus(configuration)
		if err != nil {
			log.Println("Failed to get cluster status", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		leader, err := clusterStatus.GetLeaderName()
		if err != nil {
			log.Println("Failed to get leader name", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		routerErr := router.Failover(configuration, leader)
		if routerErr != nil {
			log.Println("Router failover failed", routerErr)
		}

		dnsErr := dns.Failover(configuration, clusterStatus)
		if dnsErr != nil {
			log.Println("DNS failover failed", dnsErr)
		}

		if routerErr != nil || dnsErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		CurrentStatus = clusterStatus.Copy()

		w.WriteHeader(http.StatusOK)
	})

	// Health check endpoint
	http.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		err = router.SimpleCall(configuration)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	// Loop to check peers
	go startCheckPeerJob(configuration)

	log.Println("Server listening on http://localhost:" + configuration.AppPort)
	err = http.ListenAndServe(":"+configuration.AppPort, nil)
	if err != nil {
		log.Printf("Error starting server: %s\n", err)
		os.Exit(1)
		return
	}
}

func startCheckPeerJob(cfg config.Config) {
	heartBeatInterval := time.Tick(cfg.HeartBeatInterval)

	for range heartBeatInterval {
		checkPeers(cfg)
	}
}

func checkPeers(cfg config.Config) {
	var err error
	for peer, config := range cfg.Peers {
		if !config.CheckHealth {
			continue
		}

		err = health.CheckPeer(peer, config.Address+"/heartbeat")
		if err != nil {
			log.Println("Health check failed to execute!", err)
		}
	}
}
