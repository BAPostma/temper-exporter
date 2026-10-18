# temper-exporter

A Prometheus exporter for Temper USB temperature sensors.

- `/metrics` exposes two gauges per connected device:
  - `<namespace>_<subsystem>_temperature_celsius{device="<usb port>", vendor_product="<vendor>:<product>", model="<product name>"}`
  - `<namespace>_<subsystem>_temperature_fahrenheit{device="<usb port>", vendor_product="<vendor>:<product>", model="<product name>"}`

  `device` (the USB port, e.g. `1-1.2`) is what distinguishes multiple
  devices from each other. `vendor_product` is the USB vendor:product id
  (e.g. `0c45:7401`) and `model` is the device's USB product name string.
- `/` shows an HTML status page listing every detected device (USB port, vendor:product id, manufacturer, model, firmware, hidraw path) and its current reading.

## How it works

Device discovery and the USB HID read protocol are inspired by [urwen/temper](https://github.com/urwen/temper): it walks `/sys/bus/usb/devices` to find known Temper vendor/product ids, then reads
the temperature directly from `/dev/hidrawN` using raw 8-byte HID reports (no `libusb`/`hidapi`/cgo dependency).

This only supports the hidraw-based Temper variants (TEMPerGold, TEMPer2, TEMPerHUM, TEMPer1F, etc.) and only Linux, since `/sys/bus/usb/devices` and `/dev/hidrawN` are Linux-specific. Temper variants that communicate over a serial TTY instead of hidraw (e.g. TEMPerX232) are not supported. Pull requests welcome.

Devices are rediscovered and re-read on every `/metrics` scrape, so a device that's unplugged or plugged back in is picked up automatically without a restart. When a device shows up that wasn't present on the previous scrape, it's logged.

At startup, every discovered device's hidraw node is checked for read/write access, and a warning is logged if the running user doesn't have sufficient permissions (see [Run](#run) below).

## Build

```sh
make build-local          # builds ./temper_exporter for the host platform
make build-linux-arm64    # cross-compiles for 64-bit Debian (e.g. 64-bit Raspberry Pi OS)
make build-linux-armv6    # cross-compiles for the Raspberry Pi Zero W (32-bit ARMv6)
```

## Run

```sh
./temper_exporter -config config.json
```

The `hidraw` devices are normally root-owned, so this needs to run as root or with a udev rule granting access (e.g. to a `plugdev` group).

## Configuration

[`config.json`](config.json) (path configurable via `-config`):

```json
{
  "port": 9101,
  "metric_namespace": "temperature",
  "metric_subsystem": "temper"
}
```

All fields are optional; missing fields, or a missing file, fall back to
these defaults (`port` 9101, `metric_namespace` "temperature", `metric_subsystem` "temper").

## Dependencies

- [`github.com/prometheus/client_golang`](https://github.com/prometheus/client_golang) — official Prometheus Go client.
- [`golang.org/x/sys`](https://golang.org/x/sys) — Go team's extended syscall package, used for `poll(2)` on the hidraw file descriptor.
