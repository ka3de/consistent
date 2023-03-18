package consistent

import "github.com/ka3de/consistent/pkg/remote"

type opt func(*Consistent)

func WithReplicas(n uint) opt {
	return func(c *Consistent) {
		c.nReplicas = int(n)
	}
}

func WithHasher(h Hasher) opt {
	return func(c *Consistent) {
		c.hasher = h
	}
}

func WithRemote(r remote.Remoter) opt {
	return func(c *Consistent) {
		c.remote = r
	}
}
