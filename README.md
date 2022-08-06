# qsocks
Encapsulate SOCKS5 inside QUIC  

# Workflow
client -> lcoal quic forward -> remote quic socks5 -> target

dataflows are since bidirectional

# Examples:
```shell
# start proxy server at remote server
podman run --network host \
	ghcr.io/zizon/qsocks:master \
	/usr/local/bin/qsocks \
	sqserver --listen 0.0.0.0:1080
  
# start a local client
# RemoteServerIP = the public accessible ip of server
# RemoteServerPort = the port server listening on, 1080 for previous server example
podman run --rm --name qsocks \
	--network host \
	ghcr.io/zizon/qsocks:master \
	/usr/local/bin/qsocks \
	qsocks \
	--verbose 1 \
	--timeout 10s \
	--listen 127.0.0.1:1080 \
	--streams 5 \
	--connect sqserver://${RemoteServerIP}:${RemoteServerPort}
```

# Issue
Currently, the socks5 part only implement a limited set of spec,mostly the `CONNECT` command.  
`UDP assoicate` and `BIND` are not supported  
