package snmp

import "time"

// DefaultTimeout is the per-operation deadline used when the caller doesn't specify one.
const DefaultTimeout = 5 * time.Second

// DefaultMaxReps is the default max-repetitions for GETBULK (T-06).
const DefaultMaxReps uint32 = 10
