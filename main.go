// Command temper-exporter exposes Celsius and Fahrenheit temperature
// gauges for one or more Temper USB sensors over a Prometheus /metrics
// endpoint.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bapostma/temper-exporter/internal/collector"
	"github.com/bapostma/temper-exporter/internal/config"
	"github.com/bapostma/temper-exporter/internal/temper"
	"github.com/bapostma/temper-exporter/internal/web"
)

func main() {
	configPath := flag.String("config", "config.json", "path to JSON config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	logDiscoveredDevices()

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector.New(cfg.MetricNamespace, cfg.MetricSubsystem))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.Handle("/", web.Handler(cfg.MetricNamespace, cfg.MetricSubsystem))

	addr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("temper-exporter listening on %s (namespace=%q subsystem=%q)", addr, cfg.MetricNamespace, cfg.MetricSubsystem)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// logDiscoveredDevices logs every Temper USB device found at startup, so
// it's clear from the logs alone whether the exporter can see any hardware.
func logDiscoveredDevices() {
	devices, err := temper.DiscoverDevices()
	if err != nil {
		log.Printf("startup: failed to discover USB devices: %v", err)
		return
	}
	if len(devices) == 0 {
		log.Printf("startup: no Temper devices detected")
		return
	}
	for _, dev := range devices {
		log.Printf("startup: found device port=%s vendor:product=%04x:%04x manufacturer=%q product=%q hidraw=%s",
			dev.Port, dev.VendorID, dev.ProductID, dev.Manufacturer, dev.Product, dev.HidrawPath)

		if err := temper.CheckAccess(dev.HidrawPath); err != nil {
			log.Printf("startup: WARNING: insufficient permissions on %s: %v (run as root, or add a udev rule granting access)",
				dev.HidrawPath, err)
		}
	}
}
