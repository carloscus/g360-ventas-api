"""Forwarder TCP: expone el API (que corre en WSL2, escuchando en el relay
localhost de WSL en 127.0.0.1:8091) a la intranet en 0.0.0.0:8090, sin necesitar
admin (netsh portproxy) ni resolver la IP de WSL (que cambia en cada arranque y
cuelga wsl.exe en procesos sin consola).

Topology:
  [cliente intranet] -> http://<IP-LAN>:8090 -> forwarder.py
                        -> 127.0.0.1:8091 (WSL2 localhost forwarding)
                        -> API Go en WSL2 :8091

Uso: python forwarder.py [listen_port] [upstream_host] [upstream_port]
"""
import socket
import sys
import threading

LISTEN_PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 8090
UPSTREAM = (
    sys.argv[2] if len(sys.argv) > 2 else "127.0.0.1",
    int(sys.argv[3]) if len(sys.argv) > 3 else 8091,
)


def pipe(src: socket.socket, dst: socket.socket) -> None:
    try:
        while True:
            data = src.recv(65536)
            if not data:
                break
            dst.sendall(data)
    except OSError:
        pass
    finally:
        for s in (src, dst):
            try:
                s.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass


def handle(client: socket.socket) -> None:
    try:
        up = socket.create_connection(UPSTREAM, timeout=10)
    except OSError as e:
        print(f"upstream {UPSTREAM} fallo: {e}", flush=True)
        client.close()
        return
    threading.Thread(target=pipe, args=(client, up), daemon=True).start()
    threading.Thread(target=pipe, args=(up, client), daemon=True).start()


def main() -> None:
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("0.0.0.0", LISTEN_PORT))
    srv.listen(64)
    print(f"forwarder 0.0.0.0:{LISTEN_PORT} -> {UPSTREAM[0]}:{UPSTREAM[1]}", flush=True)
    while True:
        client, _ = srv.accept()
        threading.Thread(target=handle, args=(client,), daemon=True).start()


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(0)
