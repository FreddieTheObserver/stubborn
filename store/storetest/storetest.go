// Package storetest is the conformance suite every store.Store must pass.
package storetest

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/FreddieTheObserver/stubborn/store"
)

// Test runs the suite, giving each subtest a fresh, empty store from open.
// Leases are either zero, which has expired by the next call, or an hour,
// which outlives the test, so the suite never waits and never needs to
// control the store's clock.
func Test(t *testing.T, open func(t *testing.T) store.Store) {
	tests := []struct {
		name string
		fn   func(t *testing.T, s store.Store)
	}{
		{"CreateIdempotent", testCreateIdempotent},
		{"ClaimExclusive", testClaimExclusive},
		{"ClaimWorkflowFilter", testClaimWorkflowFilter},
		{"LeaseExpiry", testLeaseExpiry},
		{"StaleEpoch", testStaleEpoch},
		{"CheckpointsAppendOnly", testCheckpointsAppendOnly},
		{"FinishedRunsFrozen", testFinishedRunsFrozen},
		{"NoAliasing", testNoAliasing},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { tc.fn(t, open(t)) })
	}
}

func testCreateIdempotent(t *testing.T, s store.Store) {
	ctx := t.Context()
	mustCreate(t, s, "r", "wf", "in")
	mustCreate(t, s, "r", "wf", "in")

	r := mustGetRun(t, s, "r")
	if r.Status != store.RunScheduled || r.Epoch != 0 || string(r.Input) != "in" {
		t.Fatalf("after create: status %q, epoch %d, input %q; want scheduled, 0, in", r.Status, r.Epoch, r.Input)
	}

	for _, nr := range []store.NewRun{
		{ID: "r", Workflow: "other", Input: []byte("in")},
		{ID: "r", Workflow: "wf", Input: []byte("different")},
	} {
		err := s.CreateRun(ctx, nr)
		wantErr(t, fmt.Sprintf("CreateRun(workflow %q, input %q)", nr.Workflow, nr.Input), err, store.ErrIDConflict)
	}
	if r := mustGetRun(t, s, "r"); r.Workflow != "wf" || string(r.Input) != "in" {
		t.Fatalf("conflicting create changed the run: workflow %q, input %q", r.Workflow, r.Input)
	}

	c := mustClaim(t, s, "A", time.Hour, "wf")
	mustFinish(t, s, c.Fence, store.Outcome{Status: store.RunCompleted, Output: []byte("out")})
	mustCreate(t, s, "r", "wf", "in")
	if r := mustGetRun(t, s, "r"); r.Status != store.RunCompleted || string(r.Output) != "out" {
		t.Fatalf("create after finish: status %q, output %q; want completed, out", r.Status, r.Output)
	}

	_, err := s.GetRun(ctx, "missing")
	wantErr(t, `GetRun("missing")`, err, store.ErrNotFound)
}

func testClaimExclusive(t *testing.T, s store.Store) {
	const runs, claimers = 50, 8
	ctx := t.Context()
	var want []string
	for i := range runs {
		id := fmt.Sprintf("r%02d", i)
		mustCreate(t, s, id, "wf", "in")
		want = append(want, id)
	}

	var (
		mu      sync.Mutex
		claimed []string
		wg      sync.WaitGroup
	)
	for w := range claimers {
		wg.Go(func() {
			req := store.ClaimRequest{Owner: fmt.Sprintf("w%d", w), Lease: time.Hour, Workflows: []string{"wf"}}
			for {
				c, ok, err := s.Claim(ctx, req)
				if err != nil {
					t.Errorf("claimer %d: %v", w, err)
					return
				}
				if !ok {
					return
				}
				mu.Lock()
				claimed = append(claimed, c.Fence.RunID)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	slices.Sort(claimed)
	if !slices.Equal(claimed, want) {
		t.Fatalf("claimed %v, want each of the %d runs exactly once", claimed, runs)
	}
	for _, id := range want {
		if r := mustGetRun(t, s, id); r.Epoch != 1 {
			t.Errorf("run %s: epoch %d after one claim each, want 1", id, r.Epoch)
		}
	}
	claimNone(t, s, "wf")
}

func testClaimWorkflowFilter(t *testing.T, s store.Store) {
	mustCreate(t, s, "x", "a", "in")
	mustCreate(t, s, "y", "b", "in")

	if c := mustClaim(t, s, "A", time.Hour, "b"); c.Fence.RunID != "y" || c.Workflow != "b" {
		t.Fatalf("Claim([b]) = run %q of %q, want y of b", c.Fence.RunID, c.Workflow)
	}
	claimNone(t, s, "b")
	claimNone(t, s)
	claimNone(t, s, []string{}...)
	if c := mustClaim(t, s, "A", time.Hour, "a", "c"); c.Fence.RunID != "x" || c.Workflow != "a" {
		t.Fatalf("Claim([a c]) = run %q of %q, want x of a", c.Fence.RunID, c.Workflow)
	}
}

func testLeaseExpiry(t *testing.T, s store.Store) {
	mustCreate(t, s, "r", "expire", "in")
	first := mustClaim(t, s, "A", 0, "expire")
	if first.Fence.Epoch != 1 {
		t.Fatalf("first claim: epoch %d, want 1", first.Fence.Epoch)
	}
	second := mustClaim(t, s, "B", time.Hour, "expire")
	if second.Fence.RunID != "r" || second.Fence.Epoch != 2 {
		t.Fatalf("claim after expiry: run %q epoch %d, want r epoch 2", second.Fence.RunID, second.Fence.Epoch)
	}
	claimNone(t, s, "expire")
	if r := mustGetRun(t, s, "r"); r.Status != store.RunRunning || r.Owner != "B" || r.Epoch != 2 {
		t.Fatalf("after reclaim: status %q, owner %q, epoch %d; want running, B, 2", r.Status, r.Owner, r.Epoch)
	}

	mustCreate(t, s, "h", "heartbeat", "in")
	c := mustClaim(t, s, "A", 0, "heartbeat")
	if err := s.Heartbeat(t.Context(), c.Fence, time.Hour); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	claimNone(t, s, "heartbeat")
}

func testStaleEpoch(t *testing.T, s store.Store) {
	mustCreate(t, s, "r", "wf", "in")
	old := mustClaim(t, s, "A", 0, "wf")
	mustAppend(t, s, old.Fence, step(0, "first"))
	cur := mustClaim(t, s, "B", 0, "wf")

	// The next seq is right, so only the fence can reject these. The rejected
	// Heartbeat must not extend cur's zero lease, which the last claim checks.
	wantFenced(t, s, old.Fence, 1)
	err := s.AppendCheckpoint(t.Context(), old.Fence, step(5, "gap"))
	wantErr(t, "AppendCheckpoint with the old fence and a wrong seq", err, store.ErrFenced)
	wantFenced(t, s, store.Fence{RunID: "missing", Epoch: 1}, 0)

	h := mustHistory(t, s, "r")
	if len(h) != 1 || h[0].Name != "first" || h[0].Epoch != old.Fence.Epoch {
		t.Fatalf("history after rejected writes: %+v, want only seq 0 at epoch %d", h, old.Fence.Epoch)
	}
	if r := mustGetRun(t, s, "r"); r.Status != store.RunRunning || r.Owner != "B" || r.Epoch != cur.Fence.Epoch {
		t.Fatalf("run after rejected writes: status %q, owner %q, epoch %d; want running, B, %d", r.Status, r.Owner, r.Epoch, cur.Fence.Epoch)
	}
	if c := mustClaim(t, s, "C", time.Hour, "wf"); c.Fence.Epoch != cur.Fence.Epoch+1 {
		t.Fatalf("claim after rejected heartbeat: epoch %d, want %d", c.Fence.Epoch, cur.Fence.Epoch+1)
	}
}

func testCheckpointsAppendOnly(t *testing.T, s store.Store) {
	ctx := t.Context()
	mustCreate(t, s, "r", "wf", "in")
	c := mustClaim(t, s, "A", time.Hour, "wf")

	err := s.AppendCheckpoint(ctx, c.Fence, step(1, "gap"))
	wantErr(t, "AppendCheckpoint at seq 1 before seq 0", err, store.ErrSeqConflict)

	mustAppend(t, s, c.Fence, store.Checkpoint{Seq: 0, Name: "first", Status: store.CheckpointDone, Output: []byte("a")})
	err = s.AppendCheckpoint(ctx, c.Fence, store.Checkpoint{Seq: 0, Name: "overwrite", Status: store.CheckpointDone, Output: []byte("b")})
	wantErr(t, "AppendCheckpoint at seq 0 twice", err, store.ErrSeqConflict)

	mustAppend(t, s, c.Fence, store.Checkpoint{Seq: 1, Name: "second", Status: store.CheckpointFailed, Error: []byte("boom"), Epoch: 99})

	for _, status := range []store.CheckpointStatus{"", "retrying"} {
		if err := s.AppendCheckpoint(ctx, c.Fence, store.Checkpoint{Seq: 2, Name: "bad", Status: status}); err == nil {
			t.Errorf("AppendCheckpoint with status %q: err = nil, want an error", status)
		}
	}

	want := []store.Checkpoint{
		{Seq: 0, Name: "first", Status: store.CheckpointDone, Output: []byte("a"), Epoch: c.Fence.Epoch},
		{Seq: 1, Name: "second", Status: store.CheckpointFailed, Error: []byte("boom"), Epoch: c.Fence.Epoch},
	}
	if h := mustHistory(t, s, "r"); !slices.EqualFunc(h, want, checkpointEqual) {
		t.Fatalf("history:\n got %+v\nwant %+v", h, want)
	}
	if h := mustHistory(t, s, "missing"); len(h) != 0 {
		t.Fatalf(`History("missing") = %+v, want none`, h)
	}
}

func testFinishedRunsFrozen(t *testing.T, s store.Store) {
	ctx := t.Context()
	mustCreate(t, s, "ok", "wf", "in")
	c := mustClaim(t, s, "A", 0, "wf")
	mustFinish(t, s, c.Fence, store.Outcome{Status: store.RunCompleted, Output: []byte("out")})

	// The lease was zero, so only the terminal status keeps this run unclaimed.
	claimNone(t, s, "wf")
	wantFenced(t, s, c.Fence, 0)
	if r := mustGetRun(t, s, "ok"); r.Status != store.RunCompleted || string(r.Output) != "out" || len(r.Error) != 0 {
		t.Fatalf("finished run: status %q, output %q, error %q; want completed, out, none", r.Status, r.Output, r.Error)
	}
	if h := mustHistory(t, s, "ok"); len(h) != 0 {
		t.Fatalf("finished run gained checkpoints: %+v", h)
	}

	mustCreate(t, s, "bad", "wf2", "in")
	c = mustClaim(t, s, "A", time.Hour, "wf2")
	for _, status := range []store.RunStatus{"", store.RunScheduled, store.RunRunning} {
		if err := s.Finish(ctx, c.Fence, store.Outcome{Status: status}); err == nil {
			t.Errorf("Finish with status %q: err = nil, want an error", status)
		}
	}
	if r := mustGetRun(t, s, "bad"); r.Status != store.RunRunning {
		t.Fatalf("after invalid Finish: status %q, want running", r.Status)
	}
	mustFinish(t, s, c.Fence, store.Outcome{Status: store.RunFailed, Error: []byte("boom")})
	if r := mustGetRun(t, s, "bad"); r.Status != store.RunFailed || string(r.Error) != "boom" {
		t.Fatalf("failed run: status %q, error %q; want failed, boom", r.Status, r.Error)
	}
	claimNone(t, s, "wf2")
}

// Each read is checked before its result is modified, so a store that
// returns the wrong bytes fails here instead of panicking on an index.
func testNoAliasing(t *testing.T, s store.Store) {
	in := []byte("in")
	if err := s.CreateRun(t.Context(), store.NewRun{ID: "r", Workflow: "wf", Input: in}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	in[0] = 'X'
	r := mustGetRun(t, s, "r")
	if string(r.Input) != "in" {
		t.Fatalf("input after changing the caller's slice: %q, want in", r.Input)
	}
	r.Input[0] = 'X'
	c := mustClaim(t, s, "A", time.Hour, "wf")
	if string(c.Input) != "in" {
		t.Fatalf("claimed input after changing GetRun's slice: %q, want in", c.Input)
	}
	c.Input[0] = 'X'
	if r := mustGetRun(t, s, "r"); string(r.Input) != "in" {
		t.Fatalf("input after changing Claim's slice: %q, want in", r.Input)
	}

	out := []byte("out")
	mustAppend(t, s, c.Fence, store.Checkpoint{Seq: 0, Name: "step", Status: store.CheckpointDone, Output: out})
	out[0] = 'X'
	h := mustHistory(t, s, "r")
	if len(h) != 1 || string(h[0].Output) != "out" {
		t.Fatalf("history after changing the appended slice: %+v, want one checkpoint with output out", h)
	}
	h[0].Output[0] = 'X'
	if h := mustHistory(t, s, "r"); string(h[0].Output) != "out" {
		t.Fatalf("checkpoint output after changing History's slice: %q, want out", h[0].Output)
	}

	result := []byte("result")
	mustFinish(t, s, c.Fence, store.Outcome{Status: store.RunCompleted, Output: result})
	result[0] = 'X'
	r = mustGetRun(t, s, "r")
	if string(r.Output) != "result" {
		t.Fatalf("output after changing the finished slice: %q, want result", r.Output)
	}
	r.Output[0] = 'X'
	if r := mustGetRun(t, s, "r"); string(r.Output) != "result" {
		t.Fatalf("output after changing GetRun's slice: %q, want result", r.Output)
	}
}

func mustCreate(t *testing.T, s store.Store, id, workflow, input string) {
	t.Helper()
	err := s.CreateRun(t.Context(), store.NewRun{ID: id, Workflow: workflow, Input: []byte(input)})
	if err != nil {
		t.Fatalf("CreateRun(%q): %v", id, err)
	}
}

func mustGetRun(t *testing.T, s store.Store, id string) store.Run {
	t.Helper()
	r, err := s.GetRun(t.Context(), id)
	if err != nil {
		t.Fatalf("GetRun(%q): %v", id, err)
	}
	return r
}

func mustClaim(t *testing.T, s store.Store, owner string, lease time.Duration, workflows ...string) store.Claim {
	t.Helper()
	c, ok, err := s.Claim(t.Context(), store.ClaimRequest{Owner: owner, Lease: lease, Workflows: workflows})
	if err != nil {
		t.Fatalf("Claim(%v): %v", workflows, err)
	}
	if !ok {
		t.Fatalf("Claim(%v): nothing to claim", workflows)
	}
	return c
}

func claimNone(t *testing.T, s store.Store, workflows ...string) {
	t.Helper()
	c, ok, err := s.Claim(t.Context(), store.ClaimRequest{Owner: "none", Lease: time.Hour, Workflows: workflows})
	if err != nil {
		t.Fatalf("Claim(%v): %v", workflows, err)
	}
	if ok {
		t.Fatalf("Claim(%v) = run %q epoch %d, want nothing", workflows, c.Fence.RunID, c.Fence.Epoch)
	}
}

func mustAppend(t *testing.T, s store.Store, f store.Fence, cp store.Checkpoint) {
	t.Helper()
	if err := s.AppendCheckpoint(t.Context(), f, cp); err != nil {
		t.Fatalf("AppendCheckpoint(seq %d): %v", cp.Seq, err)
	}
}

func mustHistory(t *testing.T, s store.Store, runID string) []store.Checkpoint {
	t.Helper()
	h, err := s.History(t.Context(), runID)
	if err != nil {
		t.Fatalf("History(%q): %v", runID, err)
	}
	return h
}

func mustFinish(t *testing.T, s store.Store, f store.Fence, o store.Outcome) {
	t.Helper()
	if err := s.Finish(t.Context(), f, o); err != nil {
		t.Fatalf("Finish(%s): %v", o.Status, err)
	}
}

// wantFenced checks that every write with f is rejected as fenced. seq is the
// run's next sequence number, so a sequence check cannot be what rejects it.
func wantFenced(t *testing.T, s store.Store, f store.Fence, seq int) {
	t.Helper()
	ctx := t.Context()
	wantErr(t, fmt.Sprintf("Heartbeat(%+v)", f), s.Heartbeat(ctx, f, time.Hour), store.ErrFenced)
	wantErr(t, fmt.Sprintf("AppendCheckpoint(%+v)", f), s.AppendCheckpoint(ctx, f, step(seq, "late")), store.ErrFenced)
	wantErr(t, fmt.Sprintf("Finish(%+v)", f), s.Finish(ctx, f, store.Outcome{Status: store.RunCompleted}), store.ErrFenced)
}

func wantErr(t *testing.T, what string, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("%s: err = %v, want %v", what, err, target)
	}
}

func step(seq int, name string) store.Checkpoint {
	return store.Checkpoint{Seq: seq, Name: name, Status: store.CheckpointDone}
}

// Stores may return nil or empty for absent bytes, so compare by content.
func checkpointEqual(a, b store.Checkpoint) bool {
	return a.Seq == b.Seq && a.Name == b.Name && a.Status == b.Status && a.Epoch == b.Epoch &&
		bytes.Equal(a.Output, b.Output) && bytes.Equal(a.Error, b.Error)
}
