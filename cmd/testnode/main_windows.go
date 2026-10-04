//go:build integrationfixture

// A harmless console process used only by Windows host lifecycle tests.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	fmt.Println("fixture ready")
	<-c
	kind := "node"
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "--stratum-port=") {
			kind = "mining"
		}
		if strings.HasPrefix(a, "--rpc-server=") {
			kind = "wallet"
		}
	}
	fmt.Println("fixture graceful shutdown", kind)
	if trace := os.Getenv("ZKAS_TEST_TRACE"); trace != "" {
		f, e := os.OpenFile(trace, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if e == nil {
			fmt.Fprintln(f, kind)
			f.Close()
		}
	}
}
