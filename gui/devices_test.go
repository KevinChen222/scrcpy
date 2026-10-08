package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestPhoneAliasesAndPhysicalIdentity(t *testing.T) {
	mdns := parseMDNS("adb-phone _adb-tls-connect._tcp. 192.168.1.10:34025\nadb-pair _adb-tls-pairing._tcp. 192.168.1.10:40123")
	adb := parseDevices("192.168.1.10:34025 device model:SM_S926U\nadb-phone._adb-tls-connect._tcp. device model:SM_S926U")
	devices := mergeDevices(mdns, adb)
	if len(devices) != 2 {
		t.Fatalf("IP and mDNS aliases duplicated: %+v", devices)
	}
	var phone device
	for _, d := range devices {
		if !d.Pairing {
			phone = d
		}
	}
	if !phone.hasConnection(adb[0].Serial) || !phone.hasConnection(adb[1].Serial) || phone.State != "device" || phone.Source != "无线调试" {
		t.Fatalf("lost connected alias or wireless source: %+v", phone)
	}
	// Resolve two preexisting rows with a later mDNS endpoint bridge.
	if got := mergeDevices(adb, mdns); len(got) != 2 {
		t.Fatalf("late bridge did not merge all records: %+v", got)
	}
	changed := mdns[0]
	changed.Address = "192.168.1.10:35001"
	if got := mergeDevices(mdns[:1], []device{changed}); len(got) != 1 || !got[0].hasConnection(mdns[0].Address) || !got[0].hasConnection(changed.Address) {
		t.Fatalf("service address update discarded old endpoint: %+v", got)
	}
	phone.Identity, phone.DeviceName = "physical-a", "我的三星"
	secondPort := device{Serial: "192.168.1.10:35000", Address: "192.168.1.10:35000", Name: "SM S926U", Identity: "physical-a", State: "device"}
	otherPhone := device{Serial: "192.168.1.11:35000", Address: "192.168.1.11:35000", Name: "SM S926U", DeviceName: "我的三星", Identity: "physical-b", State: "device"}
	unknown := device{Serial: "192.168.1.10:36000", Address: "192.168.1.10:36000", Name: "SM S926U", DeviceName: "我的三星", State: "device"}
	devices = mergeDevices([]device{phone, secondPort, otherPhone, unknown}, mdns)
	if len(devices) != 4 {
		t.Fatalf("same model/name/IP merged without identity, or pairing lost: %+v", devices)
	}
	for _, d := range devices {
		if d.Identity == "physical-a" && (!d.hasConnection(secondPort.Serial) || d.displayName() != "我的三星") {
			t.Fatalf("physical identity did not keep ports and name: %+v", d)
		}
		if d.Pairing && d.hasConnection(secondPort.Serial) {
			t.Fatal("pairing endpoint entered a mirroring record")
		}
	}
	conflicting := secondPort
	conflicting.Identity = "physical-b"
	if got := mergeDevices([]device{secondPort, conflicting}); len(got) != 2 {
		t.Fatalf("conflicting physical identities merged by reused address: %+v", got)
	}
	session := &deviceSession{key: secondPort.key(), serial: secondPort.Serial, device: secondPort}
	if session.matches(conflicting) {
		t.Fatal("session matched another phone at a reused endpoint")
	}
}

func TestPhoneMetadataCacheAndFallback(t *testing.T) {
	for _, failure := range []string{"", "null\r\n", "failure"} {
		t.Run(failure, func(t *testing.T) {
			calls := make(map[string]int)
			b := &backend{run: func(ctx context.Context, args ...string) (string, error) {
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				calls[args[1]+":"+args[3]]++
				if args[3] == "getprop" {
					return "physical-a\n", nil
				}
				if failure == "failure" {
					return "", fmt.Errorf("settings unavailable")
				}
				return failure, nil
			}}
			adb := parseDevices("192.168.1.10:34025 device model:SM_S926U\n192.168.1.10:35000 device model:SM_S926U\nusb-locked unauthorized\n")
			mdns := parseMDNS("adb-phone _adb-tls-connect._tcp. 192.168.1.10:34025")
			devices := b.resolveDevices(context.Background(), mdns, adb)
			devices = b.resolveDevices(context.Background(), mdns, adb)
			if len(devices) != 2 || devices[0].displayName() != "SM S926U" || devices[0].Identity != "physical-a" {
				t.Fatalf("failed device name prevented model fallback or identity: %+v", devices)
			}
			if calls["192.168.1.10:34025:getprop"] != 1 || calls["192.168.1.10:35000:getprop"] != 1 || calls["192.168.1.10:34025:settings"] != 1 || len(calls) != 3 {
				t.Fatalf("metadata was not cached or queried unauthorized phone: %v", calls)
			}
		})
	}
	b := &backend{run: func(_ context.Context, args ...string) (string, error) {
		if args[3] == "getprop" {
			return "physical-a", nil
		}
		return "  我的三星  \r\n", nil
	}}
	mdns := parseMDNS("adb-phone _adb-tls-connect._tcp. 192.168.1.10:34025")
	connected := parseDevices("192.168.1.10:34025 device model:SM_S926U")
	devices := b.resolveDevices(context.Background(), mdns, connected)
	if len(devices) != 1 || devices[0].displayName() != "我的三星" {
		t.Fatalf("phone setting name did not replace model: %+v", devices)
	}
	b.run = func(context.Context, ...string) (string, error) {
		t.Fatal("disconnected discovery queried the phone")
		return "", nil
	}
	devices = b.resolveDevices(context.Background(), mdns)
	if len(devices) != 1 || devices[0].displayName() != "我的三星" || devices[0].key() != "phone:physical-a" {
		t.Fatalf("disconnected alias lost cached name/identity: %+v", devices)
	}
}

func TestRefreshKeepsSelectionAndRunningConnection(t *testing.T) {
	a := newApplication(context.Background(), nil)
	old := device{Serial: "adb-phone._adb-tls-connect._tcp", Address: "192.168.1.10:34025", Name: "SM S926U", State: "device", Source: "无线调试"}
	a.setDevices([]device{old})
	session := &deviceSession{key: old.key(), serial: old.Serial, name: old.Name, device: old}
	a.sessions = []*deviceSession{session}
	confirmed := old
	confirmed.Identity, confirmed.DeviceName = "physical-a", "我的三星"
	a.setDevices([]device{confirmed})
	a.devices = nil
	a.setDevices([]device{old})
	if a.devices[0].Identity != confirmed.Identity || a.selectedSession() != session {
		t.Fatal("unverified refresh downgraded the running phone identity")
	}
	fresh := device{Serial: "192.168.1.10:35000", Address: "192.168.1.10:35000", Name: "SM S926U", Identity: "physical-a", DeviceName: "我的三星", State: "device", Source: "无线调试"}
	a.setDevices([]device{fresh})
	if a.selected != fresh.key() || a.selectedSession() != session || session.serial != old.Serial || session.name != "我的三星" || a.visibleDevices()[0].Serial != old.Serial {
		t.Fatalf("refresh lost selection/session or changed process transport: %+v, %+v", a.devices, session)
	}
	a.sessions = nil
	a.setDevices([]device{fresh})
	if a.visibleDevices()[0].Serial != fresh.Serial {
		t.Fatal("non-running stale alias preferred over connected transport")
	}
	usb := device{Serial: "usb-phone", Name: "SM S926U", Identity: "physical-a", State: "device", Source: "USB 有线"}
	a.setDevices(mergeDevices([]device{usb, fresh}))
	if a.devices[0].Serial != fresh.Serial {
		t.Fatal("connected wireless debugging was not preferred")
	}
	a.setMode(true)
	if a.selected != fresh.key() || a.visibleDevices()[0].Serial != usb.Serial {
		t.Fatal("USB mode lost phone selection or chose a wireless connection")
	}
	for _, size := range [][2]int{{1080, 880}, {840, 600}} {
		a.setMode(false)
		tt := ui.NewTester(a.view, size[0], size[1])
		if !tt.HasText("我的三星") || !tt.HasText(fresh.Address) || tt.HasText(fresh.key()) {
			t.Fatal("card did not show phone name and actual connection address")
		}
	}
}

func TestStartIdentifiesAnotherPortBeforeLaunching(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SCRCPY_GUI_TEST_CHILD", "wait")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := &backend{adb: exe, scrcpy: exe, run: func(_ context.Context, args ...string) (string, error) {
		if args[0] == "connect" {
			return "connected to " + args[1], nil
		}
		if args[2] == "get-state" {
			return "device\n", nil
		}
		if args[3] == "getprop" {
			return "physical-a", nil
		}
		return "", fmt.Errorf("device name unavailable")
	}}
	a := newApplication(ctx, b)
	a.playback.AudioOnly, a.playback.MuteOnStop = true, false
	old := device{Serial: "192.168.1.10:34025", Address: "192.168.1.10:34025", Name: "SM S926U", State: "device", Source: "无线调试"}
	a.setDevices([]device{old})
	applyUpdate := func() {
		t.Helper()
		select {
		case update := <-a.updates:
			update()
		case <-time.After(3 * time.Second):
			t.Fatal("missing session update")
		}
	}
	t.Cleanup(func() { cancel(); a.workers.Wait() })
	a.start()
	applyUpdate()
	if len(a.sessions) != 1 || a.sessions[0].name != "SM S926U" {
		t.Fatalf("first session did not start: %s", a.errorText)
	}
	session := a.sessions[0]
	// This newly discovered port has no known identity until start validates it.
	second := old
	second.Serial, second.Address = "192.168.1.10:35000", "192.168.1.10:35000"
	a.setDevices([]device{second})
	a.start()
	applyUpdate()
	if len(a.sessions) != 1 || a.selectedSession() != session || !strings.Contains(a.status, "已有运行中的会话") || session.serial != old.Serial || session.cmd.ProcessState != nil {
		t.Fatalf("second port duplicated or disturbed running phone: %s, %+v", a.status, a.sessions)
	}
	a.stop(session)
	applyUpdate()
	a.workers.Wait()
}
