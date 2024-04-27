package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type PortCounter struct {
	connectID      string
	usedPorts      uint64
	availablePorts uint64
}

func parseIP(hexIP string) (net.IP, error) {
	var byteIP []byte
	byteIP, err := hex.DecodeString(hexIP)
	if err != nil {
		return nil, fmt.Errorf("parse ip err: Cannot parse socket field in %q: %w", hexIP, err)
	}
	switch len(byteIP) {
	case 4:
		return net.IP{byteIP[3], byteIP[2], byteIP[1], byteIP[0]}, nil
	case 16:
		i := net.IP{
			byteIP[3], byteIP[2], byteIP[1], byteIP[0],
			byteIP[7], byteIP[6], byteIP[5], byteIP[4],
			byteIP[11], byteIP[10], byteIP[9], byteIP[8],
			byteIP[15], byteIP[14], byteIP[13], byteIP[12],
		}
		return i, nil
	default:
		return nil, fmt.Errorf("parse ip err: Unable to parse IP %s: %v", hexIP, nil)
	}
}

func GetLocalPortRange() (start, end, total uint64, err error) {
	path := "/proc/sys/net/ipv4/ip_local_port_range"
	file, err := os.Open(path)
	if err != nil {
		return start, end, total, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return start, end, total, err
	}

	s := strings.Fields(string(bytes))
	if len(s) != 2 {
		return start, end, total, fmt.Errorf("invalid local port range: %s", string(bytes))
	}

	start, err = strconv.ParseUint(s[0], 10, 64)
	if err != nil {
		return start, end, total, err
	}

	end, err = strconv.ParseUint(s[1], 10, 64)
	if err != nil {
		return start, end, total, err
	}

	if start > end {
		return start, end, total, fmt.Errorf("invalid local port range: %s", string(bytes))
	}

	return start, end, end - start, nil
}

func GetMiniTcpAvailablePorts(netVersion, topN int) (portCounters []*PortCounter, err error) {
	var (
		procFilePath string
		zeroIP       string
		jointMark    = "->"
	)

	switch netVersion {
	case 4:
		procFilePath = "/proc/net/tcp"
		zeroIP = "00000000"
	case 6:
		procFilePath = "/proc/net/tcp6"
		zeroIP = "00000000000000000000000000000000"
	default:
		return nil, fmt.Errorf("NetVersion err, only suport 4 or 6, wrong version: %d", netVersion)
	}

	file, err := os.Open(procFilePath)
	if err != nil {
		return nil, fmt.Errorf("open %s err: %w", procFilePath, err)
	}
	defer file.Close()

	startPort, endPort, totalPorts, err := GetLocalPortRange()
	if err != nil {
		return nil, err
	}

	// map["localAddress"] = struct{}{}
	listenSockets := make(map[string]struct{}, 6)
	// map["localIP"] = []uint{port1, port2}
	listenPorts := make(map[string][]uint64, 4)
	// map["localIP->remoteAddress"] = PortCounter{usedPorts, availablePorts}
	portsInfo := make(map[string]*PortCounter, 1024)
	connectIDs := make([]string, 0, 1024)
	portCounters = make([]*PortCounter, 0, topN)

	lr := io.LimitReader(file, 1024*1024*1024)
	s := bufio.NewScanner(lr)
	s.Scan() // skip first line with headers
	for s.Scan() {
		line := s.Text()
		fields := strings.Fields(line)
		if len(fields) < 12 {
			return nil, fmt.Errorf("error parsing file: less than 12 columns found %q", line)
		}
		localAddress := fields[1]
		remoteAddress := fields[2]
		connStat := fields[3]

		// if connect is remote start, continue
		if _, ok := listenSockets[localAddress]; ok {
			continue
		}

		la := strings.Split(localAddress, ":")
		if len(la) != 2 {
			return nil, fmt.Errorf("error parsing local address: less than 2 columns found %q", localAddress)
		}
		localIP := la[0]
		localPort := la[1]

		// if connect is remote start, continue
		if _, ok := listenSockets[zeroIP+":"+localPort]; ok {
			continue
		}

		localPortInt, err := strconv.ParseUint(localPort, 16, 64)
		if err != nil {
			return nil, err
		}

		// "0A" means Listen
		if connStat == "0A" {
			listenSockets[localAddress] = struct{}{}
			if _, ok := listenPorts[localIP]; !ok {
				listenPorts[localIP] = []uint64{localPortInt}
			} else {
				listenPorts[localIP] = append(listenPorts[localIP], localPortInt)
			}
			continue
		}

		connectID := fmt.Sprintf("%s%s%s", localIP, jointMark, remoteAddress)
		if _, ok := portsInfo[connectID]; ok {
			portsInfo[connectID].usedPorts += 1
		} else {
			portsInfo[connectID] = &PortCounter{connectID, 1, totalPorts}
			connectIDs = append(connectIDs, connectID)
		}

		if localPortInt >= startPort && localPortInt <= endPort {
			portsInfo[connectID].availablePorts -= 1
		}
	}

	if len(portsInfo) < topN {
		topN = len(portsInfo)
	}

	for i := 0; i < topN; i++ {
		minIndex := i
		for j := i + 1; j < len(connectIDs); j++ {
			if portsInfo[connectIDs[j]].availablePorts < portsInfo[connectIDs[minIndex]].availablePorts {
				minIndex = j
			}
		}
		if minIndex != i {
			connectIDs[i], connectIDs[minIndex] = connectIDs[minIndex], connectIDs[i]
		}
	}

	for i := 0; i < topN; i++ {
		connectID := connectIDs[i]
		ld := strings.Split(connectID, jointMark)
		localIP := ld[0]
		localRealIP, err := parseIP(localIP)
		if err != nil {
			return nil, err
		}

		remoteAddress := ld[1]
		ra := strings.Split(remoteAddress, ":")
		if len(ra) != 2 {
			return nil, fmt.Errorf("error parsing remote address: less than 2 columns found %q", remoteAddress)
		}
		remoteIP := ra[0]
		remoteRealIP, err := parseIP(remoteIP)
		if err != nil {
			return nil, err
		}

		remotePort := ra[1]
		remotePortInt, err := strconv.ParseUint(remotePort, 16, 64)
		if err != nil {
			return nil, err
		}

		if ports, ok := listenPorts[localIP]; ok {
			portsInfo[connectID].usedPorts += uint64(len(ports))
			for _, port := range ports {
				if port >= startPort && port <= endPort {
					portsInfo[connectID].availablePorts -= 1
				}
			}
		}

		portsInfo[connectID].connectID = fmt.Sprintf("%s->%s:%d", localRealIP, remoteRealIP, remotePortInt)
		portCounters = append(portCounters, portsInfo[connectID])
	}
	return
}

func OutputPromMetric(topN int, portCounters []*PortCounter) {
	for i := 0; i < topN; i++ {
		fmt.Printf("tcp_used_ports_total{connect=\"%s\"} %d\n", portCounters[i].connectID, portCounters[i].usedPorts)
		fmt.Printf("tcp_available_ports_total{connect=\"%s\"} %d\n", portCounters[i].connectID, portCounters[i].availablePorts)
	}
}
