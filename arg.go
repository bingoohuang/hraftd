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

	"github.com/creasty/defaults"
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

	JoinAddrSlice []string

	hostIP string

	ApplyInterceptor ApplyInterceptor `json:"-"`
	LogDealer        `json:"-"`
	Logger           `json:"-"`
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

// FlagOptionFn defines FlagOption option func prototype.
type FlagOptionFn func(flagNames *FlagNames)

// FlagNames defines struct for flag names.
type FlagNames struct {
	Rmem  string `default:"rmem"`
	Haddr string `default:"haddr"`
	Hadv  string `default:"hadv"`
	Raddr string `default:"raddr"`
	Radv  string `default:"radv"`
	Rdir  string `default:"rdir"`
	Rjoin string `default:"rjoin"`
	Iface string `default:"iface"`
}

// FlagRmem defines InMem flag name. If empty, disable the flag.
func FlagRmem(name string) FlagOptionFn { return func(f *FlagNames) { f.Rmem = name } }

// FlagHaddr defines HTTPAddr flag name. If empty, disable the flag.
func FlagHaddr(name string) FlagOptionFn { return func(f *FlagNames) { f.Haddr = name } }

// FlagHadv defines HTTPAdv flag name. If empty, disable the flag.
func FlagHadv(name string) FlagOptionFn { return func(f *FlagNames) { f.Hadv = name } }

// FlagRaddr defines RaftAddr flag name. If empty, disable the flag.
func FlagRaddr(name string) FlagOptionFn { return func(f *FlagNames) { f.Raddr = name } }

// FlagRadv defines RaftAdv flag name. If empty, disable the flag.
func FlagRadv(name string) FlagOptionFn { return func(f *FlagNames) { f.Radv = name } }

// FlagRdir defines RaftNodeDir flag name. If empty, disable the flag.
func FlagRdir(name string) FlagOptionFn { return func(f *FlagNames) { f.Rdir = name } }

// FlagRjoin defines JoinAddrs flag name. If empty, disable the flag.
func FlagRjoin(name string) FlagOptionFn { return func(f *FlagNames) { f.Rjoin = name } }

// FlagIface defines IfaceName flag name. If empty, disable the flag.
func FlagIface(name string) FlagOptionFn { return func(f *FlagNames) { f.Iface = name } }

// DefineFlags define raft args
func DefineFlags(p FlagProvider, flagOptionFns ...FlagOptionFn) *Arg {
	f := createFlagNames(flagOptionFns)
	a := MakeArg()

	boolVar(p, &a.InMem, true, f.Rmem, "Use in-memory storage for Raft")
	strVar(p, &a.HTTPAddr, f.Haddr, "HTTP server bind address")
	strVar(p, &a.HTTPAdv, f.Hadv, "Advertised HTTP address. If not set, same as HTTP server")
	strVar(p, &a.RaftAddr, f.Raddr, "Raft communication bind address. If not set, same as haddr(port+1000)")
	strVar(p, &a.RaftAdv, f.Radv, "Advertised Raft communication address. If not set, same as Raft bind")
	strVar(p, &a.RaftNodeDir, f.Rdir, "Raft data directory, default to ~/.hraftd/{id}")
	strVar(p, &a.JoinAddrs, f.Rjoin, "Set raft cluster join addresses separated by comma, if any")
	strVar(p, &a.IfaceName, f.Iface, "iface name to bind")

	return a
}

func boolVar(p FlagProvider, ptr *bool, defaultValue bool, name, usage string) {
	if name != "" {
		p.BoolVar(ptr, name, defaultValue, usage)
	}
}

func strVar(p FlagProvider, ptr *string, name, usage string) {
	if name != "" {
		p.StringVar(ptr, name, "", usage)
	}
}

// CreateArg creates Arg by ViperProvider implementation.
func CreateArg(p ViperProvider, flagOptionFns ...FlagOptionFn) *Arg {
	f := createFlagNames(flagOptionFns)
	a := MakeArg()

	p.SetDefault(f.Rmem, true)

	a.InMem = p.GetBool(f.Rmem)
	a.HTTPAddr = p.GetString(f.Haddr)
	a.HTTPAdv = p.GetString(f.Hadv)
	a.RaftAddr = p.GetString(f.Raddr)
	a.RaftAdv = p.GetString(f.Radv)
	a.RaftNodeDir = p.GetString(f.Rdir)
	a.JoinAddrs = p.GetString(f.Rjoin)
	a.IfaceName = p.GetString(f.Iface)

	return a
}

func createFlagNames(flagOptionFns []FlagOptionFn) *FlagNames {
	f := &FlagNames{}

	if err := defaults.Set(f); err != nil {
		log.Printf("failed to set defaults %v", err)
	}

	for _, fn := range flagOptionFns {
		fn(f)
	}

	return f
}

// ViperProvider defines the args getter provider.
type ViperProvider interface {
	SetDefault(key string, value interface{})
	GetBool(key string) bool
	GetString(key string) string
}

// Fix fixes the arg for some defaults.
func (a *Arg) Fix() {
	if a.Logger == nil {
		a.Logger = DefaultLogger
	}

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
			a.Panic("port %d is too large (<= 34565)", por)
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
			a.Panic("port %d is too large (>= 2024)", por)
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
			a.Panic("fail to parse JoinAddrs %s error %v", a.JoinAddrs, err)
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
			a.Printf("retry after 1s")
			time.Sleep(1 * time.Second) // nolint gomnd
		}

		joinAddr := a.JoinAddrSlice[i%addrLen]

		if err := Join(a, joinAddr, a.RaftAddr, a.NodeID); err == nil {
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
	if errors.Is(err, ErrDealerNoExists) || errors.Is(err, ErrDealerContinue) {
		return nil, false
	}

	if err != nil {
		return err, true
	}

	return ret, true
}

// Join joins current node (raftAddr and nodeID) to joinAddr.
func Join(logger Logger, joinAddr, raftAddr string, nodeID NodeID) error {
	joinURL := BindAddr(joinAddr).URLRaftJoin()
	logger.Printf("joinURL %s", joinURL)

	r := &Rsp{}
	stateCode, resp, err := PostJSON(joinURL, JoinRequest{Addr: raftAddr, NodeID: nodeID}, r)
	logger.Printf("join response %d %s", stateCode, resp)

	if err != nil {
		logger.Printf("joined error %s", err.Error())

		return err
	}

	if r.OK {
		return nil
	}

	return errors.New(r.Msg)
}

// Cluster retrieves the RaftCluster
func Cluster(logger Logger, nodeID NodeID) (v RaftCluster, err error) {
	clusterURL := nodeID.URLRaftCluster()
	logger.Printf("GET Cluster %s", clusterURL)

	rsp, err := GetJSON(clusterURL, &v)

	logger.Printf("GET Cluster %s, result %+v", clusterURL, rsp)

	return v, err
}
