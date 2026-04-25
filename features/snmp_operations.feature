Feature: SNMP Operations
  As a network engineer using AMOS
  I want to perform SNMP operations against agents
  So that I can inspect and manage device MIB data

  Scenario: GET a single OID returns a result
    Given a local test SNMP server
    When I perform GET on OID "1.3.6.1.2.1.1.1.0"
    Then the result list is not empty
    And no result has an error

  Scenario: GETNEXT returns the OID after the given one
    Given a local test SNMP server
    When I perform GETNEXT on OID "1.3.6.1.2.1.1.1.0"
    Then the result list is not empty

  Scenario: GETBULK is rejected on SNMP v1
    Given an SNMP v1 target at "127.0.0.1" with community "public"
    When I perform GETBULK on OID "1.3.6.1.2.1.1"
    Then an error is returned containing "not supported on SNMP v1"

  Scenario: WALK collects multiple bindings
    Given a local test SNMP server
    When I perform WALK on OID "1.3.6.1.2.1.1"
    Then more than 0 bindings are collected

  Scenario: Operation times out when host is unreachable
    Given an SNMP v2c target at "192.0.2.1" with community "public"
    When I perform GET on OID "1.3.6.1.2.1.1.1.0" with a 100ms timeout
    Then an error is returned

  Scenario: Empty host is rejected before dispatch
    Given an SNMP v2c target at "" with community "public"
    When I validate the target and OID "1.3.6.1.2.1.1.1.0"
    Then a validation error is returned

  Scenario: Empty OID is rejected before dispatch
    Given an SNMP v2c target at "127.0.0.1" with community "public"
    When I validate the target and OID ""
    Then a validation error is returned
