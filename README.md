# qsocks
A proof of concept Tunning tools.  
With two party involed: a local `SOCKS5` server and a remote `QUIC` server.  

from user facing front,it looks like a normal socks5 tunnel,  
except that transports are now QUIC ,not TCP.   

# Workflow
client -> lcoal socks5 -> remote quic -> target

dataflows are since bidirectional

# Examples:
```shell
Examples:

start a quic proxy server
./qsocks sqserver -l 0.0.0.0:10086

start a local socks5 server listening 10086 wich connect the remote quic server
which listen at port 10010
./qsocks qsocks -l 0.0.0.0. -c sqserver://{address.of.quic.server}:10010

start a http proxy server
./qsocks http -l 0.0.0.0:8080
		

Available Commands:
  help        Help about any command
  http        run a http proxy server
  qsocks      start a local socks5 server
  sqserver    run a quic proxy server

Flags:
  -h, --help          help for this command
      --mode          qsocks command mode,deprecated
  -v, --verbose int   log verbose level from 0 - 4, higher means more verbose, default 2 (default 2)
```
# Issue
Currently, the socks5 part only implement a limited set of spec,mostly the `CONNECT` command.  
`UDP assoicate` and `BIND` are not supported  
