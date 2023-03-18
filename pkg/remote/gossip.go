package remote

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
)

// GossipEvents implements the memberlist.EventDelegate
// interface in order to receive notifications about members
// joining and leaving the cluster.
type GossipEvents struct {
	eventsCh chan Event
}

// NotifyJoin is invoked when a node is detected to have joined.
// The Node argument must not be modified.
func (ge *GossipEvents) NotifyJoin(node *memberlist.Node) {
	ge.eventsCh <- Event{
		Typ:  EventJoin,
		Name: node.Name,
		Addr: node.Addr,
		Port: node.Port,
	}
}

// NotifyLeave is invoked when a node is detected to have left.
// The Node argument must not be modified.
func (ge *GossipEvents) NotifyLeave(node *memberlist.Node) {
	ge.eventsCh <- Event{
		Typ:  EventLeave,
		Name: node.Name,
		Addr: node.Addr,
		Port: node.Port,
	}
}

// NotifyUpdate is invoked when a node is detected to have
// updated, usually involving the meta data. The Node argument
// must not be modified.
func (ge *GossipEvents) NotifyUpdate(node *memberlist.Node) {
	// Noop
}

const (
	GossiperNetworkLocal GossiperNetwork = iota
	GossiperNetworkLAN
	GossiperNetworkWAN
)

type GossiperNetwork int

type GossiperConfig struct {
	NodeName string
	NodeList []string
	Network  GossiperNetwork
	Port     int
}

type Gossiper struct {
	ml     *memberlist.Memberlist
	events *GossipEvents
}

// NewGossiper creates a new Gossiper which joins the cluster defined
// by the node list. Given wait group should not be incremented previously
// to this call, as it will be incremented in this constructor once the
// Gossiper is correctly initialized. This wait group can then be used
// to wait for the Gossiper to gracefully leave the cluster once the given
// context is canceled. Otherwise it will take more time for the cluster to
// realize that this node is down.
func NewGossiper(ctx context.Context, wg *sync.WaitGroup, config GossiperConfig) (*Gossiper, error) {
	events := &GossipEvents{
		// Set initial buffer to prevent blocking
		// the main thread meanwhile the consumer
		// of Gossiper starts reading the channel
		eventsCh: make(chan Event, 10),
	}

	// TODO: Move memberlist init away from constructor?
	var mlConfig *memberlist.Config
	switch config.Network {
	case GossiperNetworkWAN:
		mlConfig = memberlist.DefaultWANConfig()
	case GossiperNetworkLocal:
		mlConfig = memberlist.DefaultLocalConfig()
	default:
		mlConfig = memberlist.DefaultLANConfig()
	}

	mlConfig.Name = config.NodeName
	mlConfig.BindPort = config.Port
	mlConfig.AdvertisePort = config.Port
	mlConfig.Events = events

	ml, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating memberlist: %w", err)
	}

	_, err = ml.Join(config.NodeList)
	if err != nil {
		return nil, fmt.Errorf("error joining the cluster: %w", err)
	}

	wg.Add(1)

	go func() {
		<-ctx.Done()
		if err := ml.Leave(5 * time.Second); err != nil {
			log.Printf("error leaving memberlist cluster: %v", err)
		}
		wg.Done()
	}()

	return &Gossiper{
		ml:     ml,
		events: events,
	}, nil
}

func (g *Gossiper) EventsCh() <-chan Event {
	return g.events.eventsCh
}
