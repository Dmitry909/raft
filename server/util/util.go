package util

import "strings"

var LocalIP = "127.0.0.1"

func ConvertPortsToSlice(ports string) []string {
	portList := strings.Split(ports, ",")
	hostPorts := make([]string, len(portList))

	for i, port := range portList {
		hostPorts[i] = LocalIP + ":" + strings.TrimSpace(port)
	}

	return hostPorts
}
