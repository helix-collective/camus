package main

import (
	"strings"
	"testing"
)

var dummyOutput = strings.Trim(`
COMMAND     PID           USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
UserEvent   957 jameskozianski    6u  IPv4 0xccadfbbea68a6fb3      0t0  UDP *:*
SystemUIS  1032 jameskozianski    6u  IPv4 0xccadfbbea68a7ef3      0t0  UDP *:*
Google     1045 jameskozianski   17u  IPv4 0xccadfbbeaf688fe3      0t0  TCP 192.168.1.7:49241->74.125.68.84:443 (ESTABLISHED)
VBoxSVC   17449 jameskozianski   11u  IPv4 0xccadfbbeab08b0db      0t0  UDP *:*
VBoxHeadl 17506 jameskozianski   11u  IPv4 0xccadfbbeab08b0db      0t0  UDP *:*
VBoxHeadl 17506 jameskozianski   17u  IPv4 0xccadfbbea7638e33      0t0 ICMP *:*
VBoxHeadl 17506 jameskozianski   21u  IPv4 0xccadfbbec7e61fe3      0t0  TCP localhost:2022 (LISTEN)
VBoxHeadl 17506 jameskozianski   22u  IPv4 0xccadfbbea7a010db      0t0  UDP *:64309
VBoxHeadl 17506 jameskozianski   23u  IPv4 0xccadfbbeafe79383      0t0  UDP *:54778
VBoxHeadl 17506 jameskozianski   24u  IPv4 0xccadfbbea7a64813      0t0  UDP *:60920
VBoxHeadl 17506 jameskozianski   25u  IPv4 0xccadfbbea762d25b      0t0  UDP *:55046
VBoxNetDH 17507 jameskozianski   11u  IPv4 0xccadfbbeab08b0db      0t0  UDP *:*
camus     53404 jameskozianski    3u  IPv6 0xccadfbbea6f1811b      0t0  TCP *:9966 (LISTEN)
camus     53404 jameskozianski    8u  IPv4 0xccadfbbec7e5b7fb      0t0  TCP localhost:61992->localhost:8001 (ESTABLISHED)
camus     53404 jameskozianski    9u  IPv4 0xccadfbbec7e437fb      0t0  TCP localhost:61993->localhost:8002 (ESTABLISHED)
node      53530 jameskozianski   12u  IPv4 0xccadfbbeb791d7fb      0t0  TCP *:8001 (LISTEN)
node      53530 jameskozianski   13u  IPv4 0xccadfbbebaa2c7fb      0t0  TCP localhost:8001->localhost:61992 (ESTABLISHED)
Google    53594 jameskozianski  140u  IPv4 0xccadfbbeaf7af7fb      0t0  TCP 192.168.1.169:62945->74.125.130.188:5228 (ESTABLISHED)
node      57065 jameskozianski   14u  IPv4 0xccadfbbec7e5c7fb      0t0  TCP *:8002 (LISTEN)
node      57065 jameskozianski   15u  IPv4 0xccadfbbeba9fafe3      0t0  TCP localhost:8002->localhost:61993 (ESTABLISHED)
`, "\n")

func TestLookupPorts(t *testing.T) {
	procs, err := parseLsofOutput(dummyOutput)
	if err != nil {
		t.Fail()
	}
	found := false
	for _, proc := range procs {
		if proc.Port == 8001 {
			found = true
			if proc.Name != "node" {
				t.Error("program listening on 8001 should be node")
			}
		}
	}
	if !found {
		t.Error("Failed to find node listening on 8001")
	}
}
