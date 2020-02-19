package hraftd

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/raft"
)

// Arg Command line parameters
type Arg struct {
	Bootstrap bool

	InMem       bool
	RaftAddr    string
	RaftAdv     string
	RaftNodeDir string
	NodeID      NodeID
	HTTPAddr    string
	HTTPAdv     string
	JoinAddrs   string

	IfaceName string // 绑定网卡名称

	ApplyInterceptor ApplyInterceptor `json:"-"`
	JoinAddrSlice    []string

	LogDealer

	hostIP string
}

// MakeArg makes a Arg
func MakeArg() *Arg {
	return &Arg{LogDealer: MakeLogDealer()}
}

// FlagProvider defines the interface for flag definitions required for hraftd.
type FlagProvider interface {
	BoolVar(p *bool, name string, value bool, usage string)
	StringVar(p *string, name string, value string, usage string)
}

// DefineFlags define raft args
func DefineFlags(p FlagProvider) *Arg {
	a := MakeArg()

	p.BoolVar(&a.InMem, "rmem", true, "Use in-memory storage for Raft")
	p.StringVar(&a.HTTPAddr, "haddr", "", "HTTP server bind address")
	p.StringVar(&a.HTTPAdv, "hadv", "", "Advertised HTTP address. If not set, same as HTTP server")
	p.StringVar(&a.RaftAddr, "raddr", "", "Raft communication bind address. If not set, same as haddr(port+1000)")
	p.StringVar(&a.RaftAdv, "radv", "", "Advertised Raft communication address. If not set, same as Raft bind")
	p.StringVar(&a.RaftNodeDir, "rdir", "", "Raft data directory, default to ~/.hraftd/{id}")
	p.StringVar(&a.JoinAddrs, "rjoin", "", "Set raft cluster join addresses separated by comma, if any")
	p.StringVar(&a.IfaceName, "iface", "", "iface name to bind")

	return a
}

// ViperProvider defines the args getter provider.
type ViperProvider interface {
	SetDefault(key string, value interface{})
	GetBool(key string) bool
	GetString(key string) string
}

// CreateArg creates Arg by ViperProvider implementation.
func CreateArg(p ViperProvider) *Arg {
	a := MakeArg()

	p.SetDefault("rmem", true)

	a.InMem = p.GetBool("rmem")
	a.HTTPAddr = p.GetString("haddr")
	a.HTTPAdv = p.GetString("hadv")
	a.RaftAddr = p.GetString("raddr")
	a.RaftAdv = p.GetString("radv")
	a.RaftNodeDir = p.GetString("rdir")
	a.JoinAddrs = p.GetString("rjoin")

	return a
}

// Fix fixes the arg for some defaults.
func (a *Arg) Fix() {
	a.hostIP = InferHostIPv4(a.IfaceName)

	a.fixAddr()
	a.parseFlagRaftNodeID()
	a.parseFlagRaftDir()
	a.parseBootstrap()
}

func (a *Arg) fixAddr() {
	switch {
	case a.RaftAddr == "" && a.HTTPAddr == "":
		a.RaftAddr = a.hostIP + ":12000"
		a.HTTPAddr = a.hostIP + ":11000"
	case a.RaftAddr == "" && a.HTTPAddr != "":
		host, port, err := net.SplitHostPort(a.HTTPAddr)
		if err != nil {
			panic(err)
		}

		por, _ := strconv.Atoi(port)
		if por > 35565-1000 {
			log.Panicf("port %d is too large (<= 34565)\n", por)
		}

		host = If(a.isLocalHost(host), "", host)

		a.HTTPAddr = fmt.Sprintf("%s:%d", host, por)      // nolint gomnd
		a.RaftAddr = fmt.Sprintf("%s:%d", host, por+1000) // nolint gomnd
	case a.RaftAddr != "" && a.HTTPAddr == "":
		host, port, err := net.SplitHostPort(a.RaftAddr)
		if err != nil {
			panic(err)
		}

		por, _ := strconv.Atoi(port)

		// sudo setcap cap_net_bind_service=ep some-binary
		// In Linux, the things root can do have been broken up into a set of capabilities.
		// CAP_NET_BIND_SERVICE is the ability to bind to ports <= 1024.
		if por < 1024+1000 {
			log.Panicf("port %d is too large (>= 2024)\n", por)
		}

		host = If(a.isLocalHost(host), "", host)
		a.HTTPAddr = fmt.Sprintf("%s:%d", host, por-1000) // nolint gomnd
		a.RaftAddr = fmt.Sprintf("%s:%d", host, por)      // nolint gomnd
	}
}

func (a *Arg) isLocalHost(host string) bool {
	return host == "" || host == "127.0.0.1" || host == "localhost" || host == a.hostIP
}

// NodeID is the raft node ID
type NodeID string

// BindAddr is the address for bind
type BindAddr string

// URL returns the HTTP access URL with relative path
func (a BindAddr) URL(path string) string {
	host, port, _ := net.SplitHostPort(string(a))
	path = strings.TrimPrefix(path, "/")

	return fmt.Sprintf("http://%s:%s/%s", host, port, path)
}

// URL returns the HTTP access URL with relative path
func (r NodeID) URL(relativePath string) string { return BindAddr(r.HTTPAddr()).URL(relativePath) }

// URLRaftCluster is http://httpAddr/raft/cluster
func (r NodeID) URLRaftCluster() string { return r.URL(RaftPath + "/cluster") }

// URLRaftState is http://httpAddr/raft/state
func (r NodeID) URLRaftState() string { return r.URL(RaftPath + "/state") }

// URLRaftJoin is http://httpAddr/raft/join
func (r NodeID) URLRaftJoin() string { return r.URL(RaftPath + "/join") }

// URLRaftJoin is http://httpAddr/raft/join
func (a BindAddr) URLRaftJoin() string { return a.URL(RaftPath + "/join") }

// HTTPAddr returns the HTTP bind address in the ID
func (r NodeID) HTTPAddr() string { return strings.SplitN(string(r), ",", -1)[0] }

// RaftAddr returns the Raft bind addr in the ID
func (r NodeID) RaftAddr() string { return strings.SplitN(string(r), ",", -1)[1] }

// Fix fixes the ID component to full host:port
func (r *NodeID) Fix(host string) {
	_, hPort, _ := net.SplitHostPort(r.HTTPAddr())
	_, rPort, _ := net.SplitHostPort(r.RaftAddr())

	*r = NodeID(fmt.Sprintf("%s:%s,%s:%s", host, hPort, host, rPort))
}

func (a *Arg) parseFlagRaftNodeID() {
	a.NodeID = NodeID(a.HTTPAddr + "," + a.RaftAddr)
	a.NodeID.Fix(a.hostIP)
}

// nolint gomnd
func (a *Arg) parseFlagRaftDir() {
	if a.RaftNodeDir != "" {
		return
	}

	basePath := "./"
	if usr, err := user.Current(); err == nil {
		basePath = usr.HomeDir
	}

	a.RaftNodeDir = filepath.Join(basePath, ".hraftd", string(a.NodeID))
}

func (a *Arg) parseBootstrap() {
	a.JoinAddrSlice = make([]string, 0)

	for _, addr := range strings.Split(a.JoinAddrs, ",") {
		if addr == "" {
			continue
		}

		h, p, err := net.SplitHostPort(addr)
		if err != nil {
			log.Fatalf("fail to parse JoinAddrs %s error %v\n", a.JoinAddrs, err)
		}

		adr := fmt.Sprintf("%s:%s", EmptyThen(h, a.hostIP), p)
		a.JoinAddrSlice = append(a.JoinAddrSlice, adr)
	}

	if len(a.JoinAddrSlice) == 0 || a.JoinAddrSlice[0] == a.HTTPAddr {
		a.Bootstrap = true

		return
	}

	jHost, jPort, _ := net.SplitHostPort(a.JoinAddrSlice[0])
	_, hPort, _ := net.SplitHostPort(a.HTTPAddr)

	if jPort != hPort {
		return
	}

	a.Bootstrap = a.isLocalHost(jHost)
}

// Join joins the current not to raft cluster
func (a *Arg) Join() error {
	if a.Bootstrap {
		return nil
	}

	addrLen := len(a.JoinAddrSlice)
	if addrLen == 0 {
		return nil
	}

	for i := 0; i < 10; i++ {
		if i > 0 {
			log.Printf("retry after 1s\n")
			time.Sleep(1 * time.Second) // nolint gomnd
		}

		joinAddr := a.JoinAddrSlice[i%addrLen]

		if err := Join(joinAddr, a.RaftAddr, a.NodeID); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to join %s", a.JoinAddrs)
}

// Intercept intercepts the raft log applying.
func (a *Arg) Intercept(l *raft.Log, c Command) (interface{}, bool) {
	if a.ApplyInterceptor != nil {
		intercepted := a.ApplyInterceptor(l, c)
		if intercepted {
			return nil, true
		}
	}

	ret, err := a.Invoke(c.Key, []byte(c.Value))
	if errors.Is(err, ErrDealerNoExists) {
		return nil, false
	}

	if err != nil {
		return err, true
	}

	return ret, true
}

// Join joins current node (raftAddr and nodeID) to joinAddr.
func Join(joinAddr, raftAddr string, nodeID NodeID) error {
	joinURL := BindAddr(joinAddr).URLRaftJoin()
	log.Printf("joinURL %s\n", joinURL)

	r := &Rsp{}
	stateCode, resp, err := PostJSON(joinURL, JoinRequest{Addr: raftAddr, NodeID: nodeID}, r)
	log.Printf("join response %d %s\n", stateCode, resp)

	if err != nil {
		log.Printf("joined error %s\n", err.Error())

		return err
	}

	if r.OK {
		return nil
	}

	return errors.New(r.Msg)
}

// Cluster retrieves the RaftCluster
func Cluster(nodeID NodeID) (v RaftCluster, err error) {
	clusterURL := nodeID.URLRaftCluster()
	log.Printf("GET Cluster %s\n", clusterURL)

	rsp, err := GetJSON(clusterURL, &v)

	log.Printf("GET Cluster %s, result %+v\n", clusterURL, rsp)

	return v, err
}
