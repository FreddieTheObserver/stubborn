// Package memstore is an in-memory store.Store for tests.
package memstore

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/FreddieTheObserver/stubborn/store"
)

// Store keeps everything behind one mutex, which makes claims exclusive the way
// SQLite's single writer does. Its clock is time.Now, so inside a synctest
// bubble it runs on virtual time.
type Store struct {
	mu          sync.Mutex
	runs        map[string]*run
	checkpoints map[string][]store.Checkpoint
}

type run struct {
	store.Run
	dueAt time.Time
}

var _ store.Store = (*Store)(nil)

func New() *Store {
	return &Store{
		runs:        make(map[string]*run),
		checkpoints: make(map[string][]store.Checkpoint),
	}
}

func (s *Store) CreateRun(ctx context.Context, nr store.NewRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if r, ok := s.runs[nr.ID]; ok {
		if r.Workflow != nr.Workflow || !bytes.Equal(r.Input, nr.Input) {
			return fmt.Errorf("%w (run %q)", store.ErrIDConflict, nr.ID)
		}
		return nil
	}
	s.runs[nr.ID] = &run{
		Run: store.Run{
			ID:       nr.ID,
			Workflow: nr.Workflow,
			Input:    bytes.Clone(nr.Input),
			Status:   store.RunScheduled,
		},
		dueAt: time.Now(),
	}
	return nil
}

func (s *Store) GetRun(ctx context.Context, id string) (store.Run, error) {
	if err := ctx.Err(); err != nil {
		return store.Run{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.runs[id]
	if !ok {
		return store.Run{}, fmt.Errorf("%w (run %q)", store.ErrNotFound, id)
	}
	out := r.Run
	out.Input = bytes.Clone(out.Input)
	out.Output = bytes.Clone(out.Output)
	out.Error = bytes.Clone(out.Error)
	return out, nil
}

func (s *Store) Claim(ctx context.Context, req store.ClaimRequest) (store.Claim, bool, error) {
	if err := ctx.Err(); err != nil {
		return store.Claim{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Times keep their monotonic reading, so a zero lease is already expired
	// at the next call. The earliest due run wins, and ties go to the lowest
	// ID, so engine tests do not depend on map iteration order.
	now := time.Now()
	var next *run
	for _, r := range s.runs {
		if r.Status != store.RunScheduled && r.Status != store.RunRunning {
			continue
		}
		if r.dueAt.After(now) || !slices.Contains(req.Workflows, r.Workflow) {
			continue
		}
		if next == nil || cmp.Or(r.dueAt.Compare(next.dueAt), cmp.Compare(r.ID, next.ID)) < 0 {
			next = r
		}
	}
	if next == nil {
		return store.Claim{}, false, nil
	}

	next.Status = store.RunRunning
	next.Owner = req.Owner
	next.Epoch++
	next.dueAt = now.Add(req.Lease)
	return store.Claim{
		Fence:    store.Fence{RunID: next.ID, Epoch: next.Epoch},
		Workflow: next.Workflow,
		Input:    bytes.Clone(next.Input),
	}, true, nil
}

func (s *Store) Heartbeat(ctx context.Context, f store.Fence, lease time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.fenced(f)
	if err != nil {
		return err
	}
	r.dueAt = time.Now().Add(lease)
	return nil
}

func (s *Store) History(ctx context.Context, runID string) ([]store.Checkpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cps := s.checkpoints[runID]
	out := make([]store.Checkpoint, len(cps))
	for i, cp := range cps {
		out[i] = cloneCheckpoint(cp)
	}
	return out, nil
}

func (s *Store) AppendCheckpoint(ctx context.Context, f store.Fence, cp store.Checkpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.fenced(f); err != nil {
		return err
	}
	if next := len(s.checkpoints[f.RunID]); cp.Seq != next {
		return fmt.Errorf("%w (run %q, seq %d, next %d)", store.ErrSeqConflict, f.RunID, cp.Seq, next)
	}
	if cp.Status != store.CheckpointDone && cp.Status != store.CheckpointFailed {
		return fmt.Errorf("memstore: checkpoint status %q, want done or failed", cp.Status)
	}
	cp = cloneCheckpoint(cp)
	cp.Epoch = f.Epoch
	s.checkpoints[f.RunID] = append(s.checkpoints[f.RunID], cp)
	return nil
}

func (s *Store) Finish(ctx context.Context, f store.Fence, o store.Outcome) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.fenced(f)
	if err != nil {
		return err
	}
	if o.Status != store.RunCompleted && o.Status != store.RunFailed {
		return fmt.Errorf("memstore: finish with status %q, want completed or failed", o.Status)
	}
	r.Status = o.Status
	r.Output = bytes.Clone(o.Output)
	r.Error = bytes.Clone(o.Error)
	return nil
}

// fenced must be called with s.mu held.
func (s *Store) fenced(f store.Fence) (*run, error) {
	r, ok := s.runs[f.RunID]
	if !ok || r.Status != store.RunRunning || r.Epoch != f.Epoch {
		return nil, fmt.Errorf("%w (run %q, epoch %d)", store.ErrFenced, f.RunID, f.Epoch)
	}
	return r, nil
}

func cloneCheckpoint(cp store.Checkpoint) store.Checkpoint {
	cp.Output = bytes.Clone(cp.Output)
	cp.Error = bytes.Clone(cp.Error)
	return cp
}
