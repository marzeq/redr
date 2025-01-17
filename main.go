package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

func jsonEscape(i string) string {
	b, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return string(b[1 : len(b)-1])
}

func okMessage() string {
	return "{\"type\": \"ok\"}\n"
}

func commandRanMessage(exitCode uint, cmd string, last bool, silent bool) string {
	return fmt.Sprintf("{\"type\": \"command_ran\", \"exit_code\": %d, \"cmd\": \"%s\", \"last\": %v, \"silent\": %v}\n", exitCode, jsonEscape(cmd), last, silent)
}

func doneMessage() string {
	return "{\"type\": \"done\"}\n"
}

func clearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

const (
	MAX_BYTES = 16 * 1024
	PORT      = 45673
)

var (
	clients      = make(map[net.Conn]bool)
	clientsMutex sync.Mutex
	commandMutex sync.Mutex
	currentCmd   *exec.Cmd
	serverCWD, _ = os.Getwd()
)

func setupSignalHandler(listener net.Listener) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		clearScreen()
		listener.Close()
		shutdown()
		os.Exit(0)
	}()
}

func broadcast(message string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for client := range clients {
		client.Write([]byte(message))
	}
}

func shutdown() {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for client := range clients {
		client.Close()
	}
	if currentCmd != nil && currentCmd.Process != nil {
		currentCmd.Process.Kill()
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

	if currentCmd != nil && currentCmd.Process != nil {
		currentCmd.Process.Kill()
	}

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

		// command was killed
		if exitCode > 255 {
			broadcast(commandRanMessage(exitCode, command, last, true))
		} else {
			broadcast(commandRanMessage(exitCode, command, last, false))
		}

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

	setupSignalHandler(listener)

	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		go handleClient(conn)
	}
}
