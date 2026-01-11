// portfwd - Simple TCP port forwarder for vibespace
// Forwards connections from a local port to a target address
// Usage: portfwd <listen-port> <target-host:port>
// Example: portfwd 443 127.0.0.1:30443

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <listen-port> <target-host:port>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s 443 127.0.0.1:30443\n", os.Args[0])
		os.Exit(1)
	}

	listenAddr := "127.0.0.1:" + os.Args[1]
	targetAddr := os.Args[2]

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen on %s: %v\n", listenAddr, err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Printf("Forwarding %s -> %s\n", listenAddr, targetAddr)

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		listener.Close()
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if listener was closed
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return
			}
			fmt.Fprintf(os.Stderr, "Accept error: %v\n", err)
			continue
		}
		go handleConnection(conn, targetAddr)
	}
}

func handleConnection(client net.Conn, targetAddr string) {
	defer client.Close()

	target, err := net.Dial("tcp", targetAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to %s: %v\n", targetAddr, err)
		return
	}
	defer target.Close()

	// Bidirectional copy
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(target, client)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(client, target)
		done <- struct{}{}
	}()

	// Wait for either direction to complete
	<-done
}
