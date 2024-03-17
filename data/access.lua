-- Read the cluster domain from environment
local cluster_domain = os.getenv("KUBECTL_PORTAL_CLUSTER_DOMAIN")

-- Read the namespace from the environment
local namespace = os.getenv("KUBECTL_PORTAL_NAMESPACE")

-- Figure out what the Host value looks like
-- Options:
-- 1. my-service (assume the same namespace)
-- 2. my-service.my-namespace
-- 3. my-service.my-namespace.svc
-- 4. Other, with 3 periods or more (possibly a FQDN)
-- Based on this, try to convert it to a FQDN.
--
-- Note that:
-- + Namespace names follow the RFC 1123 format (no periods)
-- + Service names follow the RFC 1035 format (no periods)
-- These steps are needed due to how DNS names are resolved in
-- Nginx/OpenResty. See the default.conf file for more information.

local host = ngx.req.get_headers()["Host"]
local _, c = string.gsub(host, "%.", "")

if c == 0 then
  -- Option 1
  ngx.req.set_header("Host", host .. "." .. namespace .. ".svc." .. cluster_domain)
elseif c == 1 then
  -- Option 2
  ngx.req.set_header("Host", host .. ".svc." .. cluster_domain)
elseif c == 2 then
  -- Option 3
  ngx.req.set_header("Host", host .. "." .. cluster_domain)
end
-- For Option 4, do nothing
