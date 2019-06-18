package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/Diogo-Costa/rds_exporter/basic"
	"github.com/Diogo-Costa/rds_exporter/client"
	"github.com/Diogo-Costa/rds_exporter/config"
	"github.com/Diogo-Costa/rds_exporter/enhanced"
	"github.com/Diogo-Costa/rds_exporter/sessions"
)

//nolint:lll
var (
	listenAddressF       = kingpin.Flag("web.listen-address", "Address on which to expose metrics and web interface.").Default(":9042").String()
	basicMetricsPathF    = kingpin.Flag("web.basic-telemetry-path", "Path under which to expose exporter's basic metrics.").Default("/basic").String()
	enhancedMetricsPathF = kingpin.Flag("web.enhanced-telemetry-path", "Path under which to expose exporter's enhanced metrics.").Default("/enhanced").String()
	configFileF          = kingpin.Flag("config.file", "Path to configuration file.").Default("config.yml").String()
	logTraceF            = kingpin.Flag("log.trace", "Enable verbose tracing of AWS requests (will log credentials).").Default("false").Bool()
	metricTypes          = kingpin.Flag("metric.type", "Which metrics do you want to monitor. (all|basic|enhanced)").Default("all").String()
)

func main() {
	log.AddFlags(kingpin.CommandLine)
	log.Infoln("Starting RDS exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	kingpin.Parse()

	cfg, err := config.Load(*configFileF)
	if err != nil {
		log.Fatalf("Can't read configuration file: %s", err)
	}

	client := client.New()
	sess, err := sessions.New(cfg.Instances, client.HTTP(), *logTraceF)
	if err != nil {
		log.Fatalf("Can't create sessions: %s", err)
	}

	// basic metrics + client metrics + exporter own metrics (ProcessCollector and GoCollector)
	if *metricTypes == "all" || *metricTypes == "basic" {
		prometheus.MustRegister(basic.New(cfg, sess))
		prometheus.MustRegister(client)
		http.Handle(*basicMetricsPathF, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
			ErrorLog:      log.NewErrorLogger(),
			ErrorHandling: promhttp.ContinueOnError,
		}))
		log.Infof("Basic metrics   : http://%s%s", *listenAddressF, *basicMetricsPathF)
	}

	// enhanced metrics
	if *metricTypes == "all" || *metricTypes == "enhanced" {
		registry := prometheus.NewRegistry()
		registry.MustRegister(enhanced.NewCollector(sess))
		http.Handle(*enhancedMetricsPathF, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
			ErrorLog:      log.NewErrorLogger(),
			ErrorHandling: promhttp.ContinueOnError,
		}))
		log.Infof("Enhanced metrics: http://%s%s", *listenAddressF, *enhancedMetricsPathF)
	}

	log.Fatal(http.ListenAndServe(*listenAddressF, nil))
}
