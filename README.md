# qsocks
A proof of concept Tunning tools.  
With two party involed: a local `SOCKS5` server and a remote `QUIC` server.  

from user facing front,it looks like a normal socks5 tunnel,  
except that transports are now QUIC ,not TCP.   

# Workflow
client -> lcoal socks5 -> remote quic -> target

dataflows are since bidirectional

# Example
```shell
./qsocks -mode qsocks \
        -listen 0.0.0.0:10086 \
        -connect your.server:10010 
```
This will start a local socks5 server listening port 10086 on all interface,  
and connect/forward to remote quic server `your.server` whicl listing at port `10010`.  

```shell
./qsocks -mode sqsocks \
        -listen 0.0.0.0:10010 
```
This will start the quic server listeing local udp port 10010 on all interface.    
refering to `your.server:10010` above  

And now,all `TCP` connections are runing inside QUIC streams between this two party.  

# Issue
Currently, the socks5 part only implement a limited set of spec,mostly the `CONNECT` command.  
`UDP assoicate` and `BIND` are not supported  
