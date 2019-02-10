package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/grandcat/zeroconf"
	"github.com/pkg/errors"
)

type PeerDiscovery struct {
	ID          string
	ServiceType string
	OnPeerFound PeerFoundEventHook

	advertiser *zeroconf.Server
}

type PeerFoundEventHook func(id string, ip net.IP, port int) error

func NewPeerDiscovery(myID, serviceType string, onJoin PeerFoundEventHook) *PeerDiscovery {
	return &PeerDiscovery{
		ID:          myID,
		ServiceType: serviceType,
		OnPeerFound: onJoin,
	}
}

func (pd *PeerDiscovery) StartAdvertising(port int) error {
	if pd.advertiser != nil {
		return nil
	}

	zs, err := zeroconf.Register(pd.ID, fmt.Sprintf("_%s._tcp", pd.ServiceType), "local.", port, []string{"txtv=0", fmt.Sprintf("id=%s", pd.ID)}, nil)
	if err != nil {
		return errors.Wrap(err, "peer discovery start advertising error")
	}
	pd.advertiser = zs
	return nil
}

func (pd *PeerDiscovery) StopAdvertising() {
	if pd.advertiser == nil {
		return
	}

	pd.advertiser.Shutdown()
}

func (pd *PeerDiscovery) Watch(ctx context.Context) error {
	var mu sync.Mutex
	peers := make(map[string]bool)
	peers[pd.ID] = true

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for {
			select {
			case <-ctx.Done():
				return
			case entry := <-results:
				eID := idFromText(entry.Text)
				mu.Lock()
				if peers[eID] {
					mu.Unlock()
					continue
				}

				peers[eID] = true

				var addr net.IP
				if len(entry.AddrIPv4) > 0 && !entry.AddrIPv4[0].IsUnspecified() {
					addr = entry.AddrIPv4[0]
				} else if len(entry.AddrIPv6) > 0 && !entry.AddrIPv6[0].IsUnspecified() {
					addr = entry.AddrIPv6[0]
				}

				if !addr.IsUnspecified() {
					pd.OnPeerFound(eID, addr, entry.Port)
				}
				mu.Unlock()
			}
		}
	}(entries)

	zr, err := zeroconf.NewResolver(nil)
	if err != nil {
		return errors.Wrap(err, "error creating zeroconf resolver")
	}
	err = zr.Browse(ctx, fmt.Sprintf("_%s._tcp", pd.ServiceType), "local.", entries)
	return errors.Wrap(err, "error watching zeroconf")
}

func idFromText(txt []string) string {
	for _, t := range txt {
		if strings.HasPrefix(t, "id=") {
			return t[3:]
		}
	}
	return ""
}
