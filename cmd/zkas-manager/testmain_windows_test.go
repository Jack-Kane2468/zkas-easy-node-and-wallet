package main

import (
	"os"
	"testing"
)

// Service launchers copy os.Executable() to a versioned host path. Under go
// test this is the test binary, so its subprocess must enter the same service
// dispatch as the shipped manager rather than parse --host as a testing flag.
// Keep this dispatch test-only; normal test runs still execute the full suite.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "--host", "--wallet-host", "--mining-host", "--kaspa-host", "--dual-host":
			main()
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}
