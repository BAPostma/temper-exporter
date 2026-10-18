// Package collector implements a Prometheus collector that reads Temper USB
// devices at scrape time.
package collector

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bapostma/temper-exporter/internal/temper"
)

// TemperCollector discovers Temper USB devices and reports their internal
// temperature, in Celsius and Fahrenheit, on every scrape.
type TemperCollector struct {
	celsiusDesc    *prometheus.Desc
	fahrenheitDesc *prometheus.Desc

	mu        sync.Mutex
	seenPorts map[string]bool // ports seen on the previous scrape, to log newly-appeared devices
}

// New creates a TemperCollector whose metrics are named
// "<namespace>_<subsystem>_temperature_celsius" and
// "<namespace>_<subsystem>_temperature_fahrenheit".
func New(namespace, subsystem string) *TemperCollector {
	labels := []string{"device", "vendor_product", "model"}
	return &TemperCollector{
		celsiusDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "temperature_celsius"),
			"Temper device temperature in degrees Celsius.",
			labels, nil,
		),
		fahrenheitDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "temperature_fahrenheit"),
			"Temper device temperature in degrees Fahrenheit.",
			labels, nil,
		),
		seenPorts: make(map[string]bool),
	}
}

func (c *TemperCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.celsiusDesc
	ch <- c.fahrenheitDesc
}

func (c *TemperCollector) Collect(ch chan<- prometheus.Metric) {
	devices, err := temper.DiscoverDevices()
	if err != nil {
		log.Printf("temper: failed to discover USB devices: %v", err)
		return
	}

	currentPorts := make(map[string]bool, len(devices))
	for _, dev := range devices {
		currentPorts[dev.Port] = true

		c.mu.Lock()
		isNew := !c.seenPorts[dev.Port]
		c.mu.Unlock()
		if isNew {
			log.Printf("temper: new device detected port=%s vendor:product=%04x:%04x model=%q hidraw=%s",
				dev.Port, dev.VendorID, dev.ProductID, dev.Product, dev.HidrawPath)
		}

		reading, err := temper.ReadDevice(dev.HidrawPath)
		now := time.Now()
		if err != nil {
			log.Printf("temper: failed to read %s (port %s): %v", dev.HidrawPath, dev.Port, err)
			continue
		}

		vendorProduct := fmt.Sprintf("%04x:%04x", dev.VendorID, dev.ProductID)

		celsiusMetric := prometheus.MustNewConstMetric(c.celsiusDesc, prometheus.GaugeValue, reading.Celsius, dev.Port, vendorProduct, dev.Product)
		fahrenheitMetric := prometheus.MustNewConstMetric(c.fahrenheitDesc, prometheus.GaugeValue, temper.CelsiusToFahrenheit(reading.Celsius), dev.Port, vendorProduct, dev.Product)

		ch <- prometheus.NewMetricWithTimestamp(now, celsiusMetric)
		ch <- prometheus.NewMetricWithTimestamp(now, fahrenheitMetric)
	}

	c.mu.Lock()
	c.seenPorts = currentPorts
	c.mu.Unlock()
}
