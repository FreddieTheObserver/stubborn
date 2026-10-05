// Package store is the contract between the stubborn engine and a database.
package store

import (
	"context"
	"errors"
	"time"
)

// Store is implemented once per database. Each method is one transaction, and
// a call that returns an error changes nothing.
//
// A Fence matches when its run is running and has the fence's epoch. Every
// write for a run needs a matching fence, and a fence that does not match is
// reported as ErrFenced ahead of any other error.
type Store interface {
	// CreateRun adds a scheduled run that is due now. Creating an existing ID
	// again with the same workflow and input is a no-op whatever the run's
	// status, and with anything else returns ErrIDConflict.
	CreateRun(ctx context.Context, r NewRun) error

	GetRun(ctx context.Context, id string) (Run, error)

	// Claim takes one run of the requested workflows that is scheduled and
	// due, or running with an expired lease, increments its epoch, and leases
	// it until lease from the store's clock. It reports false when no run
	// qualifies.
	Claim(ctx context.Context, req ClaimRequest) (Claim, bool, error)

	Heartbeat(ctx context.Context, f Fence, lease time.Duration) error

	// History returns the run's checkpoints in Seq order.
	History(ctx context.Context, runID string) ([]Checkpoint, error)

	// AppendCheckpoint returns ErrSeqConflict unless cp.Seq is the number of
	// checkpoints the run already has. The stored epoch is the fence's, and
	// cp.Epoch is ignored.
	AppendCheckpoint(ctx context.Context, f Fence, cp Checkpoint) error

	Finish(ctx context.Context, f Fence, o Outcome) error
}

type Fence struct {
	RunID string
	Epoch int64
}

type RunStatus string

const (
	RunScheduled RunStatus = "scheduled"
	RunRunning   RunStatus = "running"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
)

type CheckpointStatus string

const (
	CheckpointDone   CheckpointStatus = "done"
	CheckpointFailed CheckpointStatus = "failed"
)

type NewRun struct {
	ID       string
	Workflow string
	Input    []byte
}

type Run struct {
	ID       string
	Workflow string
	Input    []byte
	Status   RunStatus
	Owner    string
	Epoch    int64
	Output   []byte
	Error    []byte
}

type ClaimRequest struct {
	Owner     string
	Lease     time.Duration
	Workflows []string
}

type Claim struct {
	Fence    Fence
	Workflow string
	Input    []byte
}

type Checkpoint struct {
	Seq    int
	Name   string
	Status CheckpointStatus
	Output []byte
	Error  []byte
	Epoch  int64
}

type Outcome struct {
	Status RunStatus
	Output []byte
	Error  []byte
}

var (
	ErrIDConflict  = errors.New("store: run ID exists with a different workflow or input")
	ErrNotFound    = errors.New("store: run not found")
	ErrFenced      = errors.New("store: run was claimed again or has finished")
	ErrSeqConflict = errors.New("store: checkpoint is not at the next sequence number")
)
