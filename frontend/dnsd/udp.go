package dnsd

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/global"
	"math/rand"
	"net"
	"strings"
	"time"
)

// Send forward queries to forwarder and forward the response to my DNS client.
func (dnsd *DNSD) HandleUDPQueries(myQueue chan *UDPQuery, forwarderConn net.Conn) {
	packetBuf := make([]byte, MaxPacketSize)
	for {
		query := <-myQueue
		// Set deadline for IO with forwarder
		forwarderConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := forwarderConn.Write(query.QueryPacket); err != nil {
			dnsd.Logger.Warningf("HandleUDPQueries", query.ClientAddr.String(), err, "failed to write to forwarder")
			continue
		}
		packetLength, err := forwarderConn.Read(packetBuf)
		if err != nil {
			dnsd.Logger.Warningf("HandleUDPQueries", query.ClientAddr.String(), err, "failed to read from forwarder")
			continue
		}
		// Set deadline for responding to my DNS client
		query.MyServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := query.MyServer.WriteTo(packetBuf[:packetLength], query.ClientAddr); err != nil {
			dnsd.Logger.Warningf("HandleUDPQueries", query.ClientAddr.String(), err, "failed to answer to client")
			continue
		}
	}
}

// Send blackhole answer to my DNS client.
func (dnsd *DNSD) HandleBlackHoleAnswer(myQueue chan *UDPQuery) {
	for {
		query := <-myQueue
		// Set deadline for responding to my DNS client
		blackHoleAnswer := RespondWith0(query.QueryPacket)
		query.MyServer.SetWriteDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		if _, err := query.MyServer.WriteTo(blackHoleAnswer, query.ClientAddr); err != nil {
			dnsd.Logger.Warningf("HandleUDPQueries", query.ClientAddr.String(), err, "IO failure")
		}
	}
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon to listen on UDP port only. Block caller.
*/
func (dnsd *DNSD) StartAndBlockUDP() error {
	listenAddr := fmt.Sprintf("%s:%d", dnsd.UDPListenAddress, dnsd.UDPListenPort)
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	udpServer, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	// Start queues that will respond to DNS clients
	for i, queue := range dnsd.UDPForwarderQueues {
		go dnsd.HandleUDPQueries(queue, dnsd.UDPForwarderConns[i])
	}
	for _, queue := range dnsd.UDPBlackHoleQueues {
		go dnsd.HandleBlackHoleAnswer(queue)
	}
	// Dispatch queries to forwarder queues
	packetBuf := make([]byte, MaxPacketSize)
	dnsd.Logger.Printf("StartAndBlockUDP", listenAddr, nil, "going to listen for queries")
	for {
		if global.EmergencyLockDown {
			return global.ErrEmergencyLockDown
		}
		packetLength, clientAddr, err := udpServer.ReadFromUDP(packetBuf)
		if err != nil {
			return err
		}
		// Check address against rate limit
		clientIP := clientAddr.IP.String()
		if !dnsd.RateLimit.Add(clientIP, true) {
			continue
		}
		// Check address against allowed IP prefixes
		var prefixOK bool
		for _, prefix := range dnsd.AllowQueryIPPrefixes {
			if strings.HasPrefix(clientIP, prefix) {
				prefixOK = true
				break
			}
		}
		if !prefixOK {
			dnsd.Logger.Warningf("UDPLoop", clientIP, nil, "client IP is not allowed to query")
			continue
		}

		// Prepare parameters for forwarding the query
		randForwarder := rand.Intn(len(dnsd.UDPForwarderQueues))
		forwardPacket := make([]byte, packetLength)
		copy(forwardPacket, packetBuf[:packetLength])
		domainName := ExtractDomainName(forwardPacket)
		if len(domainName) == 0 {
			// If I cannot figure out what domain is from the query, simply forward it without much concern.
			dnsd.Logger.Printf(fmt.Sprintf("UDP-%d", randForwarder), clientIP, nil,
				"handle non-name query (backlog %d)", len(dnsd.UDPForwarderQueues[randForwarder]))
			dnsd.UDPForwarderQueues[randForwarder] <- &UDPQuery{
				ClientAddr:  clientAddr,
				MyServer:    udpServer,
				QueryPacket: forwardPacket,
			}
		} else if dnsd.NamesAreBlackListed(domainName) {
			// Requested domain name is black-listed
			randBlackListResponder := rand.Intn(len(dnsd.UDPBlackHoleQueues))
			dnsd.Logger.Printf(fmt.Sprintf("UDP-%d", randBlackListResponder), clientIP, nil,
				"handle black-listed domain \"%s\" (backlog %d)", domainName[0], len(dnsd.UDPBlackHoleQueues[randBlackListResponder]))
			dnsd.UDPBlackHoleQueues[randBlackListResponder] <- &UDPQuery{
				ClientAddr:  clientAddr,
				MyServer:    udpServer,
				QueryPacket: forwardPacket,
			}
		} else {
			// This is a normal domain name query and not black-listed
			dnsd.Logger.Printf(fmt.Sprintf("UDP-%d", randForwarder), clientIP, nil,
				"handle domain \"%s\" (backlog %d)", domainName[0], len(dnsd.UDPForwarderQueues[randForwarder]))
			dnsd.UDPForwarderQueues[randForwarder] <- &UDPQuery{
				ClientAddr:  clientAddr,
				MyServer:    udpServer,
				QueryPacket: forwardPacket,
			}
		}
	}
}
