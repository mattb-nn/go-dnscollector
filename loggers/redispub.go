package loggers

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dmachard/go-dnscollector/dnsutils"
	"github.com/dmachard/go-dnscollector/transformers"
	"github.com/dmachard/go-logger"
)

type RedisPub struct {
	stopProcess        chan bool
	doneProcess        chan bool
	stopRun            chan bool
	doneRun            chan bool
	stopRead           chan bool
	doneRead           chan bool
	inputChan          chan dnsutils.DnsMessage
	outputChan         chan dnsutils.DnsMessage
	config             *dnsutils.Config
	configChan         chan *dnsutils.Config
	logger             *logger.Logger
	textFormat         []string
	name               string
	transport          string
	transportWriter    *bufio.Writer
	transportConn      net.Conn
	transportReady     chan bool
	transportReconnect chan bool
	writerReady        bool
}

func NewRedisPub(config *dnsutils.Config, logger *logger.Logger, name string) *RedisPub {
	logger.Info("[%s] logger=redispub - enabled", name)
	s := &RedisPub{
		stopProcess:        make(chan bool),
		doneProcess:        make(chan bool),
		stopRun:            make(chan bool),
		doneRun:            make(chan bool),
		stopRead:           make(chan bool),
		doneRead:           make(chan bool),
		inputChan:          make(chan dnsutils.DnsMessage, config.Loggers.RedisPub.ChannelBufferSize),
		outputChan:         make(chan dnsutils.DnsMessage, config.Loggers.RedisPub.ChannelBufferSize),
		transportReady:     make(chan bool),
		transportReconnect: make(chan bool),
		logger:             logger,
		config:             config,
		configChan:         make(chan *dnsutils.Config),
		name:               name,
	}

	s.ReadConfig()

	return s
}

func (c *RedisPub) GetName() string { return c.name }

func (c *RedisPub) SetLoggers(loggers []dnsutils.Worker) {}

func (o *RedisPub) ReadConfig() {

	o.transport = o.config.Loggers.RedisPub.Transport

	// begin backward compatibility
	if o.config.Loggers.RedisPub.TlsSupport {
		o.transport = dnsutils.SOCKET_TLS
	}
	if len(o.config.Loggers.RedisPub.SockPath) > 0 {
		o.transport = dnsutils.SOCKET_UNIX
	}
	// end

	if len(o.config.Loggers.RedisPub.TextFormat) > 0 {
		o.textFormat = strings.Fields(o.config.Loggers.RedisPub.TextFormat)
	} else {
		o.textFormat = strings.Fields(o.config.Global.TextFormat)
	}
}

func (o *RedisPub) ReloadConfig(config *dnsutils.Config) {
	o.LogInfo("reload configuration!")
	o.configChan <- config
}

func (o *RedisPub) LogInfo(msg string, v ...interface{}) {
	o.logger.Info("["+o.name+"] logger=redispub - "+msg, v...)
}

func (o *RedisPub) LogError(msg string, v ...interface{}) {
	o.logger.Error("["+o.name+"] logger=redispub - "+msg, v...)
}

func (o *RedisPub) Channel() chan dnsutils.DnsMessage {
	return o.inputChan
}

func (o *RedisPub) Stop() {
	o.LogInfo("stopping to run...")
	o.stopRun <- true
	<-o.doneRun

	o.LogInfo("stopping to receive...")
	o.stopRead <- true
	<-o.doneRead

	o.LogInfo("stopping to process...")
	o.stopProcess <- true
	<-o.doneProcess
}

func (o *RedisPub) Disconnect() {
	if o.transportConn != nil {
		o.LogInfo("closing redispub connection")
		o.transportConn.Close()
	}
}

func (o *RedisPub) ReadFromConnection() {
	buffer := make([]byte, 4096)

	go func() {
		for {
			_, err := o.transportConn.Read(buffer)
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
					o.LogInfo("read from connection terminated")
					break
				}
				o.LogError("Error on reading: %s", err.Error())
			}
			// We just discard the data
		}
	}()

	// block goroutine until receive true event in stopRead channel
	<-o.stopRead
	o.doneRead <- true

	o.LogInfo("read goroutine terminated")
}

func (o *RedisPub) ConnectToRemote() {
	for {
		if o.transportConn != nil {
			o.transportConn.Close()
			o.transportConn = nil
		}

		address := o.config.Loggers.RedisPub.RemoteAddress + ":" + strconv.Itoa(o.config.Loggers.RedisPub.RemotePort)
		connTimeout := time.Duration(o.config.Loggers.RedisPub.ConnectTimeout) * time.Second

		var conn net.Conn
		var err error

		switch o.transport {
		case dnsutils.SOCKET_UNIX:
			address = o.config.Loggers.RedisPub.RemoteAddress
			if len(o.config.Loggers.RedisPub.SockPath) > 0 {
				address = o.config.Loggers.RedisPub.SockPath
			}
			o.LogInfo("connecting to %s://%s", o.transport, address)
			conn, err = net.DialTimeout(o.transport, address, connTimeout)

		case dnsutils.SOCKET_TCP:
			o.LogInfo("connecting to %s://%s", o.transport, address)
			conn, err = net.DialTimeout(o.transport, address, connTimeout)

		case dnsutils.SOCKET_TLS:
			o.LogInfo("connecting to %s://%s", o.transport, address)

			var tlsConfig *tls.Config

			tlsOptions := dnsutils.TlsOptions{
				InsecureSkipVerify: o.config.Loggers.RedisPub.TlsInsecure,
				MinVersion:         o.config.Loggers.RedisPub.TlsMinVersion,
				CAFile:             o.config.Loggers.RedisPub.CAFile,
				CertFile:           o.config.Loggers.RedisPub.CertFile,
				KeyFile:            o.config.Loggers.RedisPub.KeyFile,
			}

			tlsConfig, err = dnsutils.TlsClientConfig(tlsOptions)
			if err == nil {
				dialer := &net.Dialer{Timeout: connTimeout}
				conn, err = tls.DialWithDialer(dialer, dnsutils.SOCKET_TCP, address, tlsConfig)
			}

		default:
			o.logger.Fatal("logger=redispub - invalid transport:", o.transport)
		}

		// something is wrong during connection ?
		if err != nil {
			o.LogError("%s", err)
			o.LogInfo("retry to connect in %d seconds", o.config.Loggers.RedisPub.RetryInterval)
			time.Sleep(time.Duration(o.config.Loggers.RedisPub.RetryInterval) * time.Second)
			continue
		}

		o.transportConn = conn

		// block until framestream is ready
		o.transportReady <- true

		// block until an error occured, need to reconnect
		o.transportReconnect <- true
	}
}

func (o *RedisPub) FlushBuffer(buf *[]dnsutils.DnsMessage) {
	// create escaping buffer
	escape_buffer := new(bytes.Buffer)
	// create a new encoder that writes to the buffer
	encoder := json.NewEncoder(escape_buffer)

	for _, dm := range *buf {
		escape_buffer.Reset()

		cmd := "PUBLISH " + strconv.Quote(o.config.Loggers.RedisPub.RedisChannel) + " "
		o.transportWriter.WriteString(cmd)

		if o.config.Loggers.RedisPub.Mode == dnsutils.MODE_TEXT {
			o.transportWriter.WriteString(strconv.Quote(dm.String(o.textFormat, o.config.Global.TextFormatDelimiter, o.config.Global.TextFormatBoundary)))
			o.transportWriter.WriteString(o.config.Loggers.RedisPub.PayloadDelimiter)
		}

		if o.config.Loggers.RedisPub.Mode == dnsutils.MODE_JSON {
			encoder.Encode(dm)
			o.transportWriter.WriteString(strconv.Quote(escape_buffer.String()))
			o.transportWriter.WriteString(o.config.Loggers.RedisPub.PayloadDelimiter)
		}

		if o.config.Loggers.RedisPub.Mode == dnsutils.MODE_FLATJSON {
			flat, err := dm.Flatten()
			if err != nil {
				o.LogError("flattening DNS message failed: %e", err)
				continue
			}
			encoder.Encode(flat)
			o.transportWriter.WriteString(strconv.Quote(escape_buffer.String()))
			o.transportWriter.WriteString(o.config.Loggers.RedisPub.PayloadDelimiter)
		}

		// flush the transport buffer
		err := o.transportWriter.Flush()
		if err != nil {
			o.LogError("send frame error", err.Error())
			o.writerReady = false
			<-o.transportReconnect
			break
		}
	}

	// reset buffer
	*buf = nil
}

func (o *RedisPub) Run() {
	o.LogInfo("running in background...")

	// prepare transforms
	listChannel := []chan dnsutils.DnsMessage{}
	listChannel = append(listChannel, o.outputChan)
	subprocessors := transformers.NewTransforms(&o.config.OutgoingTransformers, o.logger, o.name, listChannel, 0)

	// goroutine to process transformed dns messages
	go o.Process()

	// loop to process incoming messages
RUN_LOOP:
	for {
		select {
		case <-o.stopRun:
			// cleanup transformers
			subprocessors.Reset()

			o.doneRun <- true
			break RUN_LOOP

		case cfg, opened := <-o.configChan:
			if !opened {
				return
			}
			o.config = cfg
			o.ReadConfig()
			subprocessors.ReloadConfig(&cfg.OutgoingTransformers)

		case dm, opened := <-o.inputChan:
			if !opened {
				o.LogInfo("input channel closed!")
				return
			}

			// apply tranforms, init dns message with additionnals parts if necessary
			subprocessors.InitDnsMessageFormat(&dm)
			if subprocessors.ProcessMessage(&dm) == transformers.RETURN_DROP {
				continue
			}

			// send to output channel
			o.outputChan <- dm
		}
	}
	o.LogInfo("run terminated")
}

func (o *RedisPub) Process() {
	// init buffer
	bufferDm := []dnsutils.DnsMessage{}

	// init flust timer for buffer
	flushInterval := time.Duration(o.config.Loggers.RedisPub.FlushInterval) * time.Second
	flushTimer := time.NewTimer(flushInterval)

	// init remote conn
	go o.ConnectToRemote()

	o.LogInfo("ready to process")
PROCESS_LOOP:
	for {
		select {
		case <-o.stopProcess:
			// closing remote connection if exist
			o.Disconnect()
			o.doneProcess <- true
			break PROCESS_LOOP

		case <-o.transportReady:
			o.LogInfo("transport connected with success")
			o.transportWriter = bufio.NewWriter(o.transportConn)
			o.writerReady = true
			// read from the connection until we stop
			go o.ReadFromConnection()

		// incoming dns message to process
		case dm, opened := <-o.outputChan:
			if !opened {
				o.LogInfo("output channel closed!")
				return
			}

			// drop dns message if the connection is not ready to avoid memory leak or
			// to block the channel
			if !o.writerReady {
				continue
			}

			// append dns message to buffer
			bufferDm = append(bufferDm, dm)

			// buffer is full ?
			if len(bufferDm) >= o.config.Loggers.RedisPub.BufferSize {
				o.FlushBuffer(&bufferDm)
			}

		// flush the buffer
		case <-flushTimer.C:
			if !o.writerReady {
				o.LogInfo("Buffer cleared!")
				bufferDm = nil
				continue
			}

			if len(bufferDm) > 0 {
				o.FlushBuffer(&bufferDm)
			}

			// restart timer
			flushTimer.Reset(flushInterval)

		}
	}
	o.LogInfo("processing terminated")
}
