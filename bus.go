package main

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/pkg/errors"
	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/protocol/bus"
	_ "nanomsg.org/go/mangos/v2/transport/tcp"
)

type MessageBus struct {
	bus mangos.Socket

	mu           sync.RWMutex
	addrToPeerID map[string]string
}

func NewMessageBus() *MessageBus {
	return &MessageBus{
		addrToPeerID: make(map[string]string),
	}
}

type MessageHandler interface {
	HandleMessage(fromID string, msg []byte)
}

type ErrorHandler interface {
	HandleError(err error)
}

func (mb *MessageBus) Start(ctx context.Context, port int, mh MessageHandler, eh ErrorHandler) error {
	if mh == nil {
		return errors.New("message handler cannot be nil")
	}

	sock, err := bus.NewSocket()
	if err != nil {
		return errors.Wrap(err, "error creating bus")
	}
	mb.bus = sock

	err = mb.bus.Listen(fmt.Sprintf("tcp://0.0.0.0:%d", port))
	if err != nil {
		return errors.Wrap(err, "error listening on bus")
	}

	for {
		msg, err := mb.bus.RecvMsg()
		if err != nil {
			if eh != nil {
				eh.HandleError(err)
			}
			continue
		}

		id := mb.PeerIDForAddress(msg.Pipe.Address())
		mh.HandleMessage(id, msg.Body)
	}
}

func (mb *MessageBus) Send(msg []byte) error {
	err := mb.bus.Send(msg)
	return errors.Wrap(err, "error sending message")
}

func (mb *MessageBus) AddPeer(id string, ip net.IP, port int) error {
	addr := fmt.Sprintf("tcp://%s:%d", ip, port)
	err := mb.bus.Dial(addr)
	if err != nil {
		return errors.Wrap(err, "error adding peer to message bus")
	}

	mb.RegisterPeerIDAndAddress(id, addr)
	return nil
}

func (mb *MessageBus) PeerIDForAddress(addr string) string {
	mb.mu.RLock()
	id := mb.addrToPeerID[addr]
	mb.mu.RUnlock()
	return id
}

func (mb *MessageBus) RegisterPeerIDAndAddress(id, addr string) {
	mb.mu.Lock()
	mb.addrToPeerID[addr] = id
	mb.mu.Unlock()
}
