package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

const (
	MAX_BYTES = 16 * 1024
	PORT      = 45673
)

var (
	clients      = make(map[net.Conn]bool)
	killed       bool
	clientsMutex sync.Mutex
	commandMutex sync.Mutex
	currentCmd   *exec.Cmd
	serverCWD, _ = os.Getwd()
)

func broadcast(message string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for client := range clients {
		client.Write([]byte(message))
	}
}

func handleClient(conn net.Conn) {
	clientsMutex.Lock()
	clients[conn] = true
	clientsMutex.Unlock()

	defer func() {
		clientsMutex.Lock()
		delete(clients, conn)
		clientsMutex.Unlock()
		conn.Close()
	}()

	buf := make([]byte, MAX_BYTES)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		messageStr := string(buf[:n])
		var message map[string]interface{}
		err = json.Unmarshal([]byte(messageStr), &message)
		if err != nil {
			fmt.Println("Error parsing JSON:", err)
			return
		}

		if message["type"] == "run" {
			cwd := serverCWD
			if message["cwd"] != nil {
				cwd = message["cwd"].(string)
			}

			runNextAfterFailure := false
			if message["run_next_after_failure"] != nil {
				runNextAfterFailure = message["run_next_after_failure"].(bool)
			}

			commands := message["commands"].([]interface{})
			commandStrings := make([]string, len(commands))
			for i, cmd := range commands {
				commandStrings[i] = cmd.(string)
			}

			conn.Write([]byte(okMessage()))

			commandMutex.Lock()
			if currentCmd != nil && currentCmd.Process != nil {
				currentCmd.Process.Kill()
				currentCmd.Wait()
				resetTerminal()
			}
			go executeCommands(commandStrings, cwd, runNextAfterFailure)
			commandMutex.Unlock()
		} else if message["type"] == "ignore" {
		}
	}
}

func executeCommands(commands []string, cwd string, runNextAfterFailure bool) {
	oldCWD, _ := os.Getwd()
	defer os.Chdir(oldCWD)
	os.Chdir(cwd)

	clearScreen()
	for i, command := range commands {
		fmt.Println(">", command)

		if runtime.GOOS == "windows" {
			currentCmd = exec.Command("powershell.exe", "-Command", command)
		} else {
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}
			currentCmd = exec.Command(shell, "-c", command)
		}

		currentCmd.Stdout = os.Stdout
		currentCmd.Stderr = os.Stderr
		currentCmd.Stdin = os.Stdin

		err := currentCmd.Run()
		exitCode := uint(0)
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = uint(exitError.ExitCode())
			} else {
				exitCode = 1
			}
		}

		last := err != nil && !runNextAfterFailure || i == len(commands)-1

		if killed {
			broadcast(commandRanMessage(exitCode, command, true, true))
			break
		}

		broadcast(commandRanMessage(exitCode, command, last, false))

		if last {
			break
		}
	}
}

func main() {
	clearScreen()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", PORT))
	if err != nil {
		fmt.Println("Listen error:", err)
		return
	}
	defer listener.Close()

	setupCleanCloseHandler(listener)

	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		go handleClient(conn)
	}
}
