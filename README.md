# kubectl-portal
A kubectl plugin that launches an HTTP proxy, enabling you to make requests to Services, Pods and any other host reachable from within your cluster.

```bash
$ kubectl portal
Creating proxy resources...
Resources created
Waiting for proxy to be ready...
Proxy is ready
Listening at localhost:7070
```

which can then be used with any HTTP/WebSocket client with HTTP proxy support, such as cURL:
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

You can run `kubectl portal --help` to get an overview of the command line flags available. By default, running `kubectl portal` will perform the following steps:

1. A Pod will be created in the currently selected cluster, which will run the HTTP proxy.
2. `kubectl port-forward` will be executed, forwarding a local TCP port (by default, 7070) to the proxy Pod.
3. The command will wait until the user presses Ctrl-C to interrupt the operation.
4. The proxy Pod will be deleted and the command will exit.

While kubectl-portal is running, you'll need to configure your HTTP or WebSocket client to use the HTTP proxy at `http://localhost:7070`. The way this is done varies between clients, but here are some examples:

### HTTP
#### [cURL](https://curl.se/)

```bash
$ curl -x localhost:7070 http://my-service/my-endpoint
```

(note that the `-x` flag is not related to `-X`).

### GNU Emacs + [Verb](https://github.com/federicotdn/verb)
```
* Example            :verb:
:properties:
:Verb-Proxy: localhost:7070
:end:
get http://my-service/my-endpoint
```

### WebSocket
#### [websocat](https://github.com/vi/websocat) + [socat](http://www.dest-unreach.org/socat/)

```
websocat -t --ws-c-uri=ws://my-service/my-ws-endpoint - ws-c:cmd:'socat - proxy:localhost:my-service:80,proxyport=7070'
```

Or, if the target server is using TLS (port 443):
```
websocat -t --ws-c-uri=wss://my-service/my-ws-endpoint - ws-c:ssl-connect:cmd:'socat - proxy:localhost:my-service:443,proxyport=7070'
```

(in some cases, adding `--tls-domain=my-service` will be necessary).

## URLs of Services and Pods

Figuring out the correct URL to use mostly depends on the [DNS name](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/) of the resource we want to contact. Here's a short guide:

**For a Service `my-service`**:
- If the Service is in the selected namespace: `http://my-service`.
- If the Service is in namespace `my-namespace`: `http://my-service.my-namespace`.

**For a Pod with IP `10.244.2.1`**:
- If the Pod is in namespace `my-namespace`: `http://10-244-2-1.my-namespace` (Note: namespace must always be specified)

## Comparison with kubectl proxy

There is overlap between the functionality of kubectl proxy and kubectl portal. This table aims to clear it up to some degree:

| [kubectl proxy](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_proxy/) | kubectl portal *(this project)* |
| --- | --- |
| Provided by kubectl itself. | Installed using Krew or by building from source. |
| Allows local access to the Kubernetes API, and thus to endpoints exposed by Services and Pods as well. | Allows local access to endpoints exposed by Services and Pods, plus any host reachable from within the cluster (e.g. a private database, dashboard, etc). |
| Requires a URL in the form described [here](https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster-services/#manually-constructing-apiserver-proxy-urls), such as:<br> `http://localhost:8001/api/v1/namespaces/default/services/my-service:80/proxy/my-endpoint`. | Requires the user to configure the HTTP client to use the local proxy, and then use a URL such as:<br> `http://my-service/my-endpoint` (using the selected namespace). |
| Must always provide the namespace as part of the URL (e.g. `default`). | When connecting to a Service, specifying the namespace is optional, if omitted the value of `--namespace` will be used, or the current context's namespace (i.e. the selected namespace). |
| When connecting to Services, must provide the Service's name | When connecting to Services, must provide the Service's name.
| When connecting to Pods, must provide the Pod's name. | When connecting to Pods, must provide the Pod's DNS name (e.g. `10-244-2-1.default.pod`).

## Additional Links

Additional useful information can be found here:

- [Kubernetes - DNS for Services and Pods](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/)
- [Kubernetes - Proxies in Kubernetes](https://kubernetes.io/docs/concepts/cluster-administration/proxies/)
- [Kubernetes - Access Services Running on Clusters](https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster-services/)
- [MDN - Proxy servers and tunneling](https://developer.mozilla.org/en-US/docs/Web/HTTP/Proxy_servers_and_tunneling)
- [Wikipedia - HTTP tunnel](https://en.wikipedia.org/wiki/HTTP_tunnel)

## License

Distributed under the GNU General Public License, version 3.

See [LICENSE](LICENSE) for more information.
