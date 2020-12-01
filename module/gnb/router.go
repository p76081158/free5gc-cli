package gnb

import (
	"fmt"
	"free5gc-cli/logger"
	"net"
	"os"
	"os/exec"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/songgao/water"
)

// BUFFERSIZE of the packet
const BUFFERSIZE = 1500

// MTU is 1500 - IPV4 - UDP - GTP
const MTU = "1400"

func runIP(args ...string) {
	cmd := exec.Command("ip", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if nil != err {
		logger.GNBLog.Errorln("Error running ip command:", err)
	}
	return
}

var gtpRouter *GTPRouter

// GTPRouter provides the functionnality to encapsulate and desencapsulate packet using GTP protocol
type GTPRouter struct {
	GNB     *GNB
	UpfConn *net.UDPConn
	Iface   *water.Interface
}

// NewRouter build a new router
func NewRouter(upfIP string, upfPort int, gnbIP string, gnbPort int, gnb *GNB) (*GTPRouter, error) {

	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = GNBConfig.Configuration.TUN

	iface, err := water.New(config)
	if nil != err {
		logger.GNBLog.Errorln("Unable to allocate TUN interface:", err)
	}

	logger.GNBLog.Infoln("TUN Interface allocated:", iface.Name())
	// set interface parameters
	runIP("link", "set", "dev", iface.Name(), "mtu", MTU)
	runIP("addr", "add", GNBConfig.Configuration.GTPInterface.Ipv4, "dev", iface.Name())
	runIP("link", "set", "dev", iface.Name(), "up")

	var GNBAddr = net.UDPAddr{IP: net.ParseIP(gnbIP), Port: gnbPort}
	// var UPFAddr = net.UDPAddr{IP: net.ParseIP(upfIP), Port: upfPort}

	// Connect to the UPF
	fmt.Println(upfIP)
	upfAddress, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", upfIP, upfPort))
	if err != nil {
		logger.GNBLog.Errorln("Impossible to resolve UPF address")
		logger.GNBLog.Errorln(err)
		return nil, err
	}
	upfConn, err := net.DialUDP("udp", &GNBAddr, upfAddress)
	if err != nil {
		logger.GNBLog.Errorln("Impossible to Dial UPF")
		return nil, err
	}

	var gtpRouter = GTPRouter{GNB: gnb, UpfConn: upfConn, Iface: iface}
	return &gtpRouter, nil

}

// Close the connection with the UPF and Tun interface
func (r *GTPRouter) Close() {
	r.UpfConn.Close()
	r.Iface.Close()
}

// Encapsulate the packet using GTP protocol
func (r *GTPRouter) Encapsulate() {

	// Read the incoming packet on the tun interface
	// Encapsulate the packet with GTP
	// Write it to the socket with the UPF
	packet := make([]byte, BUFFERSIZE)
	var ipv4 layers.IPv4
	var gtp layers.GTPv1U
	var icmpv4 layers.ICMPv4
	var payload gopacket.Payload
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{} // See SerializeOptions for more details.
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &ipv4, &payload, &icmpv4)
	decoded := []gopacket.LayerType{}
	for {
		// read the packet coming from the TUN interface
		n, err := r.Iface.Read(packet)
		fmt.Println(fmt.Sprintf("Reading %d bytes", n))
		if err != nil {
			logger.GNBLog.Errorln("Error reading the TUN interface input")
			break
		}
		// build the ipv4 header
		err = parser.DecodeLayers(packet[:n], &decoded)
		if err == nil {
			// find the teid
			logger.GNBLog.Infoln("UPF Packet")
			teid, err := r.GNB.GetTEID(ipv4.SrcIP)
			if err == nil {
				gtp = layers.GTPv1U{
					TEID:          teid,
					MessageType:   0xFF,
					MessageLength: uint16(n),
				}
				err = gtp.SerializeTo(buf, opts)
				if err != nil {
					logger.GNBLog.Errorln("Error Serializing the packet Layers")
					break
				}
				pkt := append(buf.Bytes(), packet[:n]...)
				n, err = r.UpfConn.Write(pkt)
			}
		} else {
			logger.GNBLog.Infoln(err)
			logger.GNBLog.Infoln(len(decoded))
			logger.GNBLog.Infoln("Not a 5G UPF Packet")
		}

	}

}

// Desencapsulate the GTP packet: remove the GTP headers and route the packet
func (r *GTPRouter) Desencapsulate() {

	// Read the packet coming from the socket
	// Desencapsulate the packet and remove GTP Header
	// Write the answer to the TUN interface

	buf := make([]byte, BUFFERSIZE)
	var gtp layers.GTPv1U
	var payload gopacket.Payload
	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeGTPv1U, &gtp, &payload)
	decoded := []gopacket.LayerType{}

	for {
		n, _, err := r.UpfConn.ReadFromUDP(buf)
		if err != nil {
			break
		}

		err = parser.DecodeLayers(buf[:n], &decoded)
		if err != nil {
			break
		}

		r.Iface.Write(payload)
	}

}
