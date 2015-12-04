
global

#        log 127.0.0.1 local0 info
#        chroot /var/lib/haproxy
#        user haproxy
#        group haproxy
#
defaults
        #log     global
        mode    http
        #option  httplog
        #option  dontlognull
        timeout connect 5000
        timeout client 50000
        timeout server 50000
        #errorfile 400 /etc/haproxy/errors/400.http
        #errorfile 403 /etc/haproxy/errors/403.http
        #errorfile 408 /etc/haproxy/errors/408.http
        #errorfile 500 /etc/haproxy/errors/500.http
        #errorfile 502 /etc/haproxy/errors/502.http
        #errorfile 503 /etc/haproxy/errors/503.http
        #errorfile 504 /etc/haproxy/errors/504.http

frontend main
    bind *:%FRONTPORT%
    default_backend node


backend node
    balance leastconn

    server app-server 127.0.0.1:%APPPORT% check inter 2000

# /usr/local/sbin/haproxy -f /etc/haproxy/haproxy.cfg -p /var/run/haproxy.pid -sf $(cat /var/run/haproxy.pid )
