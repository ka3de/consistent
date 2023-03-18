package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/ka3de/consistent"
	"github.com/ka3de/consistent/pkg/remote"
)

func main() {
	var (
		nodeName    string
		apiPort     int
		nodePort    int
		nodeList    string
		nodeListDNS string
		network     string
	)

	flag.StringVar(&nodeName, "name", rndNodeName(), "node name")
	flag.IntVar(&apiPort, "api-port", rndPort(), "http API port to bind to")
	flag.IntVar(&nodePort, "port", rndPort(), "gossip node port to bind to")
	flag.StringVar(&nodeList, "nodelist", "", "gossip node list to join to")
	flag.StringVar(&nodeListDNS, "nodelist-dns", "", "DNS name to resolve gossip node list from")
	flag.StringVar(&network, "network", "LOCAL", "network type on which cluster operates in. possible values are: LOCAL, LAN, WAN")

	flag.Parse()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Build Consistent
	nodes, err := parseNodeList(nodeList, nodeListDNS, nodePort)
	if err != nil {
		log.Fatalf("error parsing node list: %v", err)
	}

	log.Printf("starting node with name: %s port: %d", nodeName, nodePort)
	gNetwork, err := parseNetwork(network)
	if err != nil {
		log.Fatalf("error parsing network: %v", err)
	}
	gConfig := remote.GossiperConfig{
		NodeName: nodeName,
		NodeList: nodes,
		Network:  gNetwork,
		Port:     nodePort,
	}
	g, err := remote.NewGossiper(ctx, &wg, gConfig)
	if err != nil {
		log.Fatalf("error building remote gossiper: %v", err)
	}

	c := consistent.NewConsistent(consistent.WithRemote(g))

	// Build HTTP API
	log.Printf("starting node HTTP API with port: %d", apiPort)
	http.HandleFunc("/consistent/snapshot", ringHandler(nodeName, c))
	http.HandleFunc("/consistent/srv", srvHandler(nodeName, c))

	go func() {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", apiPort), nil))
		close(stop)
	}()

	<-stop
	cancel()
	wg.Wait()
}

type ringResponse struct {
	From string                       `json:"from"`
	Srvs map[string][]consistent.Hash `json:"srvs"`
}

func ringHandler(srv string, c *consistent.Consistent) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		body, err := json.Marshal(ringResponse{
			From: srv,
			Srvs: c.Snapshot().Members,
		})
		if err != nil {
			w.Write([]byte(fmt.Sprintf("error marshaling response: %v", err))) //nolint:errcheck
			w.WriteHeader(http.StatusInternalServerError)
		}

		_, err = w.Write(body)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("error writing response: %v", err))) //nolint:errcheck
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

type srvResponse struct {
	From string `json:"from"`
	Srv  string `json:"srv"`
}

func srvHandler(srv string, c *consistent.Consistent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			w.WriteHeader(http.StatusBadRequest)
		}

		srv, err := c.Get(key)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("error retrieving server: %v", err))) //nolint:errcheck
			w.WriteHeader(http.StatusInternalServerError)
		}

		body, err := json.Marshal(srvResponse{
			From: srv,
			Srv:  srv,
		})
		if err != nil {
			w.Write([]byte(fmt.Sprintf("error marshaling response: %v", err))) //nolint:errcheck
			w.WriteHeader(http.StatusInternalServerError)
		}

		_, err = w.Write(body)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("error writing response: %v", err))) //nolint:errcheck
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func parseNodeList(nodeList, nodeListDNS string, nodePort int) ([]string, error) {
	if nodeListDNS != "" {
		return resolveNodeList(nodeListDNS, nodePort)
	}

	nodes := strings.Split(nodeList, ",")
	if len(nodes) > 0 && nodes[0] == "" {
		return nil, nil
	}

	return nodes, nil
}

func resolveNodeList(nodeListDNS string, nodePort int) ([]string, error) {
	nodeIPs, err := net.LookupIP(nodeListDNS)
	if err != nil {
		return nil, fmt.Errorf("error resolving DNS name: %w", err)
	}

	return ipsToStr(nodeIPs, nodePort), nil
}

func ipsToStr(ips []net.IP, port int) []string {
	ss := make([]string, len(ips))
	for i, ip := range ips {
		ss[i] = fmt.Sprintf("%s:%d", ip.To4().String(), port)
	}

	return ss
}

func parseNetwork(network string) (gNetwork remote.GossiperNetwork, err error) {
	switch strings.ToUpper(network) {
	case "LOCAL":
		gNetwork = remote.GossiperNetworkLocal
	case "LAN":
		gNetwork = remote.GossiperNetworkLAN
	case "WAN":
		gNetwork = remote.GossiperNetworkWAN
	default:
		err = fmt.Errorf("invalid network: %s", network)
	}

	return
}

func rndNodeName() string {
	return fmt.Sprintf("node-%d", rndBound(0, 100))
}

func rndPort() int {
	return rndBound(5000, 6000)
}

func rndBound(min, max int) int {
	return rand.Intn(max-min+1) + min
}
