# qsocks
Encapsulate SOCKS5 inside QUIC  

# Workflow
client -> lcoal quic forward -> remote quic socks5 -> target

dataflows are since bidirectional

# Examples:
```shell
Examples:

start a quic proxy server
./qsocks sqserver -l 0.0.0.0:10086

start a local socks5 server listening 10086 wich connect the remote quic server
which listen at port 10010
./qsocks qsocks -l 0.0.0.0. -c sqserver://{address.of.quic.server}:10010
```

# Issue
Currently, the socks5 part only implement a limited set of spec,mostly the `CONNECT` command.  
`UDP assoicate` and `BIND` are not supported  
