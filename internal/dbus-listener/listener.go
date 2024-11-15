package dbuslistener

import (
	"log"

	"github.com/godbus/dbus/v5"
)

type Listener struct {
	c  *dbus.Conn
	ch chan struct{}
}

func Connect() (*Listener, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchMember("PrepareForSleep"),
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchSender("org.freedesktop.login1"),
	); err != nil {
		conn.Close()
		return nil, err
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchMember("StateChanged"),
		dbus.WithMatchInterface("org.freedesktop.NetworkManager"),
		dbus.WithMatchSender("org.freedesktop.NetworkManager"),
	); err != nil {
		conn.Close()
		return nil, err
	}

	return &Listener{
		c:  conn,
		ch: make(chan struct{}),
	}, nil

}

func (l *Listener) Close() error {
	close(l.ch)
	return l.c.Close()
}

func WokeFromSleepWithNetwork(l *Listener) <-chan struct{} {

	if l == nil {
		return make(<-chan struct{})
	}

	go func() {
		sig := make(chan *dbus.Signal)
		l.c.Signal(sig)

		// The sequence must be
		// 1. Woke From Sleep
		// 2. Network connectivity goes through states 20-40-60
		// 3. Got connectivity (State 70)
		wokeFromSleep := false
		for s := range sig {
			if s.Name == "org.freedesktop.login1.Manager.PrepareForSleep" {
				if len(s.Body) < 1 {
					log.Printf("Empty body for signal %s", s.Name)
					continue
				}

				goingToSleep, ok := s.Body[0].(bool)
				if !ok {
					log.Printf("Body did not contain a bool: '%s'", s.Body)
					continue
				}

				// explicit rather than a = !b to avoid (my own) confusion
				if !goingToSleep {
					wokeFromSleep = true
				}
			} else if s.Name == "org.freedesktop.NetworkManager.StateChanged" && wokeFromSleep {
				// Means outside internet is accessible
				// not just local network
				// https://www.networkmanager.dev/docs/api/latest/nm-dbus-types.html#NMState
				const NM_STATE_CONNECTED_GLOBAL = 70

				if len(s.Body) < 1 {
					continue
				}

				val, ok := s.Body[0].(uint32)
				if !ok {
					continue
				}

				if val == NM_STATE_CONNECTED_GLOBAL {
					l.ch <- struct{}{}
				}
			}
		}
	}()

	return l.ch
}
