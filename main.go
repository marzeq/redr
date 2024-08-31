package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

func kick_off_message() string {
	return "{\"type\": \"kick_off\"}\n"
}

func ok_message() string {
	return "{\"type\": \"ok\"}\n"
}

func command_ran_message(exit_code uint) string {
	return fmt.Sprintf("{\"type\": \"command_ran\", \"exit_code\": %d}\n", exit_code)
}

func enableAlternateScreen() {
	fmt.Fprint(os.Stdout, "\033[?1049h\033[H")
}

func disableAlternateScreen() {
	fmt.Fprint(os.Stdout, "\033[?1049l")
}

func clearScreen() {
	fmt.Fprint(os.Stdout, "\033[2J\033[H")
}

func setupSignalHandler(l net.Listener) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		disableAlternateScreen()
		l.Close()
		os.Exit(0)
	}()
}

const (
	MAX_BYTES = 16 * 1024
	PORT      = 45673
)

var hasClient = false

func main() {
	enableAlternateScreen()
	defer disableAlternateScreen()
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", PORT))
	setupSignalHandler(l)

	if err != nil {
		fmt.Println("Listen error:", err)
		return
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			break
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, MAX_BYTES)
	introduced := false
	oldCwd, _ := os.Getwd()

	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Read error:", err)
			hasClient = false
			return
		}

		messageStr := string(buf[:n])
		var message map[string]interface{}
		err = json.Unmarshal([]byte(messageStr), &message)
		if err != nil {
			fmt.Println("Error parsing json:", err)
			return
		}

		if message["type"] == "introduce" {
			if hasClient {
				conn.Write([]byte(kick_off_message()))
				return
			}

			if message["cwd"] != nil {
				cwd := message["cwd"].(string)
				os.Chdir(cwd)
			}

			hasClient = true
			introduced = true
			conn.Write([]byte(ok_message()))

			clearScreen()
		} else if message["type"] == "run_command" {
			if !introduced {
				conn.Write([]byte(kick_off_message()))
				return
			}

			command := message["command"].(string)

			fmt.Println(">", command)
			var cmd *exec.Cmd

			if runtime.GOOS == "windows" {
				cmd = exec.Command("powershell.exe", "-Command", command)
			} else {
				shell := os.Getenv("SHELL")
				if shell == "" {
					shell = "/bin/sh"
				}
				cmd = exec.Command(shell, "-c", command)
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			err := cmd.Run()
			if err != nil {
				exitError, ok := err.(*exec.ExitError)
				if !ok {
					conn.Write([]byte(command_ran_message(1)))
				} else {
					conn.Write([]byte(command_ran_message(uint(exitError.ExitCode()))))
				}
			} else {
				conn.Write([]byte(command_ran_message(0)))
			}

			fmt.Println()
		} else if message["type"] == "bye" {
			hasClient = false
			conn.Write([]byte(ok_message()))
			os.Chdir(oldCwd)
			return
		}
	}
}
