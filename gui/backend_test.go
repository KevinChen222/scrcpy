package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// A real child process exercises argument boundaries and cancellation without
// requiring a phone or altering the user's ADB server.
func init() {
	if mode := os.Getenv("SCRCPY_GUI_TEST_CHILD"); mode != "" {
		if path := os.Getenv("SCRCPY_GUI_TEST_ARGS"); path != "" {
			data, _ := json.Marshal(os.Args[1:])
			if err := os.WriteFile(path, data, 0600); err != nil {
				os.Exit(2)
			}
		}
		if mode == "wait" {
			time.Sleep(30 * time.Second)
		}
		fmt.Print("device\n")
		os.Exit(0)
	}
}

func TestDeviceDiscoveryMergesConnectedServices(t *testing.T) {
	mdns := parseMDNS(`List of discovered mdns services
adb-phone _adb-tls-connect._tcp. 192.168.1.10:37123
adb-phone-pair _adb-tls-pairing._tcp 192.168.1.10:40123
adb-old _adb._tcp 192.168.1.20:5555
printer _ipp._tcp 192.168.1.99:631
bad _adb._tcp invalid
`)
	connected := parseDevices(`List of devices attached
adb-phone._adb-tls-connect._tcp device product:panther model:Pixel_7 device:panther transport_id:1
192.168.1.20:5555 unauthorized transport_id:2
usb-123 offline model:Pixel_8
`)
	legacy := []device{{Address: "192.168.1.20:5555"}, {Address: "192.168.1.30:5555"}}
	devices := mergeDevices(legacy, mdns, connected)
	if len(devices) != 5 {
		t.Fatalf("expected one entry per transport (including pairing), got %+v", devices)
	}
	for _, d := range devices {
		if d.Name == "Pixel 7" && (d.Address != "192.168.1.10:37123" || d.State != "device") {
			t.Fatalf("lost mDNS address or connection state: %+v", d)
		}
		if d.Pairing && d.Address != "192.168.1.10:40123" {
			t.Fatalf("pairing transport confused with connect transport: %+v", d)
		}
	}
	if devices[2].Address != "192.168.1.20:5555" || devices[3].Address != "192.168.1.30:5555" {
		t.Fatalf("distinct legacy candidates were merged: %+v", devices)
	}
}

func TestAddressesAndBoundedScan(t *testing.T) {
	for value, want := range map[string]string{
		" 192.168.1.10 ": "192.168.1.10:5555",
		"10.0.0.2:37000": "10.0.0.2:37000",
		"[fd00::2]:1234": "[fd00::2]:1234",
	} {
		got, err := normalizeAddress(value, true)
		if err != nil || got != want {
			t.Errorf("%q: %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"", "example.com:5555", "8.8.8.8:5555", "192.168.1.2:0", "192.168.1.2:65536", "192.168.1.2:5555;echo"} {
		if _, err := normalizeAddress(value, true); err == nil {
			t.Errorf("accepted invalid LAN address %q", value)
		}
	}
	hosts, err := subnetHosts("192.168.1.42/24")
	if err != nil || len(hosts) != 254 || hosts[0] != "192.168.1.1:5555" || hosts[253] != "192.168.1.254:5555" {
		t.Fatalf("incorrect scan bounds: %d hosts, %v", len(hosts), err)
	}
	if _, err := subnetHosts("10.0.0.0/8"); err == nil {
		t.Fatal("accepted unbounded subnet")
	}
}

func TestConnectRequiresReadyDevice(t *testing.T) {
	var calls [][]string
	b := &backend{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		if args[0] == "connect" {
			return "already connected to 192.168.1.10:5555\n", nil
		}
		return "device\r\n", nil
	}}
	serial, err := b.connect(context.Background(), "192.168.1.10")
	if err != nil || serial != "192.168.1.10:5555" || len(calls) != 2 {
		t.Fatalf("connect: %q, %v, calls %v", serial, err, calls)
	}
	b.run = func(_ context.Context, args ...string) (string, error) { return "failed to connect: refused", nil }
	if _, err := b.connect(context.Background(), serial); err == nil {
		t.Fatal("accepted adb failure with zero exit status")
	}
	b.run = func(_ context.Context, args ...string) (string, error) {
		if args[0] == "connect" {
			return "connected to " + serial, nil
		}
		return "unauthorized", nil
	}
	if _, err := b.connect(context.Background(), serial); err == nil {
		t.Fatal("accepted unauthorized transport")
	}
}

func TestPairValidationAndFailure(t *testing.T) {
	var calls [][]string
	b := &backend{run: func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		return "Successfully paired to 192.168.1.10:40000", nil
	}}
	for _, code := range []string{"", "12345", "1234567", "12x456"} {
		if err := b.pair(context.Background(), "192.168.1.10:40000", code); err == nil {
			t.Fatalf("accepted invalid code %q", code)
		}
	}
	if len(calls) != 0 {
		t.Fatal("invalid input launched adb")
	}
	if err := b.pair(context.Background(), "192.168.1.10:40000", "012345"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls[0], []string{"pair", "192.168.1.10:40000", "012345"}) {
		t.Fatalf("pair args: %v", calls)
	}
	b.run = func(_ context.Context, _ ...string) (string, error) { return "Failed: wrong password", nil }
	if err := b.pair(context.Background(), "192.168.1.10:40000", "123456"); err == nil {
		t.Fatal("accepted rejected pairing")
	}
}

func TestChildProcessArgumentsAndStop(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "args.json")
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "exit")
	t.Setenv("SCRCPY_GUI_TEST_ARGS", file)
	b := &backend{adb: exe, scrcpy: exe}
	out, err := b.runADB(context.Background(), "-s", "192.168.1.10:5555", "get-state")
	if err != nil || out != "device\n" {
		t.Fatalf("adb subprocess: %q %v", out, err)
	}
	cmd, _, err := b.start(context.Background(), "phone with spaces", presets[1])
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, presets[1].args("phone with spaces")) {
		t.Fatalf("argument boundaries changed: %v", args)
	}
	for _, arg := range []string{"--max-size=1600", "--max-fps=60", "--video-bit-rate=6M", "--video-codec=h264"} {
		if !strings.Contains(string(data), arg) {
			t.Errorf("balanced preset missing %s", arg)
		}
	}
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "wait")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd, _, err = b.start(ctx, "192.168.1.10:5555", presets[0])
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled mirror exited successfully")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("mirror did not stop after cancellation")
	}
}
