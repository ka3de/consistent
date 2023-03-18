package consistent

import (
	"errors"
	"log"
	"sort"
	"strconv"
	"sync"

	"github.com/ka3de/consistent/pkg/remote"
)

const (
	defNReplicas = 20
)

var (
	// ErrNoSrvs indicates that the consistent hashing ring has no servers yet.
	ErrNoSrvs = errors.New("ring has no servers")
	// ErrSrvAlreadyExists indicates that the given server already exists in the ring.
	ErrSrvAlreadyExists = errors.New("server already exists in the ring")
	// ErrSrvNotExists indicates that the given server is not present in the ring.
	ErrSrvNotExists = errors.New("server is not present in the ring")
)

type Hash uint32

// Hasher represents a hashing interface with a 32 bits output.
type Hasher interface {
	Hash(key string) Hash
}

// Consistent represents a consistent hashing ring.
type Consistent struct {
	mu sync.RWMutex

	members map[string]struct{}
	ring    map[Hash]string
	hashes  []Hash

	hasher Hasher

	nReplicas int

	remote remote.Remoter
}

// NewConsistent creates a new consistent hashing ring representation.
func NewConsistent(opts ...opt) *Consistent {
	r := &Consistent{
		members:   make(map[string]struct{}),
		ring:      make(map[Hash]string),
		hasher:    NewCRCHasher(), // default
		nReplicas: defNReplicas,
	}

	for _, o := range opts {
		o(r)
	}

	if r.isRemoteEnabled() {
		r.handleRemote()
	}

	return r
}

// Add adds a new server to the ring.
// If the server is already present in the ring returns ErrSrvAlreadyExists.
func (c *Consistent) Add(srv string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.members[srv]; ok {
		return ErrSrvAlreadyExists
	}

	c.members[srv] = struct{}{}

	for i := 0; i < c.nReplicas; i++ {
		hash := c.hasher.Hash(c.srvKey(srv, i))
		c.hashes = append(c.hashes, hash)
		c.ring[hash] = srv
	}

	c.sortHashes()

	return nil
}

// Remove deletes the given server from the ring.
// If the server does not exist returns ErrSrvNotExists.
func (c *Consistent) Remove(srv string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.members[srv]; !ok {
		return ErrSrvNotExists
	}

	delete(c.members, srv)

	for i := 0; i < c.nReplicas; i++ {
		delete(c.ring, c.hasher.Hash(c.srvKey(srv, i)))
	}

	c.updateHashes()

	return nil
}

// Get returns the associated server in the ring for the given key.
// If the ring has no servers returns ErrNoSrvs.
func (c *Consistent) Get(key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.count() == 0 {
		return "", ErrNoSrvs
	}

	idx := c.search(c.hasher.Hash(key))
	return c.ring[c.hashes[idx]], nil
}

// search returns the position in the ring (hashes index)
// for the given hash h.
// Consistent lock must be held before calling this method.
func (c *Consistent) search(h Hash) int {
	idx := sort.Search(len(c.hashes), func(i int) bool {
		return c.hashes[i] > h // Look for next ring elem clockwise
	})
	if idx == len(c.hashes) {
		return 0
	}
	return idx
}

// updateHashes updates the consistent hashes slice based on the current
// ring members and sorts them in ascending order.
// Consistent lock must be held before calling this method.
func (c *Consistent) updateHashes() {
	c.hashes = c.hashes[:0]
	// If underlying array capacity is bigger than 4 times
	// the number of servers in the ring, reallocate
	if cap(c.hashes)/(c.nReplicas) > 4*c.count() {
		c.hashes = nil
	}

	for h := range c.ring {
		c.hashes = append(c.hashes, h)
	}
	c.sortHashes()
}

// sortHashes sorts the consistent hashes slice asc.
// Consistent lock must be held before calling this method.
func (c *Consistent) sortHashes() {
	sort.Slice(c.hashes, func(i int, j int) bool {
		return c.hashes[i] < c.hashes[j]
	})
}

func (c *Consistent) srvKey(srv string, i int) string {
	return srv + strconv.Itoa(i)
}

func (c *Consistent) count() int {
	return len(c.members)
}

func (c *Consistent) isRemoteEnabled() bool {
	return c.remote != nil
}

func (c *Consistent) handleRemote() {
	eventsCh := c.remote.EventsCh()

	go func() {
		for e := range eventsCh {
			switch e.Typ {
			case remote.EventJoin:
				if err := c.Add(e.Name); err != nil {
					c.handleRcvErr(e, err)
				}
			case remote.EventLeave:
				if err := c.Remove(e.Name); err != nil {
					c.handleRcvErr(e, err)
				}
			default:
				log.Printf("warning: unknown event type: %v", e.Typ)
			}
		}
	}()
}

type Snapshot struct {
	Members map[string][]Hash
}

func (c *Consistent) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	members := make(map[string][]Hash)

	for h, m := range c.ring {
		if _, ok := members[m]; !ok {
			members[m] = []Hash{}
		}
		members[m] = append(members[m], h)
	}

	return Snapshot{
		Members: members,
	}
}

func (c *Consistent) handleRcvErr(e remote.Event, err error) {
	log.Printf("error processing remote event %v: %v", e, err)
}
