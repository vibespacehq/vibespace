// nats-bridge routes messages between the Claude Code CLI and NATS.
// It subscribes to incoming messages and publishes outgoing messages.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/nats-io/nats.go"
)

// Message represents a chat message
type Message struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	From    string `json:"from,omitempty"`
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

	claudeID := os.Getenv("VIBESPACE_CLAUDE_ID")
	if claudeID == "" {
		claudeID = "1"
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
	log.Printf("Project: %s, Claude ID: %s", project, claudeID)

	// Subject for incoming messages to this Claude
	inSubject := fmt.Sprintf("vibespace.%s.claude.%s.in", project, claudeID)
	// Subject for outgoing messages from this Claude
	outSubject := fmt.Sprintf("vibespace.%s.claude.%s.out", project, claudeID)
	// Subject for status updates
	statusSubject := fmt.Sprintf("vibespace.%s.claude.%s.status", project, claudeID)

	// Publish initial status
	publishStatus(nc, statusSubject, "idle")

	// Subscribe to incoming messages
	_, err = nc.Subscribe(inSubject, func(msg *nats.Msg) {
		log.Printf("Received message on %s", inSubject)

		var inMsg Message
		if err := json.Unmarshal(msg.Data, &inMsg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			return
		}

		// Update status to thinking
		publishStatus(nc, statusSubject, "thinking")

		// Run Claude Code CLI with the message
		response, err := runClaude(inMsg.Content)
		if err != nil {
			log.Printf("Claude error: %v", err)
			publishStatus(nc, statusSubject, "error")
			return
		}

		// Publish response
		outMsg := Message{
			Type:    "response",
			Content: response,
			From:    claudeID,
		}
		outData, _ := json.Marshal(outMsg)
		if err := nc.Publish(outSubject, outData); err != nil {
			log.Printf("Failed to publish response: %v", err)
		}

		// Update status back to idle
		publishStatus(nc, statusSubject, "idle")
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Printf("Subscribed to %s", inSubject)
	log.Printf("Publishing responses to %s", outSubject)

	// Also subscribe to broadcast messages
	allSubject := fmt.Sprintf("vibespace.%s.claude.all", project)
	_, err = nc.Subscribe(allSubject, func(msg *nats.Msg) {
		log.Printf("Received broadcast message on %s", allSubject)
		// Handle broadcast messages the same way
		var inMsg Message
		if err := json.Unmarshal(msg.Data, &inMsg); err != nil {
			log.Printf("Failed to parse broadcast message: %v", err)
			return
		}

		publishStatus(nc, statusSubject, "thinking")

		response, err := runClaude(inMsg.Content)
		if err != nil {
			log.Printf("Claude error: %v", err)
			publishStatus(nc, statusSubject, "error")
			return
		}

		outMsg := Message{
			Type:    "response",
			Content: response,
			From:    claudeID,
		}
		outData, _ := json.Marshal(outMsg)
		if err := nc.Publish(outSubject, outData); err != nil {
			log.Printf("Failed to publish response: %v", err)
		}

		publishStatus(nc, statusSubject, "idle")
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to broadcast: %v", err)
	}

	log.Printf("Subscribed to broadcast %s", allSubject)

	// Keep running
	select {}
}

func publishStatus(nc *nats.Conn, subject, status string) {
	msg := fmt.Sprintf(`{"status":"%s","timestamp":%d}`, status, time.Now().Unix())
	if err := nc.Publish(subject, []byte(msg)); err != nil {
		log.Printf("Failed to publish status: %v", err)
	}
}

func runClaude(prompt string) (string, error) {
	// Run claude CLI in non-interactive mode
	cmd := exec.Command("claude", "-p", prompt)
	cmd.Dir = "/vibespace"
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	// Read output
	var output string
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading stdout: %v", err)
			}
			break
		}
		output += line
	}

	// Also capture stderr
	stderrBytes, _ := io.ReadAll(stderr)
	if len(stderrBytes) > 0 {
		log.Printf("Claude stderr: %s", string(stderrBytes))
	}

	if err := cmd.Wait(); err != nil {
		return output, fmt.Errorf("claude exited with error: %w", err)
	}

	return output, nil
}
