FROM openresty/openresty:1.21.4.1-0-jammy

COPY openresty/default.conf /etc/nginx/conf.d/default.conf
COPY openresty/access.lua /openresty/access.lua
