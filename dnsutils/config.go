package dnsutils

import (
	"os"

	"github.com/prometheus/prometheus/model/relabel"
	"gopkg.in/yaml.v3"
)

func IsValidMode(mode string) bool {
	switch mode {
	case
		MODE_TEXT,
		MODE_JSON,
		MODE_FLATJSON:
		return true
	}
	return false
}

type MultiplexInOut struct {
	Name       string                 `yaml:"name"`
	Transforms map[string]interface{} `yaml:"transforms"`
	Params     map[string]interface{} `yaml:",inline"`
}

type MultiplexRoutes struct {
	Src []string `yaml:"from,flow"`
	Dst []string `yaml:"to,flow"`
}

type ConfigTransformers struct {
	UserPrivacy struct {
		Enable        bool `yaml:"enable"`
		AnonymizeIP   bool `yaml:"anonymize-ip"`
		MinimazeQname bool `yaml:"minimaze-qname"`
		HashIP        bool `yaml:"hash-ip"`
	} `yaml:"user-privacy"`
	Normalize struct {
		Enable         bool `yaml:"enable"`
		QnameLowerCase bool `yaml:"qname-lowercase"`
		QuietText      bool `yaml:"quiet-text"`
		AddTld         bool `yaml:"add-tld"`
		AddTldPlusOne  bool `yaml:"add-tld-plus-one"`
	} `yaml:"normalize"`
	Latency struct {
		Enable            bool `yaml:"enable"`
		MeasureLatency    bool `yaml:"measure-latency"`
		UnansweredQueries bool `yaml:"unanswered-queries"`
		QueriesTimeout    int  `yaml:"queries-timeout"`
	}
	Reducer struct {
		Enable                    bool `yaml:"enable"`
		RepetitiveTrafficDetector bool `yaml:"repetitive-traffic-detector"`
		QnamePlusOne              bool `yaml:"qname-plus-one"`
		WatchInterval             int  `yaml:"watch-interval"`
	}
	Filtering struct {
		Enable          bool     `yaml:"enable"`
		DropFqdnFile    string   `yaml:"drop-fqdn-file"`
		DropDomainFile  string   `yaml:"drop-domain-file"`
		KeepFqdnFile    string   `yaml:"keep-fqdn-file"`
		KeepDomainFile  string   `yaml:"keep-domain-file"`
		DropQueryIpFile string   `yaml:"drop-queryip-file"`
		KeepQueryIpFile string   `yaml:"keep-queryip-file"`
		KeepRdataFile   string   `yaml:"keep-rdata-file"`
		DropRcodes      []string `yaml:"drop-rcodes,flow"`
		LogQueries      bool     `yaml:"log-queries"`
		LogReplies      bool     `yaml:"log-replies"`
		Downsample      int      `yaml:"downsample"`
	} `yaml:"filtering"`
	GeoIP struct {
		Enable        bool   `yaml:"enable"`
		DbCountryFile string `yaml:"mmdb-country-file"`
		DbCityFile    string `yaml:"mmdb-city-file"`
		DbAsnFile     string `yaml:"mmdb-asn-file"`
	} `yaml:"geoip"`
	Suspicious struct {
		Enable             bool     `yaml:"enable"`
		ThresholdQnameLen  int      `yaml:"threshold-qname-len"`
		ThresholdPacketLen int      `yaml:"threshold-packet-len"`
		ThresholdSlow      float64  `yaml:"threshold-slow"`
		CommonQtypes       []string `yaml:"common-qtypes,flow"`
		UnallowedChars     []string `yaml:"unallowed-chars,flow"`
		ThresholdMaxLabels int      `yaml:"threshold-max-labels"`
		WhitelistDomains   []string `yaml:"whitelist-domains,flow"`
	} `yaml:"suspicious"`
	Extract struct {
		Enable     bool `yaml:"enable"`
		AddPayload bool `yaml:"add-payload"`
	} `yaml:"extract"`
	MachineLearning struct {
		Enable      bool `yaml:"enable"`
		AddFeatures bool `yaml:"add-features"`
	} `yaml:"machine-learning"`
}

func (c *ConfigTransformers) SetDefault() {
	c.Suspicious.Enable = false
	c.Suspicious.ThresholdQnameLen = 100
	c.Suspicious.ThresholdPacketLen = 1000
	c.Suspicious.ThresholdSlow = 1.0
	c.Suspicious.CommonQtypes = []string{"A", "AAAA", "TXT", "CNAME", "PTR",
		"NAPTR", "DNSKEY", "SRV", "SOA", "NS", "MX", "DS", "HTTPS"}
	c.Suspicious.UnallowedChars = []string{"\"", "==", "/", ":"}
	c.Suspicious.ThresholdMaxLabels = 10
	c.Suspicious.WhitelistDomains = []string{"\\.ip6\\.arpa"}

	c.UserPrivacy.Enable = false
	c.UserPrivacy.AnonymizeIP = false
	c.UserPrivacy.MinimazeQname = false
	c.UserPrivacy.HashIP = false

	c.Normalize.Enable = false
	c.Normalize.QnameLowerCase = false
	c.Normalize.QuietText = false
	c.Normalize.AddTld = false
	c.Normalize.AddTldPlusOne = false

	c.Latency.Enable = false
	c.Latency.MeasureLatency = false
	c.Latency.UnansweredQueries = false
	c.Latency.QueriesTimeout = 2

	c.Reducer.Enable = false
	c.Reducer.RepetitiveTrafficDetector = false
	c.Reducer.QnamePlusOne = false
	c.Reducer.WatchInterval = 5

	c.Filtering.Enable = false
	c.Filtering.DropFqdnFile = ""
	c.Filtering.DropDomainFile = ""
	c.Filtering.KeepFqdnFile = ""
	c.Filtering.KeepDomainFile = ""
	c.Filtering.DropQueryIpFile = ""
	c.Filtering.DropRcodes = []string{}
	c.Filtering.LogQueries = true
	c.Filtering.LogReplies = true
	c.Filtering.Downsample = 0

	c.GeoIP.Enable = false
	c.GeoIP.DbCountryFile = ""
	c.GeoIP.DbCityFile = ""
	c.GeoIP.DbAsnFile = ""

	c.Extract.Enable = false
	c.Extract.AddPayload = false

	c.MachineLearning.Enable = false
	c.MachineLearning.AddFeatures = false
}

/* main configuration */
type Config struct {
	Global struct {
		TextFormat          string `yaml:"text-format"`
		TextFormatDelimiter string `yaml:"text-format-delimiter"`
		TextFormatBoundary  string `yaml:"text-format-boundary"`
		Trace               struct {
			Verbose      bool   `yaml:"verbose"`
			LogMalformed bool   `yaml:"log-malformed"`
			Filename     string `yaml:"filename"`
			MaxSize      int    `yaml:"max-size"`
			MaxBackups   int    `yaml:"max-backups"`
		} `yaml:"trace"`
		ServerIdentity string `yaml:"server-identity"`
	} `yaml:"global"`

	Collectors struct {
		Tail struct {
			Enable       bool   `yaml:"enable"`
			TimeLayout   string `yaml:"time-layout"`
			PatternQuery string `yaml:"pattern-query"`
			PatternReply string `yaml:"pattern-reply"`
			FilePath     string `yaml:"file-path"`
		} `yaml:"tail"`
		Dnstap struct {
			Enable            bool   `yaml:"enable"`
			ListenIP          string `yaml:"listen-ip"`
			ListenPort        int    `yaml:"listen-port"`
			SockPath          string `yaml:"sock-path"`
			TlsSupport        bool   `yaml:"tls-support"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			RcvBufSize        int    `yaml:"sock-rcvbuf"`
			ResetConn         bool   `yaml:"reset-conn"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"dnstap"`
		DnstapProxifier struct {
			Enable        bool   `yaml:"enable"`
			ListenIP      string `yaml:"listen-ip"`
			ListenPort    int    `yaml:"listen-port"`
			SockPath      string `yaml:"sock-path"`
			TlsSupport    bool   `yaml:"tls-support"`
			TlsMinVersion string `yaml:"tls-min-version"`
			CertFile      string `yaml:"cert-file"`
			KeyFile       string `yaml:"key-file"`
		} `yaml:"dnstap-proxifier"`
		AfpacketLiveCapture struct {
			Enable            bool   `yaml:"enable"`
			Port              int    `yaml:"port"`
			Device            string `yaml:"device"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"afpacket-sniffer"`
		XdpLiveCapture struct {
			Enable            bool   `yaml:"enable"`
			Port              int    `yaml:"port"`
			Device            string `yaml:"device"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"xdp-sniffer"`
		PowerDNS struct {
			Enable            bool   `yaml:"enable"`
			ListenIP          string `yaml:"listen-ip"`
			ListenPort        int    `yaml:"listen-port"`
			TlsSupport        bool   `yaml:"tls-support"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			AddDnsPayload     bool   `yaml:"add-dns-payload"`
			RcvBufSize        int    `yaml:"sock-rcvbuf"`
			ResetConn         bool   `yaml:"reset-conn"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"powerdns"`
		FileIngestor struct {
			Enable            bool   `yaml:"enable"`
			WatchDir          string `yaml:"watch-dir"`
			WatchMode         string `yaml:"watch-mode"`
			PcapDnsPort       int    `yaml:"pcap-dns-port"`
			DeleteAfter       bool   `yaml:"delete-after"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"file-ingestor"`
		Tzsp struct {
			Enable            bool   `yaml:"enable"`
			ListenIp          string `yaml:"listen-ip"`
			ListenPort        int    `yaml:"listen-port"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"tzsp"`
	} `yaml:"collectors"`

	IngoingTransformers ConfigTransformers `yaml:"collectors-transformers"`

	Loggers struct {
		Stdout struct {
			Enable            bool   `yaml:"enable"`
			Mode              string `yaml:"mode"`
			TextFormat        string `yaml:"text-format"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"stdout"`
		Prometheus struct {
			Enable                  bool     `yaml:"enable"`
			ListenIP                string   `yaml:"listen-ip"`
			ListenPort              int      `yaml:"listen-port"`
			TlsSupport              bool     `yaml:"tls-support"`
			TlsMutual               bool     `yaml:"tls-mutual"`
			TlsMinVersion           string   `yaml:"tls-min-version"`
			CertFile                string   `yaml:"cert-file"`
			KeyFile                 string   `yaml:"key-file"`
			PromPrefix              string   `yaml:"prometheus-prefix"`
			LabelsList              []string `yaml:"prometheus-labels"`
			TopN                    int      `yaml:"top-n"`
			BasicAuthLogin          string   `yaml:"basic-auth-login"`
			BasicAuthPwd            string   `yaml:"basic-auth-pwd"`
			BasicAuthEnabled        bool     `yaml:"basic-auth-enable"`
			ChannelBufferSize       int      `yaml:"chan-buffer-size"`
			HistogramMetricsEnabled bool     `yaml:"histogram-metrics-enabled"`
		} `yaml:"prometheus"`
		RestAPI struct {
			Enable            bool   `yaml:"enable"`
			ListenIP          string `yaml:"listen-ip"`
			ListenPort        int    `yaml:"listen-port"`
			BasicAuthLogin    string `yaml:"basic-auth-login"`
			BasicAuthPwd      string `yaml:"basic-auth-pwd"`
			TlsSupport        bool   `yaml:"tls-support"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			TopN              int    `yaml:"top-n"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"restapi"`
		LogFile struct {
			Enable              bool   `yaml:"enable"`
			FilePath            string `yaml:"file-path"`
			MaxSize             int    `yaml:"max-size"`
			MaxFiles            int    `yaml:"max-files"`
			FlushInterval       int    `yaml:"flush-interval"`
			Compress            bool   `yaml:"compress"`
			CompressInterval    int    `yaml:"compress-interval"`
			CompressPostCommand string `yaml:"compress-postcommand"`
			Mode                string `yaml:"mode"`
			PostRotateCommand   string `yaml:"postrotate-command"`
			PostRotateDelete    bool   `yaml:"postrotate-delete-success"`
			TextFormat          string `yaml:"text-format"`
			ChannelBufferSize   int    `yaml:"chan-buffer-size"`
		} `yaml:"logfile"`
		Dnstap struct {
			Enable            bool   `yaml:"enable"`
			RemoteAddress     string `yaml:"remote-address"`
			RemotePort        int    `yaml:"remote-port"`
			Transport         string `yaml:"transport"`
			SockPath          string `yaml:"sock-path"`
			ConnectTimeout    int    `yaml:"connect-timeout"`
			RetryInterval     int    `yaml:"retry-interval"`
			FlushInterval     int    `yaml:"flush-interval"`
			TlsSupport        bool   `yaml:"tls-support"`
			TlsInsecure       bool   `yaml:"tls-insecure"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CAFile            string `yaml:"ca-file"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			ServerId          string `yaml:"server-id"`
			OverwriteIdentity bool   `yaml:"overwrite-identity"`
			BufferSize        int    `yaml:"buffer-size"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"dnstap"`
		TcpClient struct {
			Enable            bool   `yaml:"enable"`
			RemoteAddress     string `yaml:"remote-address"`
			RemotePort        int    `yaml:"remote-port"`
			SockPath          string `yaml:"sock-path"` // deprecated
			RetryInterval     int    `yaml:"retry-interval"`
			Transport         string `yaml:"transport"`
			TlsSupport        bool   `yaml:"tls-support"` // deprecated
			TlsInsecure       bool   `yaml:"tls-insecure"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CAFile            string `yaml:"ca-file"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			Mode              string `yaml:"mode"`
			TextFormat        string `yaml:"text-format"`
			PayloadDelimiter  string `yaml:"delimiter"`
			BufferSize        int    `yaml:"buffer-size"`
			FlushInterval     int    `yaml:"flush-interval"`
			ConnectTimeout    int    `yaml:"connect-timeout"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"tcpclient"`
		Syslog struct {
			Enable            bool   `yaml:"enable"`
			Severity          string `yaml:"severity"`
			Facility          string `yaml:"facility"`
			Transport         string `yaml:"transport"`
			RemoteAddress     string `yaml:"remote-address"`
			RetryInterval     int    `yaml:"retry-interval"`
			TextFormat        string `yaml:"text-format"`
			Mode              string `yaml:"mode"`
			TlsInsecure       bool   `yaml:"tls-insecure"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CAFile            string `yaml:"ca-file"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			Formatter         string `yaml:"formatter"`
			Framer            string `yaml:"framer"`
			Hostname          string `yaml:"hostname"`
			AppName           string `yaml:"app-name"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
			Tag               string `yaml:"tag"`
		} `yaml:"syslog"`
		Fluentd struct {
			Enable            bool   `yaml:"enable"`
			RemoteAddress     string `yaml:"remote-address"`
			RemotePort        int    `yaml:"remote-port"`
			SockPath          string `yaml:"sock-path"` // deprecated
			ConnectTimeout    int    `yaml:"connect-timeout"`
			RetryInterval     int    `yaml:"retry-interval"`
			FlushInterval     int    `yaml:"flush-interval"`
			Transport         string `yaml:"transport"`
			TlsSupport        bool   `yaml:"tls-support"` // deprecated
			TlsInsecure       bool   `yaml:"tls-insecure"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CAFile            string `yaml:"ca-file"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			Tag               string `yaml:"tag"`
			BufferSize        int    `yaml:"buffer-size"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"fluentd"`
		InfluxDB struct {
			Enable            bool   `yaml:"enable"`
			ServerURL         string `yaml:"server-url"`
			AuthToken         string `yaml:"auth-token"`
			TlsSupport        bool   `yaml:"tls-support"`
			TlsInsecure       bool   `yaml:"tls-insecure"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CAFile            string `yaml:"ca-file"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			Bucket            string `yaml:"bucket"`
			Organization      string `yaml:"organization"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"influxdb"`
		LokiClient struct {
			Enable            bool              `yaml:"enable"`
			ServerURL         string            `yaml:"server-url"`
			JobName           string            `yaml:"job-name"`
			Mode              string            `yaml:"mode"`
			FlushInterval     int               `yaml:"flush-interval"`
			BatchSize         int               `yaml:"batch-size"`
			RetryInterval     int               `yaml:"retry-interval"`
			TextFormat        string            `yaml:"text-format"`
			ProxyURL          string            `yaml:"proxy-url"`
			TlsInsecure       bool              `yaml:"tls-insecure"`
			TlsMinVersion     string            `yaml:"tls-min-version"`
			CAFile            string            `yaml:"ca-file"`
			CertFile          string            `yaml:"cert-file"`
			KeyFile           string            `yaml:"key-file"`
			BasicAuthLogin    string            `yaml:"basic-auth-login"`
			BasicAuthPwd      string            `yaml:"basic-auth-pwd"`
			BasicAuthPwdFile  string            `yaml:"basic-auth-pwd-file"`
			TenantId          string            `yaml:"tenant-id"`
			RelabelConfigs    []*relabel.Config `yaml:"relabel-configs"`
			ChannelBufferSize int               `yaml:"chan-buffer-size"`
		} `yaml:"lokiclient"`
		Statsd struct {
			Enable            bool   `yaml:"enable"`
			Prefix            string `yaml:"prefix"`
			RemoteAddress     string `yaml:"remote-address"`
			RemotePort        int    `yaml:"remote-port"`
			ConnectTimeout    int    `yaml:"connect-timeout"`
			Transport         string `yaml:"transport"`
			FlushInterval     int    `yaml:"flush-interval"`
			TlsSupport        bool   `yaml:"tls-support"` //deprecated
			TlsInsecure       bool   `yaml:"tls-insecure"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CAFile            string `yaml:"ca-file"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"statsd"`
		ElasticSearchClient struct {
			Enable            bool   `yaml:"enable"`
			Index             string `yaml:"index"`
			Server            string `yaml:"server"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
			BulkSize          int    `yaml:"bulk-size"`
			FlushInterval     int    `yaml:"flush-interval"`
		} `yaml:"elasticsearch"`
		ScalyrClient struct {
			Enable            bool                   `yaml:"enable"`
			Mode              string                 `yaml:"mode"`
			TextFormat        string                 `yaml:"text-format"`
			SessionInfo       map[string]string      `yaml:"sessioninfo"`
			Attrs             map[string]interface{} `yaml:"attrs"`
			ServerURL         string                 `yaml:"server-url"`
			ApiKey            string                 `yaml:"apikey"`
			Parser            string                 `yaml:"parser"`
			FlushInterval     int                    `yaml:"flush-interval"`
			ProxyURL          string                 `yaml:"proxy-url"`
			TlsInsecure       bool                   `yaml:"tls-insecure"`
			TlsMinVersion     string                 `yaml:"tls-min-version"`
			CAFile            string                 `yaml:"ca-file"`
			CertFile          string                 `yaml:"cert-file"`
			KeyFile           string                 `yaml:"key-file"`
			ChannelBufferSize int                    `yaml:"chan-buffer-size"`
		} `yaml:"scalyrclient"`
		RedisPub struct {
			Enable            bool   `yaml:"enable"`
			RemoteAddress     string `yaml:"remote-address"`
			RemotePort        int    `yaml:"remote-port"`
			SockPath          string `yaml:"sock-path"` // deprecated
			RetryInterval     int    `yaml:"retry-interval"`
			Transport         string `yaml:"transport"`
			TlsSupport        bool   `yaml:"tls-support"` // deprecated
			TlsInsecure       bool   `yaml:"tls-insecure"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CAFile            string `yaml:"ca-file"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			Mode              string `yaml:"mode"`
			TextFormat        string `yaml:"text-format"`
			PayloadDelimiter  string `yaml:"delimiter"`
			BufferSize        int    `yaml:"buffer-size"`
			FlushInterval     int    `yaml:"flush-interval"`
			ConnectTimeout    int    `yaml:"connect-timeout"`
			RedisChannel      string `yaml:"redis-channel"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"redispub"`
		KafkaProducer struct {
			Enable            bool   `yaml:"enable"`
			RemoteAddress     string `yaml:"remote-address"`
			RemotePort        int    `yaml:"remote-port"`
			RetryInterval     int    `yaml:"retry-interval"`
			TlsSupport        bool   `yaml:"tls-support"`
			TlsInsecure       bool   `yaml:"tls-insecure"`
			TlsMinVersion     string `yaml:"tls-min-version"`
			CAFile            string `yaml:"ca-file"`
			CertFile          string `yaml:"cert-file"`
			KeyFile           string `yaml:"key-file"`
			SaslSupport       bool   `yaml:"sasl-support"`
			SaslUsername      string `yaml:"sasl-username"`
			SaslPassword      string `yaml:"sasl-password"`
			SaslMechanism     string `yaml:"sasl-mechanism"`
			Mode              string `yaml:"mode"`
			BufferSize        int    `yaml:"buffer-size"`
			FlushInterval     int    `yaml:"flush-interval"`
			ConnectTimeout    int    `yaml:"connect-timeout"`
			Topic             string `yaml:"topic"`
			Partition         int    `yaml:"partition"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"kafkaproducer"`
		FalcoClient struct {
			Enable            bool   `yaml:"enable"`
			URL               string `yaml:"url"`
			ChannelBufferSize int    `yaml:"chan-buffer-size"`
		} `yaml:"falco"`
	} `yaml:"loggers"`

	OutgoingTransformers ConfigTransformers `yaml:"loggers-transformers"`

	Multiplexer struct {
		Collectors []MultiplexInOut  `yaml:"collectors"`
		Loggers    []MultiplexInOut  `yaml:"loggers"`
		Routes     []MultiplexRoutes `yaml:"routes"`
	} `yaml:"multiplexer"`
}

func (c *Config) SetDefault() {
	// global config
	c.Global.TextFormat = "timestamp identity operation rcode queryip queryport family protocol length qname qtype latency"
	c.Global.TextFormatDelimiter = " "
	c.Global.TextFormatBoundary = "\""

	c.Global.Trace.Verbose = false
	c.Global.Trace.LogMalformed = false
	c.Global.Trace.Filename = ""
	c.Global.Trace.MaxSize = 10
	c.Global.Trace.MaxBackups = 10
	c.Global.ServerIdentity = ""

	// multiplexer
	c.Multiplexer.Collectors = []MultiplexInOut{}
	c.Multiplexer.Loggers = []MultiplexInOut{}
	c.Multiplexer.Routes = []MultiplexRoutes{}

	// Collectors
	c.Collectors.Tail.Enable = false
	c.Collectors.Tail.TimeLayout = ""
	c.Collectors.Tail.PatternQuery = ""
	c.Collectors.Tail.PatternReply = ""
	c.Collectors.Tail.FilePath = ""

	c.Collectors.Dnstap.Enable = false
	c.Collectors.Dnstap.ListenIP = ANY_IP
	c.Collectors.Dnstap.ListenPort = 6000
	c.Collectors.Dnstap.SockPath = ""
	c.Collectors.Dnstap.TlsSupport = false
	c.Collectors.Dnstap.TlsMinVersion = TLS_v12
	c.Collectors.Dnstap.CertFile = ""
	c.Collectors.Dnstap.KeyFile = ""
	c.Collectors.Dnstap.RcvBufSize = 0
	c.Collectors.Dnstap.ResetConn = true
	c.Collectors.Dnstap.ChannelBufferSize = 65535

	c.Collectors.DnstapProxifier.Enable = false
	c.Collectors.DnstapProxifier.ListenIP = ANY_IP
	c.Collectors.DnstapProxifier.ListenPort = 6000
	c.Collectors.DnstapProxifier.SockPath = ""
	c.Collectors.DnstapProxifier.TlsSupport = false
	c.Collectors.DnstapProxifier.TlsMinVersion = TLS_v12
	c.Collectors.DnstapProxifier.CertFile = ""
	c.Collectors.DnstapProxifier.KeyFile = ""

	c.Collectors.XdpLiveCapture.Enable = false
	c.Collectors.XdpLiveCapture.Device = ""
	c.Collectors.XdpLiveCapture.ChannelBufferSize = 65535

	c.Collectors.AfpacketLiveCapture.Enable = false
	c.Collectors.AfpacketLiveCapture.Port = 53
	c.Collectors.AfpacketLiveCapture.Device = ""
	c.Collectors.AfpacketLiveCapture.ChannelBufferSize = 65535

	c.Collectors.PowerDNS.Enable = false
	c.Collectors.PowerDNS.ListenIP = ANY_IP
	c.Collectors.PowerDNS.ListenPort = 6001
	c.Collectors.PowerDNS.TlsSupport = false
	c.Collectors.PowerDNS.TlsMinVersion = TLS_v12
	c.Collectors.PowerDNS.CertFile = ""
	c.Collectors.PowerDNS.KeyFile = ""
	c.Collectors.PowerDNS.AddDnsPayload = false
	c.Collectors.PowerDNS.RcvBufSize = 0
	c.Collectors.PowerDNS.ResetConn = true
	c.Collectors.PowerDNS.ChannelBufferSize = 65535

	c.Collectors.FileIngestor.Enable = false
	c.Collectors.FileIngestor.WatchDir = ""
	c.Collectors.FileIngestor.PcapDnsPort = 53
	c.Collectors.FileIngestor.WatchMode = MODE_PCAP
	c.Collectors.FileIngestor.DeleteAfter = false
	c.Collectors.FileIngestor.ChannelBufferSize = 65535

	c.Collectors.Tzsp.Enable = false
	c.Collectors.Tzsp.ListenIp = ANY_IP
	c.Collectors.Tzsp.ListenPort = 10000
	c.Collectors.Tzsp.ChannelBufferSize = 65535

	// Transformers for collectors
	c.IngoingTransformers.SetDefault()

	// Loggers
	c.Loggers.Stdout.Enable = false
	c.Loggers.Stdout.Mode = MODE_TEXT
	c.Loggers.Stdout.TextFormat = ""
	c.Loggers.Stdout.ChannelBufferSize = 65535

	c.Loggers.Dnstap.Enable = false
	c.Loggers.Dnstap.RemoteAddress = LOCALHOST_IP
	c.Loggers.Dnstap.RemotePort = 6000
	c.Loggers.Dnstap.Transport = SOCKET_TCP
	c.Loggers.Dnstap.ConnectTimeout = 5
	c.Loggers.Dnstap.RetryInterval = 10
	c.Loggers.Dnstap.FlushInterval = 30
	c.Loggers.Dnstap.SockPath = ""
	c.Loggers.Dnstap.TlsSupport = false
	c.Loggers.Dnstap.TlsInsecure = false
	c.Loggers.Dnstap.TlsMinVersion = TLS_v12
	c.Loggers.Dnstap.CAFile = ""
	c.Loggers.Dnstap.CertFile = ""
	c.Loggers.Dnstap.KeyFile = ""
	c.Loggers.Dnstap.ServerId = ""
	c.Loggers.Dnstap.OverwriteIdentity = false
	c.Loggers.Dnstap.BufferSize = 100
	c.Loggers.Dnstap.ChannelBufferSize = 65535

	c.Loggers.LogFile.Enable = false
	c.Loggers.LogFile.FilePath = ""
	c.Loggers.LogFile.FlushInterval = 10
	c.Loggers.LogFile.MaxSize = 100
	c.Loggers.LogFile.MaxFiles = 10
	c.Loggers.LogFile.Compress = false
	c.Loggers.LogFile.CompressInterval = 60
	c.Loggers.LogFile.CompressPostCommand = ""
	c.Loggers.LogFile.Mode = MODE_TEXT
	c.Loggers.LogFile.PostRotateCommand = ""
	c.Loggers.LogFile.PostRotateDelete = false
	c.Loggers.LogFile.TextFormat = ""
	c.Loggers.LogFile.ChannelBufferSize = 65535

	c.Loggers.Prometheus.Enable = false
	c.Loggers.Prometheus.ListenIP = LOCALHOST_IP
	c.Loggers.Prometheus.ListenPort = 8081
	c.Loggers.Prometheus.TlsSupport = false
	c.Loggers.Prometheus.TlsMutual = false
	c.Loggers.Prometheus.TlsMinVersion = TLS_v12
	c.Loggers.Prometheus.CertFile = ""
	c.Loggers.Prometheus.KeyFile = ""
	c.Loggers.Prometheus.PromPrefix = PROG_NAME
	c.Loggers.Prometheus.TopN = 10
	c.Loggers.Prometheus.BasicAuthLogin = "admin"
	c.Loggers.Prometheus.BasicAuthPwd = "changeme"
	c.Loggers.Prometheus.BasicAuthEnabled = true
	c.Loggers.Prometheus.ChannelBufferSize = 65535
	c.Loggers.Prometheus.HistogramMetricsEnabled = false

	c.Loggers.RestAPI.Enable = false
	c.Loggers.RestAPI.ListenIP = LOCALHOST_IP
	c.Loggers.RestAPI.ListenPort = 8080
	c.Loggers.RestAPI.BasicAuthLogin = "admin"
	c.Loggers.RestAPI.BasicAuthPwd = "changeme"
	c.Loggers.RestAPI.TlsSupport = false
	c.Loggers.RestAPI.TlsMinVersion = TLS_v12
	c.Loggers.RestAPI.CertFile = ""
	c.Loggers.RestAPI.KeyFile = ""
	c.Loggers.RestAPI.TopN = 100
	c.Loggers.RestAPI.ChannelBufferSize = 65535

	c.Loggers.TcpClient.Enable = false
	c.Loggers.TcpClient.RemoteAddress = LOCALHOST_IP
	c.Loggers.TcpClient.RemotePort = 9999
	c.Loggers.TcpClient.SockPath = ""
	c.Loggers.TcpClient.RetryInterval = 10
	c.Loggers.TcpClient.Transport = SOCKET_TCP
	c.Loggers.TcpClient.TlsSupport = false
	c.Loggers.TcpClient.TlsInsecure = false
	c.Loggers.TcpClient.TlsMinVersion = TLS_v12
	c.Loggers.TcpClient.CAFile = ""
	c.Loggers.TcpClient.CertFile = ""
	c.Loggers.TcpClient.KeyFile = ""
	c.Loggers.TcpClient.Mode = MODE_FLATJSON
	c.Loggers.TcpClient.TextFormat = ""
	c.Loggers.TcpClient.PayloadDelimiter = "\n"
	c.Loggers.TcpClient.BufferSize = 100
	c.Loggers.TcpClient.ConnectTimeout = 5
	c.Loggers.TcpClient.FlushInterval = 30
	c.Loggers.TcpClient.ChannelBufferSize = 65535

	c.Loggers.Syslog.Enable = false
	c.Loggers.Syslog.Severity = "INFO"
	c.Loggers.Syslog.Facility = "DAEMON"
	c.Loggers.Syslog.Transport = "local"
	c.Loggers.Syslog.RemoteAddress = "127.0.0.1:514"
	c.Loggers.Syslog.TextFormat = ""
	c.Loggers.Syslog.Mode = MODE_TEXT
	c.Loggers.Syslog.RetryInterval = 10
	c.Loggers.Syslog.TlsInsecure = false
	c.Loggers.Syslog.TlsMinVersion = TLS_v12
	c.Loggers.Syslog.CAFile = ""
	c.Loggers.Syslog.CertFile = ""
	c.Loggers.Syslog.KeyFile = ""
	c.Loggers.Syslog.ChannelBufferSize = 65535
	c.Loggers.Syslog.Tag = ""
	c.Loggers.Syslog.Framer = ""
	c.Loggers.Syslog.Formatter = "rfc5424"
	c.Loggers.Syslog.Hostname = ""
	c.Loggers.Syslog.AppName = "DNScollector"

	c.Loggers.Fluentd.Enable = false
	c.Loggers.Fluentd.RemoteAddress = LOCALHOST_IP
	c.Loggers.Fluentd.RemotePort = 24224
	c.Loggers.Fluentd.SockPath = "" // deprecated
	c.Loggers.Fluentd.RetryInterval = 10
	c.Loggers.Fluentd.ConnectTimeout = 5
	c.Loggers.Fluentd.FlushInterval = 30
	c.Loggers.Fluentd.Transport = SOCKET_TCP
	c.Loggers.Fluentd.TlsSupport = false // deprecated
	c.Loggers.Fluentd.TlsInsecure = false
	c.Loggers.Fluentd.TlsMinVersion = TLS_v12
	c.Loggers.Fluentd.CAFile = ""
	c.Loggers.Fluentd.CertFile = ""
	c.Loggers.Fluentd.KeyFile = ""
	c.Loggers.Fluentd.Tag = "dns.collector"
	c.Loggers.Fluentd.BufferSize = 100
	c.Loggers.Fluentd.ChannelBufferSize = 65535

	c.Loggers.InfluxDB.Enable = false
	c.Loggers.InfluxDB.ServerURL = "http://localhost:8086"
	c.Loggers.InfluxDB.AuthToken = ""
	c.Loggers.InfluxDB.TlsSupport = false
	c.Loggers.InfluxDB.TlsInsecure = false
	c.Loggers.InfluxDB.TlsMinVersion = TLS_v12
	c.Loggers.InfluxDB.CAFile = ""
	c.Loggers.InfluxDB.CertFile = ""
	c.Loggers.InfluxDB.KeyFile = ""
	c.Loggers.InfluxDB.Bucket = ""
	c.Loggers.InfluxDB.Organization = ""
	c.Loggers.InfluxDB.ChannelBufferSize = 65535

	c.Loggers.LokiClient.Enable = false
	c.Loggers.LokiClient.ServerURL = "http://localhost:3100/loki/api/v1/push"
	c.Loggers.LokiClient.JobName = PROG_NAME
	c.Loggers.LokiClient.Mode = MODE_TEXT
	c.Loggers.LokiClient.FlushInterval = 5
	c.Loggers.LokiClient.BatchSize = 1024 * 1024
	c.Loggers.LokiClient.RetryInterval = 10
	c.Loggers.LokiClient.TextFormat = ""
	c.Loggers.LokiClient.ProxyURL = ""
	c.Loggers.LokiClient.TlsInsecure = false
	c.Loggers.LokiClient.TlsMinVersion = TLS_v12
	c.Loggers.LokiClient.CAFile = ""
	c.Loggers.LokiClient.CertFile = ""
	c.Loggers.LokiClient.KeyFile = ""
	c.Loggers.LokiClient.BasicAuthLogin = ""
	c.Loggers.LokiClient.BasicAuthPwd = ""
	c.Loggers.LokiClient.BasicAuthPwdFile = ""
	c.Loggers.LokiClient.TenantId = ""
	c.Loggers.LokiClient.ChannelBufferSize = 65535

	c.Loggers.Statsd.Enable = false
	c.Loggers.Statsd.Prefix = PROG_NAME
	c.Loggers.Statsd.RemoteAddress = LOCALHOST_IP
	c.Loggers.Statsd.RemotePort = 8125
	c.Loggers.Statsd.Transport = SOCKET_UDP
	c.Loggers.Statsd.ConnectTimeout = 5
	c.Loggers.Statsd.FlushInterval = 10
	c.Loggers.Statsd.TlsSupport = false // deprecated
	c.Loggers.Statsd.TlsInsecure = false
	c.Loggers.Statsd.TlsMinVersion = TLS_v12
	c.Loggers.Statsd.CAFile = ""
	c.Loggers.Statsd.CertFile = ""
	c.Loggers.Statsd.KeyFile = ""
	c.Loggers.Statsd.ChannelBufferSize = 65535

	c.Loggers.ElasticSearchClient.Enable = false
	c.Loggers.ElasticSearchClient.Server = "http://127.0.0.1:9200/"
	c.Loggers.ElasticSearchClient.Index = ""
	c.Loggers.ElasticSearchClient.ChannelBufferSize = 65535
	c.Loggers.ElasticSearchClient.BulkSize = 100
	c.Loggers.ElasticSearchClient.FlushInterval = 10

	c.Loggers.RedisPub.Enable = false
	c.Loggers.RedisPub.RemoteAddress = LOCALHOST_IP
	c.Loggers.RedisPub.RemotePort = 6379
	c.Loggers.RedisPub.SockPath = ""
	c.Loggers.RedisPub.RetryInterval = 10
	c.Loggers.RedisPub.Transport = SOCKET_TCP
	c.Loggers.RedisPub.TlsSupport = false
	c.Loggers.RedisPub.TlsInsecure = false
	c.Loggers.RedisPub.TlsMinVersion = TLS_v12
	c.Loggers.RedisPub.CAFile = ""
	c.Loggers.RedisPub.CertFile = ""
	c.Loggers.RedisPub.KeyFile = ""
	c.Loggers.RedisPub.Mode = MODE_FLATJSON
	c.Loggers.RedisPub.TextFormat = ""
	c.Loggers.RedisPub.PayloadDelimiter = "\n"
	c.Loggers.RedisPub.BufferSize = 100
	c.Loggers.RedisPub.ConnectTimeout = 5
	c.Loggers.RedisPub.FlushInterval = 30
	c.Loggers.RedisPub.RedisChannel = "dns_collector"
	c.Loggers.RedisPub.ChannelBufferSize = 65535

	c.Loggers.KafkaProducer.Enable = false
	c.Loggers.KafkaProducer.RemoteAddress = LOCALHOST_IP
	c.Loggers.KafkaProducer.RemotePort = 9092
	c.Loggers.KafkaProducer.RetryInterval = 10
	c.Loggers.KafkaProducer.TlsSupport = false
	c.Loggers.KafkaProducer.TlsInsecure = false
	c.Loggers.KafkaProducer.TlsMinVersion = TLS_v12
	c.Loggers.KafkaProducer.CAFile = ""
	c.Loggers.KafkaProducer.CertFile = ""
	c.Loggers.KafkaProducer.KeyFile = ""
	c.Loggers.KafkaProducer.SaslSupport = false
	c.Loggers.KafkaProducer.SaslUsername = ""
	c.Loggers.KafkaProducer.SaslPassword = ""
	c.Loggers.KafkaProducer.SaslMechanism = SASL_MECHANISM_PLAIN
	c.Loggers.KafkaProducer.Mode = MODE_FLATJSON
	c.Loggers.KafkaProducer.BufferSize = 100
	c.Loggers.KafkaProducer.ConnectTimeout = 5
	c.Loggers.KafkaProducer.FlushInterval = 10
	c.Loggers.KafkaProducer.Topic = "dnscollector"
	c.Loggers.KafkaProducer.Partition = 0
	c.Loggers.KafkaProducer.ChannelBufferSize = 65535

	c.Loggers.FalcoClient.Enable = false
	c.Loggers.FalcoClient.URL = "http://127.0.0.1:9200"
	c.Loggers.FalcoClient.ChannelBufferSize = 65535

	// Transformers for loggers
	c.OutgoingTransformers.SetDefault()

}

func (c *Config) GetServerIdentity() string {
	if len(c.Global.ServerIdentity) > 0 {
		return c.Global.ServerIdentity
	} else {
		hostname, err := os.Hostname()
		if err == nil {
			return hostname
		} else {
			return "undefined"
		}
	}
}

func ReloadConfig(configPath string, config *Config) error {
	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return err
	}
	return nil
}

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}
	config.SetDefault()

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func GetFakeConfig() *Config {
	config := &Config{}
	config.SetDefault()
	return config
}

func GetFakeConfigTransformers() *ConfigTransformers {
	config := &ConfigTransformers{}
	config.SetDefault()
	return config
}
