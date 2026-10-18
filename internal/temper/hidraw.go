package temper

import (
	"encoding/binary"
	"fmt"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	firmwareQueryRetries = 10
	firmwarePollMillis   = 200
	dataPollMillis       = 100
)

var firmwareQuery = []byte{0x01, 0x86, 0xff, 0x01, 0, 0, 0, 0}
var dataQuery = []byte{0x01, 0x80, 0x33, 0x01, 0, 0, 0, 0}

// Reading holds the result of querying a Temper device.
type Reading struct {
	Firmware string
	Celsius  float64
}

// ReadDevice opens the given hidraw device, queries its firmware identifier
// and sensor data using the device's raw 8-byte USB HID report protocol,
// and returns the device's firmware identifier and temperature in Celsius.
func ReadDevice(path string) (Reading, error) {
	fd, err := unix.Open(path, unix.O_RDWR, 0)
	if err != nil {
		return Reading{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer unix.Close(fd)

	firmwareBytes, err := queryFirmware(fd)
	if err != nil {
		return Reading{}, fmt.Errorf("querying firmware on %s: %w", path, err)
	}
	firmware := strings.TrimSpace(string(firmwareBytes))

	data, err := queryData(fd)
	if err != nil {
		return Reading{Firmware: firmware}, fmt.Errorf("querying data on %s: %w", path, err)
	}

	celsius, err := parseTemperature(firmware, data)
	if err != nil {
		return Reading{Firmware: firmware}, err
	}

	return Reading{Firmware: firmware, Celsius: celsius}, nil
}

// CelsiusToFahrenheit converts a Celsius temperature to Fahrenheit.
func CelsiusToFahrenheit(c float64) float64 {
	return c*1.8 + 32.0
}

// CheckAccess reports whether the current process has read and write
// permission on the given hidraw device, without opening it. hidraw devices
// are normally root-owned, so this is used to warn early (e.g. at startup)
// when the process doesn't have the access it'll need to read the device.
func CheckAccess(path string) error {
	return unix.Access(path, unix.R_OK|unix.W_OK)
}

// queryFirmware sends the firmware identification command and collects the
// reply, retrying a handful of times since devices sometimes answer with a
// short or empty reply.
func queryFirmware(fd int) ([]byte, error) {
	var firmware []byte
	for i := 0; i < firmwareQueryRetries; i++ {
		if _, err := unix.Write(fd, firmwareQuery); err != nil {
			return nil, fmt.Errorf("writing firmware query: %w", err)
		}

		firmware = nil
		for {
			ready, err := pollReadable(fd, firmwarePollMillis)
			if err != nil {
				return nil, fmt.Errorf("polling for firmware reply: %w", err)
			}
			if !ready {
				break
			}
			buf := make([]byte, 8)
			n, err := unix.Read(fd, buf)
			if err != nil {
				return nil, fmt.Errorf("reading firmware reply: %w", err)
			}
			if n == 0 {
				break
			}
			firmware = append(firmware, buf[:n]...)
		}

		if len(firmware) == 0 {
			return nil, fmt.Errorf("no response reading device firmware identifier")
		}
		if len(firmware) > 8 {
			break
		}
	}
	return firmware, nil
}

// queryData sends the temperature/humidity read command and collects the
// reply.
func queryData(fd int) ([]byte, error) {
	if _, err := unix.Write(fd, dataQuery); err != nil {
		return nil, fmt.Errorf("writing data query: %w", err)
	}

	var data []byte
	for {
		ready, err := pollReadable(fd, dataPollMillis)
		if err != nil {
			return nil, fmt.Errorf("polling for data reply: %w", err)
		}
		if !ready {
			break
		}
		buf := make([]byte, 8)
		n, err := unix.Read(fd, buf)
		if err != nil {
			return nil, fmt.Errorf("reading data reply: %w", err)
		}
		if n == 0 {
			break
		}
		data = append(data, buf[:n]...)
	}
	return data, nil
}

// pollReadable waits up to timeoutMillis for fd to become readable.
func pollReadable(fd int, timeoutMillis int) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, timeoutMillis)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return false, err
		}
		return n > 0 && fds[0].Revents&unix.POLLIN != 0, nil
	}
}

// parseTemperature identifies the device firmware and decodes its
// temperature from the raw data reply, using the byte layout specific to
// that firmware version.
func parseTemperature(fw string, data []byte) (float64, error) {
	switch {
	case hasPrefix(fw, 10, "TEMPerF1.2", "TEMPerF1.4", "TEMPer1F1."):
		return readInt16(data, 2, 256.0)

	case hasPrefix(fw, 15, "TEMPerGold_V3.1", "TEMPerGold_V3.3", "TEMPerGold_V3.4", "TEMPerGold_V3.5"):
		return readInt16(data, 2, 100.0)

	case hasPrefix(fw, 12, "TEMPerX_V3.1", "TEMPerX_V3.3"):
		return readInt16(data, 2, 100.0)

	case hasPrefix(fw, 16, "TEMPer2_M12_V1.3"):
		return readInt16(data, 2, 256.0)

	case hasPrefix(fw, 12, "TEMPer2_V3.7", "TEMPer2_V3.9"):
		return readInt16(data, 2, 100.0)

	case hasPrefix(fw, 14, "TEMPerHUM_V3.9"):
		return readInt16(data, 2, 100.0)

	case hasPrefix(fw, 16, "TEMPer1F_H1V1.5F"):
		// SHT20 sensor: the raw register value is shifted and run through
		// the SHT20 datasheet equation rather than a simple divisor.
		raw, err := readInt16(data, 2, 1.0)
		if err != nil {
			return 0, err
		}
		t := int64(raw) << 2
		return -46.85 + 175.72*float64(t)/65536.0, nil

	case hasPrefix(fw, 12, "TEMPer2_V4.1"):
		return readInt16(data, 2, 100.0)

	case hasPrefix(fw, 13, "TEMPer1F_V3.9", "TEMPer1F_V4.1"):
		return readInt16(data, 2, 100.0)
	}

	return 0, fmt.Errorf("unknown firmware %q", fw)
}

// hasPrefix reports whether fw, truncated to prefixLen, matches one of want.
func hasPrefix(fw string, prefixLen int, want ...string) bool {
	if len(fw) < prefixLen {
		return false
	}
	prefix := fw[:prefixLen]
	for _, w := range want {
		if prefix == w {
			return true
		}
	}
	return false
}

// readInt16 decodes a big-endian signed 16-bit value at offset in data and
// divides it by divisor. It returns an error if the sensor is absent (the
// device reports 0x4e 0x20 at the offset) or the reply is too short.
func readInt16(data []byte, offset int, divisor float64) (float64, error) {
	if offset+1 >= len(data) {
		return 0, fmt.Errorf("sensor reply too short (got %d bytes, need offset %d)", len(data), offset)
	}
	if data[offset] == 0x4e && data[offset+1] == 0x20 {
		return 0, fmt.Errorf("no sensor present at offset %d", offset)
	}
	v := int16(binary.BigEndian.Uint16(data[offset : offset+2]))
	return float64(v) / divisor, nil
}
