package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error starting stadium:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Build-a-Bot Stadium is OPEN and listening on port 8080...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleBot(conn)
	}
}

func handleBot(conn net.Conn) {
	defer conn.Close()

	// bufio.Scanner makes it easy to read one line at a time
	scanner := bufio.NewScanner(conn)
	conn.Write([]byte("BBS_WELCOME: Please send JOIN <bot_name>\n"))

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		parts := strings.Split(input, " ")
		command := parts[0]

		switch command {
		case "JOIN":
			if len(parts) > 1 {
				fmt.Printf("Bot '%s' has entered the Stadium!\n", parts[1])
				conn.Write([]byte("OK: You are now seated.\n"))
			}
		case "MOVE":
			// Here is where you'll eventually call your game logic
			fmt.Printf("Bot submitted a move: %s\n", input)
			conn.Write([]byte("OK: Move received.\n"))
		case "QUIT":
			return
		default:
			conn.Write([]byte("ERR: Unknown command\n"))
		}
	}
}
