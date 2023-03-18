# consistent

Simple playground project which implements Consistent Hashing using a gossip protocol in order to propagate cluster membership changes.

Consistent uses Hashicorp's [memberlist](https://github.com/hashicorp/memberlist) library in order to manage cluster membership and member failure detection, which is based on [SWIM](https://ieeexplore.ieee.org/document/1028914) protocol.
