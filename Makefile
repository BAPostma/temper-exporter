BINARY_NAME := temper_exporter

.PHONY: build-local build-linux-arm64 build-linux-armv6 clean

build-local:
	go build -o $(BINARY_NAME) .

# Cross-compile for 64-bit Debian on arm64 (e.g. Raspberry Pi OS 64-bit).
build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(BINARY_NAME) .

# Cross-compile for the Raspberry Pi Zero W (single-core ARMv6, ARM1176JZF-S),
# which only runs 32-bit Raspberry Pi OS.
build-linux-armv6:
	GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -o $(BINARY_NAME) .

clean:
	rm -f $(BINARY_NAME)
