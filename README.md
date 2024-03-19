# kubectl-portal
An HTTP proxy for connecting directly to Kubernetes Services

WS
```
websocat -t --ws-c-uri=ws://echo.websocket.org/ - ws-c:cmd:'socat - proxy:127.0.0.1:echo.websocket.org:443,proxyport=7071'
```

WS + TLS
```
websocat -t --ws-c-uri=wss://echo.websocket.org/ - ws-c:ssl-connect:cmd:'socat - proxy:127.0.0.1:echo.websocket.org:443,proxyport=7071' --tls-domain echo.websocket.org
```
