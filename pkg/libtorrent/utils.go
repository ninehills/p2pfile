package libtorrent

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	apimachinery_net "k8s.io/apimachinery/pkg/util/net"
)

func GetAvailablePort(portRange string) (int, error) {
	if portRange == "" {
		return 0, fmt.Errorf("no port range specified: %v", portRange)
	}
	ports := strings.Split(portRange, "-")
	if len(ports) != 2 {
		return 0, fmt.Errorf("invalid port range specified: %v", portRange)
	}
	min, err := strconv.Atoi(ports[0])
	if err != nil {
		return 0, fmt.Errorf("invalid port range specified: %v", portRange)
	}
	max, err := strconv.Atoi(ports[1])
	if err != nil {
		return 0, fmt.Errorf("invalid port range specified: %v", portRange)
	}
	if min > max {
		return 0, fmt.Errorf("invalid port range specified: %v", portRange)
	}

	for i := min; i <= max; i++ {
		if isPortAvailable(i) {
			return i, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range: %v", portRange)
}

func isPortAvailable(port int) bool {
	// if use ipv6, need check udp6 and tcp6
	udpLn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Debugf("UDP Port %v is not available: %v", port, err)
		return false
	}
	err = udpLn.Close()
	if err != nil {
		log.Debugf("Couldn't stop listening on udp port %ve: %v", port, err)
		return false
	}

	tcpLn, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Debugf("TCP Port %v is not available: %v", port, err)
		return false
	}
	err = tcpLn.Close()
	if err != nil {
		log.Debugf("Couldn't stop listening on tcp port %ve: %v", port, err)
		return false
	}
	return true
}

func GetPublicIP(ip string) (net.IP, error) {
	var err error
	var publicIP net.IP
	if ip != "" {
		publicIP = net.ParseIP(ip)
	} else {
		publicIP, err = apimachinery_net.ChooseHostInterface()
		if err != nil {
			log.Fatalf("failed to get default public ip: %v", err)
		} else {
			log.Infof("get default public ip: %s", publicIP)
		}
	}
	return publicIP, nil
}

func isURI(arg string) bool {
	return strings.HasPrefix(arg, "magnet:") || strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://")
}
