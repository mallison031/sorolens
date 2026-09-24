package poller

import (
	"context"
	"time"
)

// RPCClient is the subset of the Soroban RPC client the poller needs.
// The concrete implementation is soroban.Client from apps/api.
type RPCClient interface {
	GetLatestLedger(ctx context.Context) (*LatestLedger, error)
	GetEvents(ctx context.Context, startLedger, endLedger uint32, filters []EventFilter) (*GetEventsResult, error)
	GetTransaction(ctx context.Context, hash string) (*TransactionResult, error)
}

// Store is the subset of the data store the poller needs.
// The concrete implementation is store.postgresStore from apps/api.
type Store interface {
	ListContracts(ctx context.Context, cursor string, limit int) ([]Contract, string, error)
	BatchInsertEvents(ctx context.Context, events []Event) error
	BatchInsertInvocations(ctx context.Context, invocations []Invocation) error
	GetSyncState(ctx context.Context, contractID string) (SyncState, error)
	UpsertSyncState(ctx context.Context, s SyncState) error
	CreateNextMonthPartition(ctx context.Context) error
	CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error

	// RecentHourlyActivity returns per-hour activity buckets for the most
	// recent `hours` hours (oldest first), aggregated across events and
	// invocations. Used by the anomaly detector to build a rolling baseline.
	RecentHourlyActivity(ctx context.Context, contractID string, hours int) ([]HourlyActivity, error)
	// InsertAlert persists an anomaly/health alert row (severity Warning for
	// anomaly spikes). Implementations may de-duplicate on (tx_hash,
	// contract_id).
	InsertAlert(ctx context.Context, a Alert) error
}

// RedisClient is the subset of Redis operations the poller needs for advisory locks.
type RedisClient interface {
	// SetNX sets key to value with ttl if key does not already exist.
	// Returns true if the key was set (lock acquired).
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	// Del removes the key (releases the lock).
	Del(ctx context.Context, key string) error
}

// ---- local mirror types (avoid importing apps/api from this package) ------
// The poller defines its own minimal types. The main.go adapter converts
// between apps/api types and these types when wiring up the real implementations.

// LatestLedger mirrors soroban.LatestLedger.
type LatestLedger struct {
	Sequence        uint32
	ProtocolVersion int
}

// EventFilter mirrors soroban.EventFilter.
type EventFilter struct {
	Type        string
	ContractIDs []string
}

// RPCEvent mirrors soroban.RPCEvent.
type RPCEvent struct {
	ID                       string
	ContractID               string
	Ledger                   uint32
	LedgerClosedAt           string
	TxHash                   string
	Type                     string
	Topic                    []string
	Value                    string
	InSuccessfulContractCall bool
	TransactionIndex         int
}

// GetEventsResult mirrors soroban.GetEventsResult.
type GetEventsResult struct {
	Events       []RPCEvent
	LatestLedger uint32
	Cursor       string
}

// TransactionResult mirrors soroban.TransactionResult.
type TransactionResult struct {
	Status           string
	Ledger           uint32
	LedgerClosedAt   time.Time
	ApplicationOrder int
	ResultXDR        string
	ResourceFee      int64
}

// Contract mirrors store.Contract (fields the poller needs).
type Contract struct {
	ID      string
	Status  string
	Network string
}

// Event mirrors store.Event.
type Event struct {
	ID               string
	ContractID       string
	Network          string
	Ledger           uint32
	LedgerClosedAt   time.Time
	TxHash           string
	Type             string
	TopicXDR         []string
	ValueXDR         string
	InSuccessfulCall bool
}

// Invocation mirrors store.Invocation.
type Invocation struct {
	TxHash           string
	ContractID       string
	Network          string
	Ledger           uint32
	LedgerClosedAt   time.Time
	Status           string
	ResultXDR        string
	ApplicationOrder int
}

// SyncState mirrors store.SyncState.
type SyncState struct {
	ContractID string
	LastLedger uint32
}

// HourlyActivity is one per-hour aggregate bucket for a contract, used by the
// anomaly detector. CPU and fees are totals over the hour.
type HourlyActivity struct {
	Hour        time.Time // bucket start, UTC
	EventCount  int64
	InvokeCount int64
	CPU         int64 // sum of cpu_insn
	Fees        int64 // sum of resource fees, stroops
}

// Alert mirrors store.ContractAlert. TxHash carries a synthetic, deterministic
// key (see anomaly job) so implementations can de-duplicate re-runs.
type Alert struct {
	ContractID string
	Severity   string // Info | Warning | Critical
	Message    string
	Ledger     int64
	TxHash     string
	Timestamp  time.Time
}
