//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package collectors

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/dmachard/go-dnscollector/dnsutils"
	"github.com/dmachard/go-dnscollector/processors"
	"github.com/dmachard/go-dnscollector/xdp"
	"github.com/dmachard/go-logger"
	"golang.org/x/sys/unix"
)

func GetIpAddress[T uint32 | [4]uint32](ip T, mapper func(T) net.IP) net.IP {
	return mapper(ip)
}

func ConvertIp4(ip uint32) net.IP {
	addr := make(net.IP, net.IPv4len)
	binary.BigEndian.PutUint32(addr, ip)
	return addr
}

func ConvertIp6(ip [4]uint32) net.IP {
	addr := make(net.IP, net.IPv6len)
	binary.LittleEndian.PutUint32(addr[0:], ip[0])
	binary.LittleEndian.PutUint32(addr[4:], ip[1])
	binary.LittleEndian.PutUint32(addr[8:], ip[2])
	binary.LittleEndian.PutUint32(addr[12:], ip[3])
	return addr
}

type XdpSniffer struct {
	done       chan bool
	exit       chan bool
	identity   string
	loggers    []dnsutils.Worker
	config     *dnsutils.Config
	configChan chan *dnsutils.Config
	logger     *logger.Logger
	name       string
}

func NewXdpSniffer(loggers []dnsutils.Worker, config *dnsutils.Config, logger *logger.Logger, name string) *XdpSniffer {
	logger.Info("[%s] collector=xdp - enabled", name)
	s := &XdpSniffer{
		done:       make(chan bool),
		exit:       make(chan bool),
		config:     config,
		configChan: make(chan *dnsutils.Config),
		loggers:    loggers,
		logger:     logger,
		name:       name,
	}
	s.ReadConfig()
	return s
}

func (c *XdpSniffer) LogInfo(msg string, v ...interface{}) {
	c.logger.Info("["+c.name+"] collector=xdp - "+msg, v...)
}

func (c *XdpSniffer) LogError(msg string, v ...interface{}) {
	c.logger.Error("["+c.name+"] collector=xdp - "+msg, v...)
}

func (c *XdpSniffer) GetName() string { return c.name }

func (c *XdpSniffer) SetLoggers(loggers []dnsutils.Worker) {
	c.loggers = loggers
}

func (c *XdpSniffer) Loggers() ([]chan dnsutils.DnsMessage, []string) {
	channels := []chan dnsutils.DnsMessage{}
	names := []string{}
	for _, p := range c.loggers {
		channels = append(channels, p.Channel())
		names = append(names, p.GetName())
	}
	return channels, names
}

func (c *XdpSniffer) ReadConfig() {
	c.identity = c.config.GetServerIdentity()
}

func (c *XdpSniffer) ReloadConfig(config *dnsutils.Config) {
	c.LogInfo("reload configuration...")
	c.configChan <- config
}

func (c *XdpSniffer) Channel() chan dnsutils.DnsMessage {
	return nil
}

func (c *XdpSniffer) Stop() {
	c.LogInfo("stopping...")

	// exit to close properly
	c.exit <- true

	// read done channel and block until run is terminated
	<-c.done
	close(c.done)
}

func (c *XdpSniffer) Run() {
	c.LogInfo("starting collector...")

	dnsProcessor := processors.NewDnsProcessor(c.config, c.logger, c.name, c.config.Collectors.XdpLiveCapture.ChannelBufferSize)
	go dnsProcessor.Run(c.Loggers())

	iface, err := net.InterfaceByName(c.config.Collectors.XdpLiveCapture.Device)
	if err != nil {
		c.LogError("lookup network iface: %s", err)
		os.Exit(1)
	}

	// Load pre-compiled programs into the kernel.
	objs := xdp.BpfObjects{}
	if err := xdp.LoadBpfObjects(&objs, nil); err != nil {
		c.LogError("loading BPF objects: %s", err)
		os.Exit(1)
	}
	defer objs.Close()

	// Attach the program.
	l, err := link.AttachXDP(link.XDPOptions{
		Program:   objs.XdpSniffer,
		Interface: iface.Index,
	})
	if err != nil {
		c.LogError("could not attach XDP program: %s", err)
		os.Exit(1)
	}
	defer l.Close()

	c.LogInfo("XDP program attached to iface %q (index %d)", iface.Name, iface.Index)

	perfEvent, err := perf.NewReader(objs.Pkts, 1<<24)
	if err != nil {
		panic(err)
	}

	dnsChan := make(chan dnsutils.DnsMessage)

	// goroutine to read all packets reassembled
	go func() {
		for {
			select {
			// new config provided?
			case cfg, opened := <-c.configChan:
				if !opened {
					return
				}
				c.config = cfg
				c.ReadConfig()

				// send the config to the dns processor
				dnsProcessor.ConfigChan <- cfg

			//dns message to read ?
			case dm := <-dnsChan:

				// update identity with config ?
				dm.DnsTap.Identity = c.identity

				dnsProcessor.GetChannel() <- dm

			}
		}
	}()

	go func() {
		var pkt xdp.BpfPktEvent
		for {
			// The data submitted via bpf_perf_event_output.
			record, err := perfEvent.Read()
			if err != nil {
				c.LogError("BPF reading map: %s", err)
				break
			}

			if record.LostSamples != 0 {
				c.LogError("BPF dump: Dropped %d samples from kernel perf buffer", record.LostSamples)
				continue
			}

			reader := bytes.NewReader(record.RawSample)
			if err := binary.Read(reader, binary.LittleEndian, &pkt); err != nil {
				c.LogError("BPF reading sample: %s", err)
				break
			}

			// adjust arrival time
			timenow := time.Now().UTC()
			var ts unix.Timespec
			unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
			elapsed := time.Since(timenow) * time.Nanosecond
			delta3 := time.Duration(uint64(unix.TimespecToNsec(ts))-pkt.Timestamp) * time.Nanosecond
			tsAdjusted := timenow.Add(-(delta3 + elapsed))

			// convert ip
			var saddr, daddr net.IP
			if pkt.IpVersion == 0x0800 {
				saddr = GetIpAddress(pkt.SrcAddr, ConvertIp4)
				daddr = GetIpAddress(pkt.DstAddr, ConvertIp4)
			} else {
				saddr = GetIpAddress(pkt.SrcAddr6, ConvertIp6)
				daddr = GetIpAddress(pkt.DstAddr6, ConvertIp6)
			}

			// prepare DnsMessage
			dm := dnsutils.DnsMessage{}
			dm.Init()

			dm.DnsTap.TimeSec = int(tsAdjusted.Unix())
			dm.DnsTap.TimeNsec = int(tsAdjusted.UnixNano() - tsAdjusted.Unix()*1e9)

			if pkt.SrcPort == 53 {
				dm.DnsTap.Operation = dnsutils.DNSTAP_CLIENT_RESPONSE
			} else {
				dm.DnsTap.Operation = dnsutils.DNSTAP_CLIENT_QUERY
			}

			dm.NetworkInfo.QueryIp = saddr.String()
			dm.NetworkInfo.QueryPort = fmt.Sprint(pkt.SrcPort)
			dm.NetworkInfo.ResponseIp = daddr.String()
			dm.NetworkInfo.ResponsePort = fmt.Sprint(pkt.DstPort)

			if pkt.IpVersion == 0x0800 {
				dm.NetworkInfo.Family = dnsutils.PROTO_IPV4
			} else {
				dm.NetworkInfo.Family = dnsutils.PROTO_IPV6
			}

			if pkt.IpProto == 0x11 {
				dm.NetworkInfo.Protocol = dnsutils.PROTO_UDP
				dm.DNS.Payload = record.RawSample[int(pkt.PktOffset)+int(pkt.PayloadOffset):]
				dm.DNS.Length = len(dm.DNS.Payload)
			} else {
				dm.NetworkInfo.Protocol = dnsutils.PROTO_TCP
				dm.DNS.Payload = record.RawSample[int(pkt.PktOffset)+int(pkt.PayloadOffset)+2:]
				dm.DNS.Length = len(dm.DNS.Payload)
			}

			dnsChan <- dm
		}
	}()

	<-c.exit
	close(dnsChan)
	close(c.configChan)

	// stop dns processor
	dnsProcessor.Stop()

	c.LogInfo("run terminated")
	c.done <- true
}
