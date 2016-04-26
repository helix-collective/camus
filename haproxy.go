package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	// This is the index of the process name for each of the
	// processes currently being tracked by HaProxy
	haProxyPxnameIndex = 0
	// This is the index of the status of each process
	// currently being tracked by HaProxy
	haProxyStatusIndex = 17
	// This is the status page that can be read when HaProxy is
	// running
	haProxyStatusPage = "http://localhost:%d/;csv"
	// This is a name given to the app being deployed when setting
	// it up in HaProxy. The name can be anything, it just
	// needs to be consistent from when the server is first
	// started, as it is used to define the currently active
	// deploy
	backendProcessName = "deployed-app-"
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



listen stats
    bind *:%STATS_PORT%
    mode http
    stats enable
    stats hide-version
    stats realm Haproxy\ Statistics
    stats uri /

frontend main
    bind *:%FRONT_PORT%
    default_backend %BACKEND_ENTRY%


backend %BACKEND_ENTRY%
    balance leastconn

    server app-server-%APP_PORT% 127.0.0.1:%APP_PORT% check inter 2000
`

func HaproxyConfig(statsPort int, frontPort int, appPort int) string {
	str := cfgTemplate

	str = replace(str, "STATS_PORT", statsPort)
	str = replace(str, "FRONT_PORT", frontPort)
	str = replace(str, "APP_PORT", appPort)
	str = strings.Replace(str, "%BACKEND_ENTRY%", nameBackendEntry(appPort), -1)

	return str
}

func replace(str string, varname string, num int) string {
	return strings.Replace(str, "%"+varname+"%", strconv.Itoa(num), -1)
}

// This looks through the HaProxy status page to determine
// which port is currently being pointed to by HaProxy.
// The haProxyPort is the port on which the status page
// can be accessed. Normally this is the end port for the
// server implementation.
func getPortMarkedAsSet(haProxyPort int) (int, error) {

	url := fmt.Sprintf(haProxyStatusPage, haProxyPort)
	resp, err := http.Get(url)
	if err != nil {
		// Couldn't connect to haproxy don't modify the deploy list
		return -1, fmt.Errorf("Could not connect to status page, got error: %s", err)
	}

	defer resp.Body.Close()

	records, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		return -1, err
	}

	for i, row := range records {

		// skip header
		if i == 0 {
			continue
		}

		// Find the first row that contains a Process name that
		// matches the app id.
		ok, port := parseBackendEntry(row)
		if ok {
			return port, nil
		}

	}

	return -1, errors.New(fmt.Sprintf("haProxy has does not have an application marked as set\n"))

}

func parseBackendEntry(row []string) (bool, int) {
	pxName := row[haProxyPxnameIndex]
	if strings.Contains(pxName, backendProcessName) {
		// Get the port number
		portStr := strings.TrimPrefix(pxName, backendProcessName)
		port, err := strconv.Atoi(portStr)

		if err != nil {
			log.Println("Could not parse port: ", portStr, " as int")
			return false, 0
		}

		// Check if that process is UP
		status := row[haProxyStatusIndex]
		if strings.Compare(status, "UP") == 0 {
			return true, port
		} else {
			log.Println("App on active port: ", port, " is down")
			return false, 0
		}

	}
	return false, 0
}

func nameBackendEntry(port int) string {
	return fmt.Sprintf("%s%d", backendProcessName, port)
}

// /usr/local/sbin/haproxy -f /etc/haproxy/haproxy.cfg -p /var/run/haproxy.pid -sf $(cat /var/run/haproxy.pid )
