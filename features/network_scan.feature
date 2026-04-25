Feature: Network Scanner
  As a network engineer using AMOS
  I want to scan my local subnet for SNMP agents
  So that I can discover devices to inspect

  Scenario: Scanning a single loopback host discovers a local agent
    Given a running SNMP simulator on "127.0.0.1"
    When I scan the CIDR "127.0.0.1/32"
    Then at least 1 device is found

  Scenario: Scanning an unreachable range finds nothing
    When I scan the CIDR "192.0.2.0/30" with a 100ms timeout
    Then 0 devices are found

  Scenario: Invalid CIDR returns an error immediately
    When I scan the CIDR "not-a-cidr"
    Then an error is returned from the scanner

  Scenario: Single IP (no CIDR mask) is accepted
    When I scan the CIDR "127.0.0.1"
    Then no parse error is returned

  Scenario: Scanner can be cancelled mid-scan
    When I start a scan of "192.0.2.0/24" and cancel after 50ms
    Then the scanner stops within 500ms
