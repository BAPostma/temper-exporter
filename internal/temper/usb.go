package temper

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// sysUSBPath is the Linux sysfs hierarchy that exposes USB device metadata.
const sysUSBPath = "/sys/bus/usb/devices"

var hidrawNameRE = regexp.MustCompile(`^hidraw[0-9]+$`)

// usbID identifies a USB device by its vendor and product id.
type usbID struct {
	vendor  uint16
	product uint16
}

// knownIDs lists the vendor/product pairs of Temper devices that expose a
// hidraw interface. The one Temper variant that only exposes a serial (tty)
// interface (vendor 0x1a86, product 0x5523) is intentionally excluded, since
// this exporter only speaks the hidraw protocol.
var knownIDs = map[usbID]bool{
	{0x0c45, 0x7401}: true,
	{0x0c45, 0x7402}: true,
	{0x413d, 0x2107}: true,
	{0x1a86, 0xe025}: true,
	{0x3553, 0xa001}: true,
}

// Device describes a discovered Temper USB device and the hidraw node used
// to talk to it.
type Device struct {
	Port         string // sysfs device name, e.g. "1-1.2"
	VendorID     uint16
	ProductID    uint16
	Manufacturer string
	Product      string
	BusNum       int
	DevNum       int
	HidrawPath   string // e.g. "/dev/hidraw0"
}

// DiscoverDevices scans the sysfs USB hierarchy for known Temper devices and
// returns one Device per discovered unit that has a usable hidraw node.
// Devices with no matching vendor/product id, or with no hidraw node, are
// silently skipped (returns an empty slice if the sysfs path is absent,
// e.g. when running on a non-Linux platform or with nothing plugged in).
func DiscoverDevices() ([]Device, error) {
	entries, err := os.ReadDir(sysUSBPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var devices []Device
	for _, entry := range entries {
		dir := filepath.Join(sysUSBPath, entry.Name())

		// Entries under /sys/bus/usb/devices are symlinks to the real device
		// directories elsewhere in sysfs, so entry.IsDir() (which reflects
		// the symlink itself, not its target) is always false here. Stat
		// follows the symlink to check the target is actually a directory.
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}

		vendor, ok := readHexUint16(filepath.Join(dir, "idVendor"))
		if !ok {
			continue
		}
		product, ok := readHexUint16(filepath.Join(dir, "idProduct"))
		if !ok {
			continue
		}
		if !knownIDs[usbID{vendor, product}] {
			continue
		}

		hidraws := findHidrawDevices(dir)
		if len(hidraws) == 0 {
			continue
		}
		sort.Strings(hidraws)
		hidraw := hidraws[len(hidraws)-1]

		busnum, _ := strconv.Atoi(readFile(filepath.Join(dir, "busnum")))
		devnum, _ := strconv.Atoi(readFile(filepath.Join(dir, "devnum")))

		devices = append(devices, Device{
			Port:         entry.Name(),
			VendorID:     vendor,
			ProductID:    product,
			Manufacturer: readFile(filepath.Join(dir, "manufacturer")),
			Product:      readFile(filepath.Join(dir, "product")),
			BusNum:       busnum,
			DevNum:       devnum,
			HidrawPath:   filepath.Join("/dev", hidraw),
		})
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].BusNum*1000+devices[i].DevNum <
			devices[j].BusNum*1000+devices[j].DevNum
	})

	return devices, nil
}

// findHidrawDevices recursively searches dir for hidraw device names,
// mirroring the sysfs walk done by the Python reference implementation.
func findHidrawDevices(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var found []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			found = append(found, findHidrawDevices(filepath.Join(dir, entry.Name()))...)
		}
		if hidrawNameRE.MatchString(entry.Name()) {
			found = append(found, entry.Name())
		}
	}
	return found
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readHexUint16(path string) (uint16, bool) {
	s := readFile(path)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(v), true
}
