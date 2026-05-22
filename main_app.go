//go:build app

package main

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	configFileEnvName = "IP_PLZ_CONFIG_FILE"

	appName = "ip-plz"
)

var (
	BuildVersion string
	CommitHash   string

	namedClientsTemplate = template.Must(template.New("clients").Funcs(template.FuncMap{
		"relativeTime": relativeTime,
	}).Parse(`<!DOCTYPE html>
<html>
<head><title>Named Clients</title></head>
<body>
  <h1>Named Clients</h1>
  <table border="1">
    <tr><th>Name</th><th>IP Address</th><th>Last Seen</th><th>Last Seen Ago</th></tr>
    {{range .}}
    <tr>
      <td>{{.Name}}</td>
      <td>{{.IpAddress}}</td>
      <td>{{if .TimeSeen.IsZero}}Never{{else}}{{.TimeSeen.Format "2006-01-02T15:04:05Z07:00"}}{{end}}</td>
      <td>{{if not .TimeSeen.IsZero}}{{relativeTime .TimeSeen}}{{end}}</td>
    </tr>
    {{end}}
  </table>
</body>
</html>`))

	requestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: strings.ReplaceAll(appName, "-", "_"),
		Name:      "requests_total",
		Help:      "The total number of processed requests",
	})
	namedClientRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: strings.ReplaceAll(appName, "-", "_"),
		Subsystem: "named_clients",
		Name:      "requests_total",
		Help:      "The total number of processed requests for named clients",
	})
	namedClientUnknownRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: strings.ReplaceAll(appName, "-", "_"),
		Subsystem: "named_clients",
		Name:      "unknown_requests_total",
		Help:      "The total number of unknown requests for named clients",
	})
	requestsTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: strings.ReplaceAll(appName, "-", "_"),
		Name:      "most_recent_request_timestamp_seconds",
		Help:      "Timestamp of the most recent request received",
	})
	version = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: strings.ReplaceAll(appName, "-", "_"),
		Name:      "version",
		Help:      "Version of ip-plz",
	}, []string{"version"})
)

type Conf struct {
	MetricsAddr       string            `env:"IP_PLZ_METRICS_ADDR" json:"metrics_addr"`
	Path              string            `env:"IP_PLZ_PATH" json:"path"`
	Address           string            `env:"IP_PLZ_ADDR" json:"address"`
	TrustedHeaders    []string          `env:"IP_PLZ_TRUSTED_HEADERS" envSeparator:"," json:"trusted_headers"`
	ReadTimeout       int               `env:"IP_PLZ_READ_TIMEOUT" json:"read_timeout"`
	WriteTimeout      int               `env:"IP_PLZ_WRITE_TIMEOUT" json:"write_timeout"`
	IdleTimeout       int               `env:"IP_PLZ_IDLE_TIMEOUT" json:"idle_timeout"`
	ReadHeaderTimeout int               `env:"IP_PLZ_READ_HEADER_TIMEOUT" json:"read_header_timeout"`
	TlsCertFile       string            `env:"IP_PLZ_TLS_CERT_FILE" json:"tls_cert_file"`
	TlsKeyFile        string            `env:"IP_PLZ_TLS_KEY_FILE" json:"tls_key_file"`
	NamedClients      map[string]string `env:"IP_PLZ_NAMED_CLIENTS" json:"named_clients"`
	NamedClientsAddr  string            `env:"IP_PLZ_NAMED_CLIENTS_ADDR" json:"named_clients_addr"`
	NamedClientDbFile string            `env:"IP_PLZ_NAMED_CLIENTS_DB_FILE" json:"named_clients_db_file"`
}

type NamedClient struct {
	IpAddress string    `json:"ip_address"`
	Name      string    `json:"name"`
	TimeSeen  time.Time `json:"time_seen"`
}

func defaultConf() *Conf {
	return &Conf{
		Path:              "/ip-plz",
		Address:           ":8080",
		MetricsAddr:       "127.0.0.1:9191",
		NamedClientsAddr:  "127.0.0.1:9192",
		NamedClientDbFile: "/tmp/ip-plz.db",
		ReadTimeout:       1,
		WriteTimeout:      1,
		IdleTimeout:       5,
		ReadHeaderTimeout: 2,
	}
}

func (c *Conf) loadCert(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if len(c.TlsCertFile) == 0 || len(c.TlsKeyFile) == 0 {
		return nil, errors.New("no client certificates defined")
	}

	certificate, err := tls.LoadX509KeyPair(c.TlsCertFile, c.TlsKeyFile)
	if err != nil {
		slog.Error("user-defined client certificates could not be loaded", "error", err)
	}
	return &certificate, err
}

func ParseConf(configFile string) (*Conf, error) {
	conf := defaultConf()

	if configFile != "" {
		// #nosec G703
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("could not read config file: %s", configFile)
		}

		err = json.Unmarshal(data, &conf)
		if err != nil {
			return nil, fmt.Errorf("could not parse config file: %s", configFile)
		}
	}

	if err := env.Parse(conf); err != nil {
		log.Fatalf("could not parse conf: %v", err)
	}

	return conf, nil
}

type IpPlz struct {
	headers      []string
	namedClients *NamedClientStore
}

func NewIpPlz(trustedHeaders []string, store *NamedClientStore) (*IpPlz, error) {
	return &IpPlz{
		headers:      trustedHeaders,
		namedClients: store,
	}, nil
}

func (b *IpPlz) getIp(req *http.Request) string {
	for _, h := range b.headers {
		for _, ip := range strings.Split(req.Header.Get(h), ",") {
			pubIp, err := GetPublicIp(ip)
			if err == nil {
				return pubIp
			}
		}
	}

	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return host
	}

	return req.RemoteAddr
}

func (b *IpPlz) namedClientsOverview(w http.ResponseWriter, req *http.Request) {
	acceptHeader := req.Header.Get("Accept")
	wantJSON := acceptHeader == "application/json" || acceptHeader == "application/json; charset=utf-8"

	clients := make([]*NamedClient, 0, b.namedClients.Len())
	b.namedClients.Each(func(key string, c NamedClient) {
		clients = append(clients, &c)
	})

	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Name < clients[j].Name
	})

	// JSON response
	if wantJSON {
		type namedClientOutput struct {
			Name      string `json:"name"`
			IpAddress string `json:"ip_address"`
			TimeSeen  string `json:"time_seen"`
			TimeAgo   string `json:"time_ago"`
		}

		outputs := make([]namedClientOutput, 0, len(clients))
		for _, c := range clients {
			outputs = append(outputs, namedClientOutput{
				Name:      c.Name,
				IpAddress: c.IpAddress,
				TimeSeen:  c.TimeSeen.Format(time.RFC3339),
				TimeAgo:   relativeTime(c.TimeSeen),
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(outputs); err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		}
		return
	}

	// HTML response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := namedClientsTemplate.Execute(w, clients); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (b *IpPlz) detectIp(w http.ResponseWriter, req *http.Request) {
	requestsTotal.Inc()
	requestsTimestamp.SetToCurrentTime()
	pubIp := b.getIp(req)

	if pubIp != "" {
		clientId := req.PathValue("id")

		if clientId != "" {
			namedClientRequestsTotal.Inc()
			foundClient := false
			for _, key := range b.namedClients.Keys() {
				if subtle.ConstantTimeCompare([]byte(clientId), []byte(key)) == 1 {
					foundClient = true
					if err := b.namedClients.UpdateIp(key, pubIp); err != nil {
						slog.Error("could not update ip for client", "err", err)
					}
				}
			}

			if !foundClient {
				namedClientUnknownRequestsTotal.Inc()
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := fmt.Fprint(w, pubIp) // #nosec G705
	if err != nil {
		slog.Error("detectIp: error writing to writer", "error", err)
	}
}

func (b *IpPlz) healthcheckHandler(w http.ResponseWriter, req *http.Request) {
	_, err := w.Write([]byte("pong"))
	if err != nil {
		slog.Error("healthcheckHandler: error writing to writer", "error", err)
	}
}

func serveNamedClientsOverview(ctx context.Context, wg *sync.WaitGroup, conf *Conf, ipPlz *IpPlz) {
	slog.Info("Starting server for named clients", "address", conf.NamedClientsAddr)

	wg.Add(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", ipPlz.namedClientsOverview)
	mux.HandleFunc("/health", ipPlz.healthcheckHandler)

	var tlsConfig *tls.Config
	useTls := len(conf.TlsCertFile) > 0 && len(conf.TlsKeyFile) > 0
	if useTls {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
		tlsConfig.GetCertificate = conf.loadCert
		_, err := conf.loadCert(nil)
		if err != nil {
			log.Fatalf("tls keypair was defined but could not be loaded: %v", err)
		}
	}

	server := &http.Server{
		Addr:              conf.NamedClientsAddr,
		Handler:           mux,
		ReadTimeout:       time.Duration(conf.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(conf.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(conf.IdleTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(conf.ReadHeaderTimeout) * time.Second,
		TLSConfig:         tlsConfig,
	}

	errChan := make(chan error)
	go func() {
		if useTls {
			errChan <- server.ListenAndServeTLS("", "")
		} else {
			errChan <- server.ListenAndServe()
		}
	}()

	select {
	case err := <-errChan:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("could not serve metrics: %v", err)
		}
	case <-ctx.Done():
		slog.Info("Shutting down named clients server")
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		_ = server.Shutdown(ctx)
	}

	wg.Done()
}

func serveApp(ctx context.Context, wg *sync.WaitGroup, conf *Conf, ipPlz *IpPlz) {
	slog.Info("Starting ip-plz server", "address", conf.Address, "path", conf.Path, "trusted headers", conf.TrustedHeaders)
	wg.Add(1)

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("GET %s", conf.Path), rateLimitMiddleware(ipPlz.detectIp))
	mux.HandleFunc(fmt.Sprintf("GET %s/{id}", conf.Path), rateLimitMiddleware(ipPlz.detectIp))
	mux.HandleFunc("/health", ipPlz.healthcheckHandler)

	var tlsConfig *tls.Config
	useTls := len(conf.TlsCertFile) > 0 && len(conf.TlsKeyFile) > 0
	if useTls {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
		tlsConfig.GetCertificate = conf.loadCert
		_, err := conf.loadCert(nil)
		if err != nil {
			log.Fatalf("tls keypair was defined but could not be loaded: %v", err)
		}
	}

	server := &http.Server{
		Addr:              conf.Address,
		Handler:           mux,
		ReadTimeout:       time.Duration(conf.ReadTimeout) * time.Second,
		WriteTimeout:      time.Duration(conf.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(conf.IdleTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(conf.ReadHeaderTimeout) * time.Second,
		TLSConfig:         tlsConfig,
	}

	errChan := make(chan error)
	go func() {
		if useTls {
			errChan <- server.ListenAndServeTLS("", "")
		} else {
			errChan <- server.ListenAndServe()
		}
	}()

	select {
	case err := <-errChan:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("could not serve metrics: %v", err)
		}
	case <-ctx.Done():
		slog.Info("Shutting down app server")
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		_ = server.Shutdown(ctx)
	}

	wg.Done()
}

func serveMetrics(ctx context.Context, wg *sync.WaitGroup, conf *Conf) {
	slog.Info("Starting metrics server", "addr", conf.MetricsAddr)
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
		slog.Info("Shutting down metrics server")
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		_ = server.Shutdown(ctx)
	}

	wg.Done()
}

func main() {
	conditionalPrintVersion()
	version.WithLabelValues(BuildVersion).Set(1)

	slog.Info("ip-plz", "version", BuildVersion, "commit", CommitHash)

	configFile := os.Getenv(configFileEnvName)

	conf, err := ParseConf(configFile)
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	if len(conf.MetricsAddr) > 0 {
		go serveMetrics(ctx, wg, conf)
	}

	store := NewNamedClientStore(conf.NamedClientDbFile)

	if err := store.Load(); err != nil {
		log.Fatalf("error loading namedClients: %v", err)
	}

	for id, client := range conf.NamedClients {
		if !store.Has(id) {
			if err := store.Upsert(id, &NamedClient{Name: client}); err != nil {
				log.Fatalf("could not namedClients client %v: %v", id, err)
			}
		}
	}

	ipPlz, err := NewIpPlz(conf.TrustedHeaders, store)
	if err != nil {
		log.Fatalf("error creating ip-plz: %v", err)
	}
	go func() {
		serveApp(ctx, wg, conf, ipPlz)
	}()

	if conf.NamedClientsAddr != "" && len(conf.NamedClients) > 0 {
		go serveNamedClientsOverview(ctx, wg, conf, ipPlz)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	<-done

	slog.Info("Caught signal, quitting gracefully")
	cancel()
	wg.Wait()
	slog.Info("Bye!")
}

func conditionalPrintVersion() {
	version := flag.Bool("version", false, "print version info")
	flag.Parse()
	if *version {
		fmt.Println(BuildVersion)
		os.Exit(0)
	}
}

func relativeTime(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < 2*time.Minute:
		return "1 minute ago"
	case duration < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	case duration < 2*time.Hour:
		return "1 hour ago"
	case duration < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	case duration < 48*time.Hour:
		return "1 day ago"
	default:
		return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
	}
}
