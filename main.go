package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"gopkg.in/alecthomas/kingpin.v2"
)

type promHTTPLogger struct {
	logger log.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	level.Error(l.logger).Log("msg", fmt.Sprint(v...))
}

func main() {
	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9725").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		opts          = SunnyBoyOpts{}
	)

	kingpin.Flag("sunnyboy.url", "Sunny boy URL").Required().StringVar(&opts.URL)
	kingpin.Flag("sunnyboy.skil-cert-validation", "Ignore self sign certificate").Default("true").BoolVar(&opts.SkipVerifyCert)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	exporter, err := NewExporter(opts, logger)
	if err != nil {
		level.Error(logger).Log("msg", "Error creating the exporter", "err", err)
		os.Exit(1)
	}
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath,
		promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer,
			promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{
					ErrorLog: &promHTTPLogger{
						logger: logger,
					},
				},
			),
		),
	)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Sunny Boy Exporter</title></head>
             <body>
             <h1>Sunny Boy Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	http.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})
	http.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
