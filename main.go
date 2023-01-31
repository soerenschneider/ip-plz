package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/caarlos0/env/v6"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

var (
	BuildVersion string
	CommitHash   string

	appName       = "ip-plz"
	requestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: strings.Replace(appName, "-", "_", -1),
		Name:      "requests_total",
		Help:      "The total number of processed requests",
	})
	requestsTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: strings.Replace(appName, "-", "_", -1),
		Name:      "most_recent_request_timestamp_seconds",
		Help:      "Timestamp of the most recent request received",
	})
)

type IpPlz struct {
	headers []string
}

type Conf struct {
	MetricsAddr       string   `env:"IP_PLZ_METRICS_ADDR"`
	Path              string   `env:"IP_PLZ_PATH"`
	Address           string   `env:"IP_PLZ_ADDR"`
	TrustedHeaders    []string `env:"IP_PLZ_TRUSTED_HEADERS" envSeparator:","`
	ReadTimeout       int      `env:"IP_PLZ_READ_TIMEOUT"`
	WriteTimeout      int      `env:"IP_PLZ_WRITE_TIMEOUT"`
	IdleTimeout       int      `env:"IP_PLZ_IDLE_TIMEOUT"`
	ReadHeaderTimeout int      `env:"IP_PLZ_READ_HEADER_TIMEOUT"`
}

func getDefaultConf() Conf {
	return Conf{
		Path:              "/ip-plz",
		Address:           ":8080",
		MetricsAddr:       ":9191",
		ReadTimeout:       1,
		WriteTimeout:      1,
		IdleTimeout:       5,
		ReadHeaderTimeout: 2,
	}
}

func NewIpPlz(trustedHeaders []string) *IpPlz {
	return &IpPlz{
		headers: trustedHeaders,
	}
}

func (b *IpPlz) getIp(req *http.Request) string {
	for _, h := range b.headers {
		for _, ip := range strings.Split(req.Header.Get(h), ",") {
			ip = strings.TrimSpace(ip)
			parsedIp := net.ParseIP(ip)
			if parsedIp != nil && parsedIp.IsGlobalUnicast() && !parsedIp.IsPrivate() {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return host
	}

	return req.RemoteAddr
}

func (b *IpPlz) detectIp(w http.ResponseWriter, req *http.Request) {
	requestsTotal.Inc()
	requestsTimestamp.SetToCurrentTime()
	pubIp := b.getIp(req)
	w.Write([]byte(pubIp))
}

func (b *IpPlz) healthcheckHandler(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("pong"))
}

func serveMetrics(addr string) {
	log.Printf("Serving metrics at '%s'\n", addr)
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("could not serve metrics: %v", err)
	}
}

func conditionalPrintVersion() {
	version := flag.Bool("version", false, "print version info")
	flag.Parse()
	if *version {
		fmt.Println(BuildVersion)
		os.Exit(0)
	}
}

func main() {
	conditionalPrintVersion()

	log.Printf("ip-plz, version %s (%s)", BuildVersion, CommitHash)
	conf := getDefaultConf()
	if err := env.Parse(&conf); err != nil {
		log.Fatalf("could not parse conf: %v", err)
	}

	if len(conf.MetricsAddr) > 0 {
		go serveMetrics(conf.MetricsAddr)
	}

	ipPlz := NewIpPlz(conf.TrustedHeaders)
	mux := http.NewServeMux()
	mux.HandleFunc(conf.Path, ipPlz.detectIp)
	mux.HandleFunc("/health", ipPlz.healthcheckHandler)

	httpServer := &http.Server{
		Addr:              conf.Address,
		Handler:           mux,
		ReadTimeout:       time.Duration(conf.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(conf.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(conf.IdleTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(conf.ReadHeaderTimeout) * time.Second,
	}

	go func() {
		log.Printf("Starting server on '%s' at path '%s' using trusted headers '%v'\n", conf.Address, conf.Path, conf.TrustedHeaders)
		err := httpServer.ListenAndServe()
		if err != nil {
			log.Fatalf("could not start server: %v", err)
		}
	}()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	<-done

	log.Println("Caught signal, quitting gracefully")
	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	httpServer.Shutdown(ctx)
}
