package natneg

import (
	"encoding/binary"
	"net"
	"owfc/logging"
	"time"
)

func (s *Session) sendConnectRequests(moduleName string) {
	if s.Host == nil || !s.Host.isMapped() {
		return
	}
	if s.Guest == nil || !s.Guest.isMapped() {
		return
	}

	s.mutex.Lock()
	if s.Connecting {
		s.mutex.Unlock()
		return
	}
	s.Connecting = true
	s.mutex.Unlock()

	s.Host.ConnectAck = false
	s.Guest.ConnectAck = false

	logging.Notice(moduleName, "Exchange connect requests")

	go func() {
		for range time.NewTicker(time.Millisecond * 500).C {
			if !s.Open || s.Guest.Result != -1 || s.Host.Result != -1 {
				break
			}

			var retry bool
			if !s.Host.ConnectAck {
				retry = true
				s.Guest.sendConnectRequestPacket(natnegConn, s.Host, s.Version)
			}
			if !s.Guest.ConnectAck {
				retry = true
				s.Host.sendConnectRequestPacket(natnegConn, s.Guest, s.Version)
			}
			if !retry {
				break
			}
		}

		s.Connecting = false
	}()
}

func (client *Peer) sendConnectRequestPacket(conn net.PacketConn, destination *Peer, version byte) {
	connectHeader := createPacketHeader(version, NNConnectRequest, destination.Cookie)
	connectHeader = append(connectHeader, client.GameAddr.IP...)
	connectHeader = binary.BigEndian.AppendUint16(connectHeader, uint16(client.GameAddr.Port))
	// Two bytes: "gotyourdata" and "finished"
	connectHeader = append(connectHeader, 0x42, 0x00)

	conn.WriteTo(connectHeader, destination.NegotiateAddr)
}

func (s *Session) handleConnectReply(moduleName string, conn net.PacketConn, addr *net.UDPAddr, buffer []byte) {
	// portType := buffer[0]
	clientIndex := buffer[1]
	// useGamePort := buffer[2]
	// localIP, ok := netip.AddrFromSlice(buffer[3:7])

	client := s.Guest
	if clientIndex == 1 {
		client = s.Host
	}
	if client == nil {
		return
	}

	client.ConnectAck = true
}
