Feature: MIB Loading
  As a network engineer using AMOS
  I want to load standard and custom MIB files
  So that OIDs are resolved to human-readable names

  Scenario: Standard MIBs load without errors
    Given the bundled MIB directory
    When I load all MIBs
    Then no fatal load errors are reported

  Scenario: sysDescr OID resolves to a name
    Given the bundled MIB directory
    When I load all MIBs
    Then OID "1.3.6.1.2.1.1.1.0" resolves to a name containing "sysDescr"

  Scenario: Unknown OID falls back to numeric form
    Given the bundled MIB directory
    When I load all MIBs
    Then OID "1.9.9.9.9.9" resolves to "1.9.9.9.9.9"

  Scenario: Malformed MIB file does not prevent other MIBs loading
    Given a temporary directory with a valid MIB and a malformed MIB
    When I load all MIBs from that directory
    Then no fatal load errors are reported
    And OIDs from the valid MIB resolve correctly

  Scenario: Building the OID tree produces a root node
    Given the bundled MIB directory
    When I load all MIBs
    And I build the OID tree
    Then the root node is not nil
