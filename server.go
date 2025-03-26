package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Connection struct {
	conn     net.Conn
	id       int
	addr     string
	messages []string
	msgMutex sync.Mutex
	isActive bool
}

const (
	discordWebhookURL = "<URLHERE>"
)

func sendDiscordNotification(message string) {
	webhookContent := map[string]string{
		"content": message,
	}

	jsonContent, err := json.Marshal(webhookContent)
	if err != nil {
		fmt.Printf("Error creating webhook JSON: %v\n", err)
		return
	}

	resp, err := http.Post(discordWebhookURL, "application/json", bytes.NewBuffer(jsonContent))
	if err != nil {
		fmt.Printf("Error sending Discord notification: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		fmt.Printf("Discord webhook returned non-OK status: %s\n", resp.Status)
	}
}

func main() {
	listener, err := net.Listen("tcp4", "127.0.0.1:8080")
	if err != nil {
		fmt.Printf("Couldn't listen on localhost:8080: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Listening on localhost:8080")
	defer listener.Close()

	var connections []*Connection
	var connMutex sync.Mutex
	nextID := 1

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Printf("Error accepting connection: %v\n", err)
				continue
			}

			connMutex.Lock()
			newConn := &Connection{
				conn:     conn,
				id:       nextID,
				addr:     conn.RemoteAddr().String(),
				messages: make([]string, 0),
				isActive: false,
			}
			connections = append(connections, newConn)
			nextID++
			connMutex.Unlock()

			fmt.Printf("New connection from %s (ID: %d)\n", newConn.addr, newConn.id)

			sendDiscordNotification(fmt.Sprintf("🟢 New connection from %s (ID: %d)", newConn.addr, newConn.id))

			go handleConnection(newConn)
		}
	}()

	var activeConn *Connection = nil
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if activeConn == nil {
			fmt.Print("\nCommands:\n  list - Show all connections\n  select <id> - Select a connection\n  close <id> - Close a connection\n  exit - Exit the server\n> ")

			scanner.Scan()
			cmd := strings.TrimSpace(scanner.Text())
			parts := strings.Fields(cmd)

			if len(parts) == 0 {
				continue
			}

			switch parts[0] {
			case "list":
				connMutex.Lock()
				fmt.Println("\nActive connections:")
				for _, c := range connections {
					fmt.Printf("  [%d] %s\n", c.id, c.addr)
				}
				connMutex.Unlock()

			case "select":
				if len(parts) != 2 {
					fmt.Println("Usage: select <id>")
					continue
				}

				id, err := strconv.Atoi(parts[1])
				if err != nil {
					fmt.Println("Invalid ID")
					continue
				}

				connMutex.Lock()
				found := false
				for _, c := range connections {
					if c.id == id {
						activeConn = c
						activeConn.isActive = true
						found = true
						break
					}
				}
				connMutex.Unlock()

				if !found {
					fmt.Printf("No connection with ID %d\n", id)
					continue
				}

				activeConn.msgMutex.Lock()
				if len(activeConn.messages) > 0 {
					fmt.Println("\nBuffered messages:")
					for _, msg := range activeConn.messages {
						fmt.Print(msg)
					}
					activeConn.messages = nil
				}
				activeConn.msgMutex.Unlock()

				fmt.Printf("\nConnected to %s (ID: %d). Type 'exit' to return to connection menu.\n",
					activeConn.addr, activeConn.id)

			case "close":
				if len(parts) != 2 {
					fmt.Println("Usage: close <id>")
					continue
				}

				id, err := strconv.Atoi(parts[1])
				if err != nil {
					fmt.Println("Invalid ID")
					continue
				}

				connMutex.Lock()
				for i, c := range connections {
					if c.id == id {
						sendDiscordNotification(fmt.Sprintf("🔴 Connection manually closed: %s (ID: %d)", c.addr, c.id))
						c.conn.Close()
						connections = append(connections[:i], connections[i+1:]...)
						fmt.Printf("Closed connection %d\n", id)
						break
					}
				}
				connMutex.Unlock()

			case "exit":
				fmt.Println("Shutting down server...")
				connMutex.Lock()
				for _, c := range connections {
					c.conn.Close()
				}
				connMutex.Unlock()
				return

			default:
				fmt.Println("Unknown command. Type 'list', 'select <id>', 'close <id>', or 'exit'")
			}

		} else {
			fmt.Print("> ")
			scanner.Scan()
			input := scanner.Text()

			if input == "exit" {
				activeConn.isActive = false
				activeConn = nil
				continue
			}

			_, err := activeConn.conn.Write([]byte(input + "\n"))
			if err != nil {
				fmt.Printf("Error writing to connection: %v\n", err)
				activeConn.isActive = false
				activeConn = nil
			}
		}
	}
}

func handleConnection(conn *Connection) {
	defer func() {
		conn.conn.Close()
		fmt.Printf("Connection closed: %s (ID: %d)\n", conn.addr, conn.id)
		sendDiscordNotification(fmt.Sprintf("🔴 Connection closed: %s (ID: %d)", conn.addr, conn.id))
	}()

	buffer := make([]byte, 1024)
	for {
		n, err := conn.conn.Read(buffer)
		if err != nil {
			return
		}

		message := string(buffer[:n])

		if conn.isActive {
			fmt.Print(message)
		} else {
			conn.msgMutex.Lock()
			conn.messages = append(conn.messages, message)
			conn.msgMutex.Unlock()
		}
	}
}
