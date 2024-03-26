# kubectl-portal
![GitHub Tag](https://img.shields.io/github/v/tag/federicotdn/kubectl-portal)
![GitHub License](https://img.shields.io/github/license/federicotdn/kubectl-portal)
[![Static Badge](https://img.shields.io/badge/krew-install-aquamarine)](https://krew.sigs.k8s.io/)

A kubectl plugin that launches an HTTP proxy, enabling you to make requests to Services, Pods and any other host reachable from within your cluster.

```bash
$ kubectl portal
Creating proxy resources...
Resources created
Waiting for proxy to be ready...
Proxy is ready
Listening at localhost:7070
```

which can then be used with any HTTP/WebSocket client with HTTP proxy support, such as cURL with `-x`:
```bash
$ curl -x localhost:7070 http://my-service/my-endpoint
{"foo": "bar"}
```

_See how usage of kubectl portal compares to kubectl proxy/port-forward in the section [below](#comparison-with-kubectl-proxyport-forward)._

## Installation

You can install kubectl-portal via one of these methods:

### Krew _(recommended)_

Assuming you have [Krew](https://krew.sigs.k8s.io/) installed, run:
```bash
kubectl krew install portal
```

### Install from Binary
You can go to the [releases](https://github.com/federicotdn/kubectl-portal/releases) page, and download the binary that corresponds to your system.

### Build from Source

You'll need to run the following commands:
```bash
git clone https://github.com/federicotdn/kubectl-portal.git
cd kubectl-portal
make build
```
After that, copy the `kubectl-portal` binary somewhere in your `$PATH` - or run `./kubectl-portal` directly.

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
- If the Service is in the currently selected namespace: `http://my-service`.
- If the Service is in namespace `my-namespace`: `http://my-service.my-namespace`.

**For a Pod with name `my-pod`**:
- First, find the Pod's IP using: `kubectl get pod my-pod --template {{.status.podIP}}`. The URL then is `http://<IP>`.

Change `http://` to `https://`, `ws://` or `wss://` when needed.

## Comparison with kubectl proxy/port-forward

There is some overlap between how kubectl portal/proxy/port-forward can be used in order to send HTTP requests. These tables aim to clear it up to some degree:

<table width="100%">
  <thead>
    <tr>
      <th width="50%"><a href="https://kubernetes.io/docs/reference/kubectl/generated/kubectl_proxy/">kubectl proxy</a></th>
      <th width="50%">kubectl portal <i>(this project)</i></th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td width="50%">Allows local access to the Kubernetes API, and thus to endpoints exposed by Services and Pods as well.</td>
      <td width="50%">Allows local access to endpoints exposed by Services and Pods, plus any host reachable from within the cluster (e.g. a private database, dashboard, etc).</td>
    </tr>
    <tr>
      <td width="50%">Requires a URL in the form described <a href="https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster-services/#manually-constructing-apiserver-proxy-urls)">here</a>, such as:<br> <code>http://localhost:8001/api/v1/namespaces/default/services/my-service:80/proxy/my-endpoint</code>.</td>
      <td width="50%">Requires the user to configure the HTTP client to use the local proxy, and then use a URL such as:<br> <code>http://my-service/my-endpoint</code> (using the currently selected namespace).</td>
    </tr>
    <tr>
      <td width="50%">Must always provide the namespace as part of the URL.</td>
      <td width="50%">When sending requests to Services, specifying the namespace is optional.</td>
    </tr>
    <tr>
      <td width="50%">When sending requests to Services, must provide the Service's name.</td>
      <td width="50%">When sending requests to Services, must provide the Service's name.</td>
    </tr>
    <tr>
      <td width="50%">When sending requests to Pods, must provide the Pod's name.</td>
      <td width="50%">When sending requests to Pods, must provide the Pod's IP.</td>
    </tr>
    <tr>
      <td width="50%">Only needs to be executed once for sending requests to different targets.</td>
      <td width="50%">Only needs to be executed once for sending requests to different targets.</td>
    </tr>
  </tbody>
</table>

<br>

<table width="100%">
  <thead>
    <tr>
      <th width="50%"><a href="https://kubernetes.io/docs/reference/kubectl/generated/kubectl_port-forward/">kubectl port-forward</a></th>
      <th width="50%">kubectl portal <i>(this project)</i></th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td width="50%">Allows local access to any ports of any specific Pod.</td>
      <td width="50%">Allows local access to endpoints exposed by Services and Pods, plus any host reachable from within the cluster (e.g. a private database, dashboard, etc).</td>
    </tr>
    <tr>
      <td width="50%">Requires the user to send requests to a local address, e.g. <code>http://localhost:6000/my-endpoint</code>.</td>
      <td width="50%">Requires the user to configure the HTTP client to use the local proxy, and then use a URL such as:<br> <code>http://my-service/my-endpoint</code> (using the currently selected namespace).</td>
    </tr>
    <tr>
      <td width="50%">Specifying the Pod's namespace is optional.</td>
      <td width="50%">When sending requests to Services, specifying the namespace is optional.</td>
    </tr>
    <tr>
      <td width="50%">Does now allow for sending requests to a Service.</td>
      <td width="50%">When sending requests to Services, must provide the Service's name.</td>
    </tr>
    <tr>
      <td width="50%">Must provide the Pod's name to run the command.</td>
      <td width="50%">When sending requests to Pods, must provide the Pod's IP.</td>
    </tr>
    <tr>
      <td width="50%">Needs to be executed once for each different Pod the user wants to send requests to.</td>
      <td width="50%">Only needs to be executed once for sending requests to different targets.</td>
    </tr>
  </tbody>
</table>

## Related Projects
- [kubectl-plugin-socks5-proxy](https://github.com/yokawasa/kubectl-plugin-socks5-proxy) - structured like kubectl-portal, but runs a SOCKS5 proxy instead of an HTTP one.

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
