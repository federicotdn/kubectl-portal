# kubectl-portal
A kubectl plugin that enables you to make HTTP requests directly to Kubernetes Services, via a local HTTP proxy:

```bash
$ kubectl portal
Creating proxy resources...
Resources created
Waiting for proxy to be ready...
Proxy is ready
Listening at localhost:7070
```

which can then be used with any HTTP/WebSocket client supporting HTTP proxies, such as cURL:
```bash
$ curl -x localhost:7070 http://my-service/my-endpoint
{"foo": "bar"}
```

## Installation

You can install kubectl-portal via different methods:

### Krew

TODO

### Build

You'll need to run the following commands:
```bash
git clone https://github.com/federicotdn/kubectl-portal.git
cd kubectl-portal
make build
```
After that, copy the `kubectl-portal` binary somewhere in your `$PATH`.

## Usage

You can run `kubectl portal --help` to get an overview of the command line options available. By default, running `kubectl portal` will perform the following steps:

1. A Pod will be created in the currently selected cluster, which will run the HTTP proxy.
2. `kubectl port-forward` will be executed, forwarding a local TCP port (by default, 7070) to the proxy Pod.
3. The command will wait until the user presses Ctrl-C to interrupt the operation.
4. The proxy Pod will be deleted.

While kubectl-portal is running, you'll need to configure your HTTP or WebSocket client to use the HTTP proxy at `http://localhost:7070`. The way this is done varies between clients, but here are some examples:

**cURL**
```bash
$ curl -x localhost:7070 http://my-service/my-endpoint
```

**websocat**
```
websocat -t --ws-c-uri=ws://my-service/my-ws-endpoint - ws-c:cmd:'socat - proxy:localhost:my-service:80,proxyport=7070'
```

Or, if the target server is using TLS (port 443):
```
websocat -t --ws-c-uri=wss://my-service/my-ws-endpoint - ws-c:ssl-connect:cmd:'socat - proxy:localhost:my-service:443,proxyport=7070'
```

(in some cases, adding `--tls-domain my-service` will be necessary).

## License

Distributed under the GNU General Public License, version 3.

See [LICENSE](LICENSE) for more information.
