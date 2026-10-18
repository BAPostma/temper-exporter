// Package web renders the exporter's human-facing status page.
package web

import (
	"html/template"
	"net/http"

	"github.com/bapostma/temper-exporter/internal/temper"
)

type deviceView struct {
	Port         string
	VendorID     uint16
	ProductID    uint16
	Manufacturer string
	Product      string
	Firmware     string
	HidrawPath   string
	Celsius      float64
	Fahrenheit   float64
	Error        string
}

type statusView struct {
	Namespace string
	Subsystem string
	Devices   []deviceView
}

var statusTemplate = template.Must(template.New("status").Parse(`<!doctype html>
<html>
<head><title>temper_exporter</title></head>
<body>
<h1>temper_exporter</h1>
<p>Metrics namespace: <code>{{.Namespace}}</code>, subsystem: <code>{{.Subsystem}}</code> &mdash; <a href="/metrics">/metrics</a></p>
{{if .Devices}}
<table border="1" cellpadding="4" cellspacing="0">
<tr>
<th>Port</th><th>Vendor:Product</th><th>Manufacturer</th><th>Model</th>
<th>Firmware</th><th>Device</th><th>Celsius</th><th>Fahrenheit</th>
</tr>
{{range .Devices}}
<tr>
<td>{{.Port}}</td>
<td>{{printf "%04x:%04x" .VendorID .ProductID}}</td>
<td>{{.Manufacturer}}</td>
<td>{{.Product}}</td>
<td>{{.Firmware}}</td>
<td>{{.HidrawPath}}</td>
{{if .Error}}
<td colspan="2">error: {{.Error}}</td>
{{else}}
<td>{{printf "%.2f" .Celsius}}</td>
<td>{{printf "%.2f" .Fahrenheit}}</td>
{{end}}
</tr>
{{end}}
</table>
{{else}}
<p>No Temper devices detected.</p>
{{end}}
</body>
</html>
`))

// Handler returns an http.HandlerFunc that renders a status page listing
// every discovered Temper device along with its current reading.
func Handler(namespace, subsystem string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		devices, err := temper.DiscoverDevices()
		if err != nil {
			http.Error(w, "failed to discover USB devices: "+err.Error(), http.StatusInternalServerError)
			return
		}

		view := statusView{Namespace: namespace, Subsystem: subsystem}
		for _, dev := range devices {
			dv := deviceView{
				Port:         dev.Port,
				VendorID:     dev.VendorID,
				ProductID:    dev.ProductID,
				Manufacturer: dev.Manufacturer,
				Product:      dev.Product,
				HidrawPath:   dev.HidrawPath,
			}

			reading, err := temper.ReadDevice(dev.HidrawPath)
			dv.Firmware = reading.Firmware
			if err != nil {
				dv.Error = err.Error()
			} else {
				dv.Celsius = reading.Celsius
				dv.Fahrenheit = temper.CelsiusToFahrenheit(reading.Celsius)
			}

			view.Devices = append(view.Devices, dv)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := statusTemplate.Execute(w, view); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
