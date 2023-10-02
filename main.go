package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

func defaultConf() *Conf {
	return &Conf{
		Path:              "/ip-plz",
		Address:           ":8080",
		MetricsAddr:       ":9191",
		ReadTimeout:       1,
		WriteTimeout:      1,
		IdleTimeout:       5,
		ReadHeaderTimeout: 2,
	}
}

func ParseConf() *Conf {
	conf := defaultConf()
	if err := env.Parse(conf); err != nil {
		log.Fatalf("could not parse conf: %v", err)
	}
	return conf
}

func serveApp(ctx context.Context, wg *sync.WaitGroup, conf *Conf, ipPlz *IpPlz) {
	log.Printf("Starting server on '%s' at path '%s' using trusted headers '%v'\n", conf.Address, conf.Path, conf.TrustedHeaders)
	wg.Add(1)

	mux := http.NewServeMux()
	mux.HandleFunc(conf.Path, ipPlz.detectIp)
	mux.HandleFunc("/health", ipPlz.healthcheckHandler)

	server := &http.Server{
		Addr:              conf.Address,
		Handler:           mux,
		ReadTimeout:       time.Duration(conf.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(conf.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(conf.IdleTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(conf.ReadHeaderTimeout) * time.Second,
	}

	errChan := make(chan error)
	go func() {
		errChan <- server.ListenAndServe()
	}()

	select {
	case err := <-errChan:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("could not serve metrics: %v", err)
		}
	case <-ctx.Done():
		log.Println("Shutting down app server")
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		server.Shutdown(ctx)
		wg.Done()
	}
}

func serveMetrics(ctx context.Context, wg *sync.WaitGroup, conf *Conf) {
	log.Printf("Starting metrics server at '%s'\n", conf.MetricsAddr)
	wg.Add(1)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := http.Server{
		Addr:              conf.MetricsAddr,
		Handler:           mux,
		ReadTimeout:       time.Duration(conf.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(conf.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(conf.IdleTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(conf.ReadHeaderTimeout) * time.Second,
	}

	errChan := make(chan error)
	go func() {
		errChan <- server.ListenAndServe()
	}()

	select {
	case err := <-errChan:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("could not serve metrics: %v", err)
		}
	case <-ctx.Done():
		log.Println("Shutting down metrics server")
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		server.Shutdown(ctx)
		wg.Done()
	}
}

func main() {
	conditionalPrintVersion()

	log.Printf("ip-plz, version %s (%s)", BuildVersion, CommitHash)
	conf := ParseConf()

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	if len(conf.MetricsAddr) > 0 {
		go serveMetrics(ctx, wg, conf)
	}

	ipPlz := NewIpPlz(conf.TrustedHeaders)
	go func() {
		serveApp(ctx, wg, conf, ipPlz)
	}()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	<-done

	log.Println("Caught signal, quitting gracefully")
	cancel()
	wg.Wait()
	log.Println("Bye!")
}

func conditionalPrintVersion() {
	version := flag.Bool("version", false, "print version info")
	flag.Parse()
	if *version {
		fmt.Println(BuildVersion)
		os.Exit(0)
	}
}
