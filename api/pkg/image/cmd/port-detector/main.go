// port-detector monitors listening TCP ports and publishes events to NATS
// when ports are opened or closed by processes in the container.
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// Ports to ignore (system services)
var ignorePorts = map[int]bool{
	22:   true, // SSH
	53:   true, // DNS
	7681: true, // ttyd
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.default.svc.cluster.local:4222"
	}

	project := os.Getenv("VIBESPACE_PROJECT")
	if project == "" {
		log.Fatal("VIBESPACE_PROJECT environment variable is required")
	}

	// Connect to NATS with retry
	var nc *nats.Conn
	var err error
	for i := 0; i < 30; i++ {
		nc, err = nats.Connect(natsURL)
		if err == nil {
			break
		}
		log.Printf("Waiting for NATS... (%d/30)", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	log.Printf("Connected to NATS at %s", natsURL)
	log.Printf("Monitoring ports for project: %s", project)

	knownPorts := make(map[int]bool)

	for {
		currentPorts := scanListeningPorts()

		// Detect new ports
		for port := range currentPorts {
			if !knownPorts[port] && port > 1024 && !ignorePorts[port] {
				msg := fmt.Sprintf(`{"port":%d}`, port)
				subject := fmt.Sprintf("vibespace.%s.ports.register", project)
				if err := nc.Publish(subject, []byte(msg)); err != nil {
					log.Printf("Failed to publish port register: %v", err)
				} else {
					log.Printf("Port %d opened, published to %s", port, subject)
				}
				knownPorts[port] = true
			}
		}

		// Detect closed ports
		for port := range knownPorts {
			if !currentPorts[port] {
				msg := fmt.Sprintf(`{"port":%d}`, port)
				subject := fmt.Sprintf("vibespace.%s.ports.unregister", project)
				if err := nc.Publish(subject, []byte(msg)); err != nil {
					log.Printf("Failed to publish port unregister: %v", err)
				} else {
					log.Printf("Port %d closed, published to %s", port, subject)
				}
				delete(knownPorts, port)
			}
		}

		time.Sleep(2 * time.Second)
	}
}

// scanListeningPorts parses /proc/net/tcp to find LISTEN sockets
func scanListeningPorts() map[int]bool {
	ports := make(map[int]bool)

	file, err := os.Open("/proc/net/tcp")
	if err != nil {
		log.Printf("Failed to open /proc/net/tcp: %v", err)
		return ports
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Skip header line
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// State is in field 3 (0-indexed), 0A = LISTEN
		state := fields[3]
		if state != "0A" {
			continue
		}

		// Local address is in field 1, format: IP:PORT (hex)
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		portHex := parts[1]
		portBytes, err := hex.DecodeString(portHex)
		if err != nil || len(portBytes) != 2 {
			continue
		}

		port := int(portBytes[0])<<8 + int(portBytes[1])
		ports[port] = true
	}

	// Also check /proc/net/tcp6 for IPv6
	file6, err := os.Open("/proc/net/tcp6")
	if err == nil {
		defer file6.Close()
		scanner6 := bufio.NewScanner(file6)
		scanner6.Scan() // Skip header

		for scanner6.Scan() {
			line := scanner6.Text()
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}

			state := fields[3]
			if state != "0A" {
				continue
			}

			localAddr := fields[1]
			parts := strings.Split(localAddr, ":")
			if len(parts) != 2 {
				continue
			}

			port, err := strconv.ParseInt(parts[1], 16, 32)
			if err != nil {
				continue
			}

			ports[int(port)] = true
		}
	}

	return ports
}
