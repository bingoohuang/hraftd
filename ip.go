package hraftd

import (
	"strings"
	"sync"

	"github.com/bingoohuang/gonet"
)

var localAddrMap sync.Map // nolint

// IsLocalAddr tells if an addr is local or not.
func IsLocalAddr(addr string) bool {
	if addr == "" {
		return false
	}

	if yes, ok := localAddrMap.Load(addr); ok {
		return yes.(bool)
	}

	yes, _ := gonet.IsLocalAddr(addr)
	localAddrMap.Store(addr, yes)

	return yes
}

const (
	localhostIPv4 = "127.0.0.1"
	localhostIPv6 = "::1"
)

// ReplaceAddr2Local try to replace an local IP to localhost.
func ReplaceAddr2Local(ip string) (replaced, original string) {
	if IsLocalAddr(ip) {
		if gonet.IsIPv6(ip) {
			return localhostIPv6, ip
		}

		return localhostIPv4, ip
	}

	return ip, ip
}

// TryReplaceAddr2Local try to replace an local IP to localhost.
func TryReplaceAddr2Local(ip string) (replaced string) {
	replaced, _ = ReplaceAddr2Local(ip)

	return replaced
}

// ReplaceIP replace a single local address to main iface ip.
func ReplaceIP(address, primaryIP string) string {
	sepPos := strings.LastIndex(address, ":")

	host := address
	if sepPos > 0 {
		host = address[:sepPos]
	}

	if IsLocalAddr(host) {
		return strings.ReplaceAll(address, host, primaryIP)
	}

	return address
}

// ReplaceIPAll replace local addresses slice to main iface ips slice.
func ReplaceIPAll(addresses []string, primaryIP string) []string {
	for i, addr := range addresses {
		addresses[i] = ReplaceIP(addr, primaryIP)
	}

	return addresses
}

// InferHostIPv4 infers the host IP address.
func InferHostIPv4(ifaceName string) string {
	if ifaceName != "" {
		ipList, _ := HostIPv4(IfaceNameMatchExactly, ifaceName)
		if len(ipList) > 0 {
			return ipList[0]
		}

		ipList, _ = HostIPv4(IfaceNameMatchPrefix, ifaceName)
		if len(ipList) > 0 {
			return ipList[0]
		}
	}

	ipList, _ := HostIPv4(IfaceNameMatchPrefix, "en", "eth")

	if len(ipList) == 0 {
		ipList, _ = HostIPv4(IfaceNameMatchPrefix)
	}

	if len(ipList) == 1 {
		return ipList[0]
	}

	ipList2, _ := HostIPv4(IfaceNameMatchPrefix, "en0", "eth0")
	if len(ipList2) > 0 {
		return ipList2[0]
	}

	return ipList[0]
}

// IfaceNameMatchMode defines the mode for IfaceName matching.
type IfaceNameMatchMode int

const (
	// IfaceNameMatchPrefix matches iface name in prefix mode.
	IfaceNameMatchPrefix IfaceNameMatchMode = iota
	// IfaceNameMatchExactly matches iface name in exactly same mode
	IfaceNameMatchExactly
)

// HostIPv4 根据 primaryIfaceName 确定的名字，返回主IP PrimaryIP，以及以空格分隔的本机IP列表 ipList.
// PrimaryIfaceName 表示主网卡的名称，用于获取主IP(v4)，不设置时，从eth0(linux), en0(darwin)，或者第一个ip v4的地址.
// eg.  HostIP("eth0", "en0").
func HostIPv4(mode IfaceNameMatchMode, primaryIfaceNames ...string) (ipList []string, err error) {
	ips, err := gonet.ListIfaces(gonet.IPv4)
	if err != nil {
		return
	}

	ipList = make([]string, 0, len(ips))

	for _, addr := range ips {
		if mode.match(primaryIfaceNames, addr.IfaceName) {
			ipList = append(ipList, addr.IP.String())
		}
	}

	return
}

func (mode IfaceNameMatchMode) match(ifaceNames []string, name string) bool {
	if len(ifaceNames) == 0 {
		return true
	}

	for _, ifaceName := range ifaceNames {
		switch mode {
		case IfaceNameMatchPrefix:
			if strings.HasPrefix(name, ifaceName) {
				return true
			}
		case IfaceNameMatchExactly:
			if name == ifaceName {
				return true
			}
		}
	}

	return false
}
