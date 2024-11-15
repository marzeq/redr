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
	"time"
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

func setupSignalHandler(l net.Listener) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		clearScreen()
		l.Close()
		if currentCmd != nil && currentCmd.Process != nil {
			currentCmd.Process.Kill()
		}
		os.Exit(0)
	}()
}

const (
	MAX_BYTES = 16 * 1024
	PORT      = 45673
)

var (
	hasClient          = false
	currentCmd         *exec.Cmd
	currentConn        net.Conn
	clientDisconnected = make(chan bool)
)

func main() {
	clearScreen()
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

		if hasClient {
			currentConn.Write([]byte(kick_off_message()))

			currentConn.Close()
			currentConn = nil
			hasClient = false

			if currentCmd != nil && currentCmd.Process != nil {
				currentCmd.Process.Kill()
				time.Sleep(500 * time.Millisecond)
			}
		}

		currentConn = conn
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		clientDisconnected <- true
	}()

	buf := make([]byte, MAX_BYTES)
	introduced := false
	oldCwd, _ := os.Getwd()
	run_next_after_failure := false

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if n == 0 || err.Error() == "use of closed network connection" {
				hasClient = false
				return
			}
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

			if message["run_next_after_failure"] != nil {
				run_next_after_failure = message["run_next_after_failure"].(bool)
			}

			hasClient = true
			introduced = true
			conn.Write([]byte(ok_message()))

			clearScreen()
		} else if message["type"] == "run_commands" {
			if !introduced {
				conn.Write([]byte(kick_off_message()))
				return
			}

			commandsUnknown := message["commands"].([]interface{})
			commands := make([]string, len(commandsUnknown))
			for i, v := range commandsUnknown {
				commands[i] = v.(string)
			}

			for _, command := range commands {
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
				tobreak := false
				if err != nil {
					exitError, ok := err.(*exec.ExitError)
					if !ok {
						conn.Write([]byte(command_ran_message(1)))
					} else {
						conn.Write([]byte(command_ran_message(uint(exitError.ExitCode()))))
					}

					if !run_next_after_failure {
						tobreak = true
					}
				} else {
					conn.Write([]byte(command_ran_message(0)))
				}

				n, err := conn.Read(buf)
				if err != nil {
					fmt.Println("Read error:", err)
					return
				}

				messageStr := string(buf[:n])
				var message map[string]interface{}
				err = json.Unmarshal([]byte(messageStr), &message)
				if err != nil {
					fmt.Println("Error parsing json:", err)
					return
				}

				if message["type"] == "bye" {
					hasClient = false
					conn.Write([]byte(ok_message()))
					os.Chdir(oldCwd)
					return
				}

				if message["type"] != "ok" {
					fmt.Println("Unexpected message:", message)
					return
				}

				fmt.Println()

				if tobreak {
					break
				}
			}
		} else if message["type"] == "bye" {
			hasClient = false
			conn.Write([]byte(ok_message()))
			os.Chdir(oldCwd)
			return
		}
	}
}
