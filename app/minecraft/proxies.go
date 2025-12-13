package minecraft

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/txthinking/socks5"
)

// proxyInfo ...
type proxyInfo struct {
	IP       string
	Port     string
	Username string
	Password string
	Auth     bool
}

// loadProxies ...
func loadProxies(filename string) ([]proxyInfo, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var (
		proxies []proxyInfo
		scanner = bufio.NewScanner(file)
	)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), ":")
		if len(s) == 2 {
			proxies = append(proxies, proxyInfo{IP: s[0], Port: s[1]})
		} else if len(s) == 4 {
			proxies = append(proxies, proxyInfo{Username: s[0], Password: s[1], IP: s[2], Port: s[3], Auth: true})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return proxies, nil
}

// randomProxy ...
func randomProxy(proxies []proxyInfo) proxyInfo {
	return proxies[rand.Intn(len(proxies))]
}

// newClientWithProxy ...
func newClientWithProxy(proxy proxyInfo) *socks5.Client {
	if proxy.Auth {
		return &socks5.Client{
			Server:     fmt.Sprintf("%s:%s", proxy.IP, proxy.Port),
			UserName:   proxy.Username,
			Password:   proxy.Password,
			UDPTimeout: 60,
		}
	}
	return &socks5.Client{
		Server:     fmt.Sprintf("%s:%s", proxy.IP, proxy.Port),
		UDPTimeout: 60,
	}
}
