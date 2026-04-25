# AMOS — Design Tenets

Derived from FAILURE_MODES.md. Every code change must be validated against these.

| ID   | Tenet                                                                                                                 |
|------|-----------------------------------------------------------------------------------------------------------------------|
| T-01 | **Non-blocking operations**: All SNMP operations run in goroutines. The UI goroutine must never block on I/O.        |
| T-02 | **Graceful MIB parse failure**: The MIB loader must recover from all panics. A bad file shows an error; all other loaded MIBs remain usable. |
| T-03 | **Numeric OID fallback**: When an OID has no MIB name, display its numeric form. Never crash or hide the result.     |
| T-04 | **Error PDU surfaced**: SET/GET errors received from the device are always shown in the results table with the PDU error reason. |
| T-05 | **Scanner is cancellable**: The subnet scanner must honour a `context.Context` cancellation within one probe interval. |
| T-06 | **Bounded GETBULK**: Max-repetitions must be user-configurable and default to 10 to prevent OOM from runaway bulk fetches. |
| T-07 | **One connection per target**: gosnmp clients are keyed by host+port+credentials. Concurrent operations on the same target reuse the client under a mutex. |
| T-08 | **Input validated before dispatch**: Host, OID, community string, and CIDR inputs are validated before any network call is made. |
