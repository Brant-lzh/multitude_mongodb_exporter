package main

import (
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/percona/mongodb_exporter/exporter"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

//nolint:gochecknoglobals
var (
	version   string
	commit    string
	buildDate string

	//初始化配置
	conf config
	opts Flags
	log  = logrus.New()
	//cache exporter
	//cacheExporters = make(map[string]*exporter.Exporter)
	cache  = InitCacheExporter()
	levels = map[string]logrus.Level{
		"debug": logrus.DebugLevel,
		"error": logrus.ErrorLevel,
		"fatal": logrus.FatalLevel,
		"info":  logrus.InfoLevel,
		"warn":  logrus.WarnLevel,
	}
)

type Flags struct {
	ConfigPath string `name:"config" help:"config path"  env:"CONFIG" default:"./config/config.yaml"`

	CollStatsNamespaces   string `name:"mongodb.collstats-colls" help:"List of comma separared databases.collections to get $collStats" placeholder:"db1,db2.col2"`
	IndexStatsCollections string `name:"mongodb.indexstats-colls" help:"List of comma separared databases.collections to get $indexStats" placeholder:"db1.col1,db2.col2"`
	URI                   string `name:"mongodb.uri" help:"MongoDB connection URI" env:"MONGODB_URI" placeholder:"mongodb://user:pass@127.0.0.1:27017/admin?ssl=true"`
	GlobalConnPool        bool   `name:"mongodb.global-conn-pool" help:"Use global connection pool instead of creating new pool for each http request." negatable:""`
	DirectConnect         bool   `name:"mongodb.direct-connect" help:"Whether or not a direct connect should be made. Direct connections are not valid if multiple hosts are specified or an SRV URI is used." default:"false" negatable:""`
	WebListenAddress      string `name:"web.listen-address" help:"Address to listen on for web interface and telemetry" env:"LISTEN_ADDRESS" default:":58080"`
	WebTelemetryPath      string `name:"web.telemetry-path" help:"Metrics expose path" env:"TELEMETRY_PATH" default:"/metrics"`
	TLSConfigPath         string `name:"web.config" help:"Path to the file having Prometheus TLS config for basic auth"`
	LogLevel              string `name:"log.level" help:"Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]" enum:"debug,info,warn,error,fatal" env:"LOG_LEVEL" default:"error"`

	EnableDiagnosticData   bool `name:"collector.diagnosticdata" help:"Enable collecting metrics from getDiagnosticData"`
	EnableReplicasetStatus bool `name:"collector.replicasetstatus" help:"Enable collecting metrics from replSetGetStatus"`
	EnableDBStats          bool `name:"collector.dbstats" help:"Enable collecting metrics from dbStats"`
	EnableTopMetrics       bool `name:"collector.topmetrics" help:"Enable collecting metrics from top admin command"`
	EnableIndexStats       bool `name:"collector.indexstats" help:"Enable collecting metrics from $indexStats"`
	EnableCollStats        bool `name:"collector.collstats" help:"Enable collecting metrics from $collStats"`

	EnableOverrideDescendingIndex bool `name:"metrics.overridedescendingindex" help:"Enable descending index name override to replace -1 with _DESC"`

	CollectAll bool `name:"collect-all" help:"Enable all collectors. Same as specifying all --collector.<name>"`

	CollStatsLimit int `name:"collector.collstats-limit" help:"Disable collstats, dbstats, topmetrics and indexstats collector if there are more than <n> collections. 0=No limit" default:"0"`

	DiscoveringMode bool `name:"discovering-mode" help:"Enable autodiscover collections" negatable:""`
	CompatibleMode  bool `name:"compatible-mode" help:"Enable old mongodb-exporter compatible metrics" negatable:""`
	Version         bool `name:"version" help:"Show version and exit"`
}

func main() {
	_ = kong.Parse(&opts,
		kong.Name("mongodb_exporter"),
		kong.Description("MongoDB Prometheus exporter"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": version,
		})
	if opts.Version {
		fmt.Println("mongodb_exporter - MongoDB Prometheus exporter")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build date: %s\n", buildDate)
		//return
	}
	//log
	log.SetLevel(levels[opts.LogLevel])
	log.Debugf("Compatible mode: %v", opts.CompatibleMode)
	//config
	if err := conf.Init(opts.ConfigPath); err != nil {
		panic(err)
	}
	fmt.Println("config path:", opts.ConfigPath)
	Run(&opts)
}

func Run(opts *Flags) {
	mux := http.DefaultServeMux
	mux.HandleFunc(opts.WebTelemetryPath, resourceHandler)
	server := &http.Server{
		Addr:    opts.WebListenAddress,
		Handler: mux,
	}

	if err := web.ListenAndServe(server, opts.TLSConfigPath, promlog.New(&promlog.Config{})); err != nil {
		log.Errorf("error starting server: %v", err)
		os.Exit(1)
	}
}

func resourceHandler(w http.ResponseWriter, r *http.Request) {
	req, err := banding(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		unescape, _ := url.QueryUnescape(r.URL.RawQuery)
		_, _ = w.Write([]byte(err.Error() + unescape))
		return
	}
	var (
		e     *multiExporter
		found bool
	)
	if e, found = cache.Get(req.String()); !found {
		//new exporter
		opts.URI = "mongodb://" + conf.Username + ":" + conf.Password + "@" + req.Address + "/?replicaSet=replset"
		log.Debugf("Connection URI: %s", opts.URI)

		exporterOpts := &exporter.Opts{
			DisableDefaultRegistry: true,
			CollStatsNamespaces:    strings.Split(opts.CollStatsNamespaces, ","),
			CompatibleMode:         opts.CompatibleMode,
			DiscoveringMode:        opts.DiscoveringMode,
			IndexStatsCollections:  strings.Split(opts.IndexStatsCollections, ","),
			Logger:                 log,
			Path:                   opts.WebTelemetryPath,
			URI:                    opts.URI,
			GlobalConnPool:         opts.GlobalConnPool,
			WebListenAddress:       opts.WebListenAddress,
			TLSConfigPath:          opts.TLSConfigPath,
			DirectConnect:          opts.DirectConnect,

			EnableDiagnosticData:   opts.EnableDiagnosticData,
			EnableReplicasetStatus: opts.EnableReplicasetStatus,
			EnableTopMetrics:       opts.EnableTopMetrics,
			EnableDBStats:          opts.EnableDBStats,
			EnableIndexStats:       opts.EnableIndexStats,
			EnableCollStats:        opts.EnableCollStats,

			EnableOverrideDescendingIndex: opts.EnableOverrideDescendingIndex,

			CollStatsLimit: opts.CollStatsLimit,
			CollectAll:     opts.CollectAll,
		}
		e = NewMultiExporter(exporterOpts)
		cache.Set(req.String(), e)
	} else {
		e.SetUpdateTime(time.Now())
	}
	e.exp.Handler().ServeHTTP(w, r)
}

//config

type config struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

func (c *config) Init(path string) error {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(file, c)
	if err != nil {
		return err
	}
	return nil
}
