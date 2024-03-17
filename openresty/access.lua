local path = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
local f = assert(io.open(path, "rb"))
local namespace = f:read("*all")
f:close()

ngx.log(ngx.ERR, "NS")
ngx.log(ngx.ERR, namespace)

-- ngx.req.set_header('Host', 'example.com')
