package consistent

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

type checker interface {
	name() string
	check(c *Consistent) bool
}

type membersNotNilChecker struct{}

func membersNotNilCheck() checker {
	return &membersNotNilChecker{}
}

func (ch *membersNotNilChecker) name() string {
	return structName(ch)
}

func (ch *membersNotNilChecker) check(c *Consistent) bool {
	return c.members != nil
}

type ringNotNilChecker struct{}

func ringNotNilCheck() checker {
	return &ringNotNilChecker{}
}

func (ch *ringNotNilChecker) name() string {
	return structName(ch)
}

func (ch *ringNotNilChecker) check(c *Consistent) bool {
	return c.ring != nil
}

type hasherEqToChecker struct {
	h Hasher
}

func hasherEqToCheck(h Hasher) checker {
	return &hasherEqToChecker{h}
}

func (ch *hasherEqToChecker) name() string {
	return structName(ch)
}

func (ch *hasherEqToChecker) check(c *Consistent) bool {
	return reflect.DeepEqual(c.hasher, ch.h)
}

type nReplicasEqToChecker struct {
	n int
}

func nReplicasEqToCheck(n int) checker {
	return &nReplicasEqToChecker{n}
}

func (ch *nReplicasEqToChecker) name() string {
	return structName(ch)
}

func (ch *nReplicasEqToChecker) check(c *Consistent) bool {
	return c.nReplicas == ch.n
}

func structName(s any) string {
	return reflect.TypeOf(s).String()
}

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		opts     []opt
		checkers []checker
	}{
		{
			name: "should create new default consistent",
			checkers: []checker{
				membersNotNilCheck(),
				ringNotNilCheck(),
				hasherEqToCheck(&CRCHasher{}),
				nReplicasEqToCheck(defNReplicas),
			},
		},
		{
			name: "should create consistent with 30 n replicas",
			opts: []opt{
				WithReplicas(30),
			},
			checkers: []checker{
				membersNotNilCheck(),
				ringNotNilCheck(),
				nReplicasEqToCheck(30),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := NewConsistent(tc.opts...)

			for _, checker := range tc.checkers {
				if ok := checker.check(c); !ok {
					t.Fatalf("error verifying check: %s", checker.name())
				}
			}
		})
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		c              *Consistent
		srvs           []string
		wantMembersLen int
		wantRingLen    int
		wantHashesLen  int
		wantErr        error
	}{
		{
			name: "should add two servers",
			c:    newTestC(t, 0),
			srvs: []string{
				"srv0",
				"srv1",
			},
			wantMembersLen: 2,
			wantRingLen:    2 * defNReplicas,
			wantHashesLen:  2 * defNReplicas,
		},
		{
			name:           "should return error srv already exists",
			c:              newTestC(t, 2),
			srvs:           []string{"srv1"},
			wantMembersLen: 2,
			wantRingLen:    2 * defNReplicas,
			wantHashesLen:  2 * defNReplicas,
			wantErr:        ErrSrvAlreadyExists,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, s := range tc.srvs {
				if err := tc.c.Add(s); !errors.Is(err, tc.wantErr) {
					t.Fatalf("unexpected error. want: %v but got: %v", tc.wantErr, err)
				}
			}

			checkC(t, tc.c, tc.wantMembersLen, tc.wantRingLen, tc.wantHashesLen)
		})
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		c              *Consistent
		srvs           []string
		wantMembersLen int
		wantRingLen    int
		wantHashesLen  int
		wantErr        error
	}{
		{
			name:           "should remove one srv",
			c:              newTestC(t, 2),
			srvs:           []string{"srv1"},
			wantMembersLen: 1,
			wantRingLen:    1 * defNReplicas,
			wantHashesLen:  1 * defNReplicas,
		},
		{
			name:           "should remove two srv",
			c:              newTestC(t, 2),
			srvs:           []string{"srv0", "srv1"},
			wantMembersLen: 0,
			wantRingLen:    0,
			wantHashesLen:  0,
		},
		{
			name:           "should return error server is not present in the ring",
			c:              newTestC(t, 2),
			srvs:           []string{"srv2"},
			wantMembersLen: 2,
			wantRingLen:    2 * defNReplicas,
			wantHashesLen:  2 * defNReplicas,
			wantErr:        ErrSrvNotExists,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, s := range tc.srvs {
				if err := tc.c.Remove(s); !errors.Is(err, tc.wantErr) {
					t.Fatalf("unexpected error. want: %v but got: %v", tc.wantErr, err)
				}
			}

			checkC(t, tc.c, tc.wantMembersLen, tc.wantRingLen, tc.wantHashesLen)
		})
	}
}

type mockHasher struct {
	c uint32
}

func (mh *mockHasher) Hash(key string) Hash {
	if key == "first" {
		return Hash(mh.c - 1) // Last hash returned will point to first elem in Consistent hashes
	}
	if key == "last" {
		return Hash(mh.c - 2) // Second to last hash returned will point to last elem in Consistent hashes
	}

	h := mh.c
	mh.c++
	return Hash(h)
}

func TestGet(t *testing.T) {
	t.Parallel()

	type key2srv struct {
		key string
		srv string
	}

	testCases := []struct {
		name           string
		c              *Consistent
		keys2srv       []key2srv
		wantMembersLen int
		wantRingLen    int
		wantHashesLen  int
		wantErr        error
	}{
		{
			name: "should get one srv",
			c:    newTestC(t, 1),
			keys2srv: []key2srv{
				{
					"any",
					"srv0",
				},
			},
			wantMembersLen: 1,
			wantRingLen:    1 * defNReplicas,
			wantHashesLen:  1 * defNReplicas,
		},
		{
			name: "should get two srvs",
			c:    newTestC(t, 2, WithHasher(&mockHasher{})),
			keys2srv: []key2srv{
				{
					key: "first",
					srv: "srv0",
				},
				{
					key: "last",
					srv: "srv1",
				},
			},
			wantMembersLen: 2,
			wantRingLen:    2 * defNReplicas,
			wantHashesLen:  2 * defNReplicas,
		},
		{
			name: "should return error rin has no servers",
			c:    newTestC(t, 0),
			keys2srv: []key2srv{
				{
					key: "any",
				},
			},
			wantMembersLen: 0,
			wantRingLen:    0,
			wantHashesLen:  0,
			wantErr:        ErrNoSrvs,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, ks := range tc.keys2srv {
				s, err := tc.c.Get(ks.key)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("unexpected error. want: %v but got: %v", tc.wantErr, err)
				}
				if s != ks.srv {
					t.Fatalf("expected srv for key:%s to be: %s but got: %s", ks.key, ks.srv, s)
				}
			}

			checkC(t, tc.c, tc.wantMembersLen, tc.wantRingLen, tc.wantHashesLen)
		})
	}
}

// newTestC creates a new Consistent for testing purposes.
func newTestC(t *testing.T, nSrvs int, opts ...opt) *Consistent {
	t.Helper()

	c := NewConsistent(opts...)

	for i := 0; i < nSrvs; i++ {
		if err := c.Add(fmt.Sprintf("srv%d", i)); err != nil {
			t.Fatalf("error creating new test Consistent: %v", err)
		}
	}

	return c
}

// checkC verifies that the elements of a Consistent c match the input parameters and that its
// hashes are sorted.
func checkC(t *testing.T, c *Consistent, wantMembersLen, wantRingLen, wantHashesLen int) {
	t.Helper()

	if membersLen := len(c.members); membersLen != wantMembersLen {
		t.Fatalf("expected members len to be %d, but got %d", wantMembersLen, membersLen)
	}
	if ringLen := len(c.ring); ringLen != wantRingLen {
		t.Fatalf("expected ring len to be %d, but got %d", wantMembersLen, ringLen)
	}
	if hashesLen := len(c.hashes); hashesLen != wantHashesLen {
		t.Fatalf("expected hashes len to be %d, but got %d", wantHashesLen, hashesLen)
	}
	if isSorted := sort.SliceIsSorted(c.hashes, func(i, j int) bool {
		return c.hashes[i] < c.hashes[j]
	}); !isSorted {
		t.Fatal("hashes are not sorted")
	}
}
