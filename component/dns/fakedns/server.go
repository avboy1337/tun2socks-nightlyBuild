package fakedns

import (
	"errors"
	"fmt"
	"net"
	"strings"

	trie "github.com/xjasonlyu/tun2socks/common/domain-trie"
	"github.com/xjasonlyu/tun2socks/common/fakeip"

	D "github.com/miekg/dns"
)

const (
	dnsFakeTTL    uint32 = 1
	dnsDefaultTTL uint32 = 600
)

var (
	ServeAddr = "127.0.0.1:5353"
)

type Server struct {
	*D.Server
	p *fakeip.Pool
	h handler
}

func (s *Server) ServeDNS(w D.ResponseWriter, r *D.Msg) {
	if len(r.Question) == 0 {
		D.HandleFailed(w, r)
		return
	}
	s.h(w, r)
}

func (s *Server) Start() error {
	_, port, err := net.SplitHostPort(ServeAddr)
	if port == "0" || port == "" || err != nil {
		return errors.New("address format error")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", ServeAddr)
	if err != nil {
		return err
	}

	pc, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	s.Server = &D.Server{Addr: ServeAddr, PacketConn: pc, Handler: s}
	go func() {
		s.ActivateAndServe()
	}()

	return nil
}

func (s *Server) Stop() error {
	return s.Shutdown()
}

func (s *Server) IPToHost(ip net.IP) (string, bool) {
	return s.p.LookBack(ip)
}

func NewServer(fakeIPRange, hosts string, size int) (*Server, error) {
	_, ipnet, err := net.ParseCIDR(fakeIPRange)
	if err != nil {
		return nil, err
	}
	pool, err := fakeip.New(ipnet, size)
	if err != nil {
		return nil, err
	}

	hostsTree := func(str string) *trie.Trie {
		// trim `'` `"` ` ` char
		str = strings.Trim(str, "' \"")
		if str == "" {
			return nil
		}
		tree := trie.New()
		s := strings.Split(str, ",")
		for _, host := range s {
			m := strings.Split(host, "=")
			if len(m) != 2 {
				continue
			}
			domain := strings.TrimSpace(m[0])
			target := strings.TrimSpace(m[1])
			if err := tree.Insert(domain, net.ParseIP(target)); err != nil {
				panic(fmt.Sprintf("add hosts error: %v", err))
			}
		}
		return tree
	}(hosts)

	handler := newHandler(hostsTree, pool)

	return &Server{
		p: pool,
		h: handler,
	}, nil
}