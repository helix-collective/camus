package main

import (
	"strconv"
	"strings"
)

var cfgTemplate = `
global
        daemon

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



listen stats *:%STATS_PORT%
    mode http
    stats enable
    stats hide-version
    stats realm Haproxy\ Statistics
    stats uri /

frontend main *:%FRONT_PORT%
    default_backend thing-%APP_PORT%


backend thing-%APP_PORT%
    balance leastconn

    server app-server-%APP_PORT% 127.0.0.1:%APP_PORT% check inter 2000
`

func HaproxyConfig(statsPort int, frontPort int, appPort int) string {
	str := cfgTemplate

	str = replace(str, "STATS_PORT", statsPort)
	str = replace(str, "FRONT_PORT", frontPort)
	str = replace(str, "APP_PORT", appPort)

	return str
}

func replace(str string, varname string, num int) string {
	return strings.Replace(str, "%"+varname+"%", strconv.Itoa(num), -1)
}

// /usr/local/sbin/haproxy -f /etc/haproxy/haproxy.cfg -p /var/run/haproxy.pid -sf $(cat /var/run/haproxy.pid )
