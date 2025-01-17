import socket
import json
import threading


def listen_to_server(sock):
    while True:
        try:
            data = sock.recv(1024)
            if not data:
                break
            print(data.decode("utf-8").strip())
        except ConnectionResetError:
            print("Connection lost.")
            break


def main():
    server_host = "127.0.0.1"
    server_port = 45673

    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.connect((server_host, server_port))

    listener_thread = threading.Thread(target=listen_to_server, args=(sock,))
    listener_thread.daemon = True
    listener_thread.start()

    try:
        while True:
            commands = input("Enter commands separated by commas: ").strip().split(",")

            message = {"type": "run", "commands": commands}

            sock.sendall(json.dumps(message).encode("utf-8"))

    except KeyboardInterrupt:
        print("Exiting.")

    sock.close()


if __name__ == "__main__":
    main()
