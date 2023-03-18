package remote

import "net"

const (
	EventJoin EventType = iota
	EventLeave
)

type EventType int

type Event struct {
	Typ  EventType
	Name string
	Addr net.IP
	Port uint16
}

type Remoter interface {
	EventsCh() <-chan Event
}
