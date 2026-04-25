# AMOS — Failure Modes

This document is the **design authority** for AMOS. Every feature, tenet, and code change must be evaluated against these failures. Do not introduce code that causes any of them.

---

## Category A — SNMP Operation Failures

| ID   | Failure                                          | Consequence                                     | Status                                             |
|------|--------------------------------------------------|-------------------------------------------------|----------------------------------------------------|
| F-01 | Device unreachable / timeout                     | GET/WALK hangs, UI freezes                      | Mitigated: per-operation deadline; goroutine + cancel button |
| F-02 | Wrong community string                           | No response, silent failure                     | Mitigated: timeout + explicit "no response" error shown |
| F-03 | SNMPv3 auth/priv credentials mismatch           | Engine-time / auth failure                      | Mitigated: surface PDU error reason in results     |
| F-04 | SET on read-only OID                             | Device returns error PDU                        | Mitigated: display error PDU reason in results     |
| F-05 | GETBULK returns excessive rows                  | UI overloaded, OOM risk                         | Mitigated: max-repetitions capped, user-configurable |
| F-06 | Concurrent WALK and GET on same session         | Race condition on gosnmp connection             | Mitigated: one SNMP client per target; mutex-protected |

## Category B — MIB Failures

| ID   | Failure                                          | Consequence                                     | Status                                             |
|------|--------------------------------------------------|-------------------------------------------------|----------------------------------------------------|
| F-07 | MIB file parse error                             | Tree not built; crash if unhandled              | Mitigated: recover from panic; show error in status bar |
| F-08 | OID not present in any loaded MIB               | No name resolution, confusing output            | Mitigated: fall back to numeric OID display        |
| F-09 | MIB circular dependency or infinite loop        | Parser loops, app hangs                         | Mitigated: visited-set guard during import resolution |
| F-10 | MIB file missing dependency import              | Partial tree, wrong OID resolution              | Mitigated: warn user about missing imports; show best-effort tree |

## Category C — Network Scanner Failures

| ID   | Failure                                          | Consequence                                     | Status                                             |
|------|--------------------------------------------------|-------------------------------------------------|----------------------------------------------------|
| F-11 | Network scan takes too long                      | UI appears frozen                               | Mitigated: progress bar + cancel; 250ms timeout/host |
| F-12 | Invalid IP / CIDR input                          | Crash or bad scan range                         | Mitigated: validate input before scan; inline error |
| F-13 | Scanner probe causes false positives            | Non-SNMP ports flagged as agents                | Mitigated: require valid SNMP response PDU, not just UDP reply |

## Category D — UI Failures

| ID   | Failure                                          | Consequence                                     | Status                                             |
|------|--------------------------------------------------|-------------------------------------------------|----------------------------------------------------|
| F-14 | App window too small to show all panels         | Panels overlap, unusable                        | Mitigated: Fyne resizable split containers; min window size 900×600 |
| F-15 | Results table not cleared between operations    | Old results mixed with new                      | Mitigated: table cleared on each new operation dispatch |
