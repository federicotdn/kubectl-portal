FROM openresty/openresty:1.21.4.1-0-jammy

COPY nginx/default.conf /etc/nginx/conf.d/default.conf
