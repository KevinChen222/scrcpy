package main

import (
	"context"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNativeViewSelectionAndPairing(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	a.networks, a.network = []string{"192.168.1.0/24"}, "192.168.1.0/24"
	a.devices = []device{
		{Name: "Pixel 9", Address: "192.168.1.10:37123", State: "device", Source: "无线调试"},
		{Name: "Pixel 配对服务", Address: "192.168.1.10:40123", Pairing: true, State: "待配对", Source: "无线调试"},
	}
	a.selected = a.devices[0].key()
	tt := ui.NewTester(a.view, 1080, 880)
	if err := tt.Click("高清 · 60 fps"); err != nil {
		t.Fatal(err)
	}
	if a.preset != 2 {
		t.Fatal("preset selection did not change")
	}
	if err := tt.Click("Pixel 配对服务"); err != nil {
		t.Fatal(err)
	}
	if a.pairAddress != "192.168.1.10:40123" {
		t.Fatal("pairing address was not populated")
	}
	if !a.showPair || !tt.HasText("配对 Android 设备") {
		t.Fatal("pairing dialog did not open")
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("开始投屏"); err != nil {
		t.Fatal(err)
	}
	if a.busy {
		t.Fatal("pairing transport was offered for mirroring")
	}
	if err := tt.Click("Pixel 9"); err != nil {
		t.Fatal(err)
	}
	if a.selected != "192.168.1.10:37123" {
		t.Fatal("device selection did not change")
	}
	if path := os.Getenv("SCRCPY_GUI_SCREENSHOT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeViewAtMinimumSize(t *testing.T) {
	a := newApplication(context.Background(), &backend{})
	a.networks, a.network = nil, ""
	tt := ui.NewTester(a.view, 940, 720)
	for _, text := range []string{"附近的设备", "流畅 · 30 fps", "原画 · 60 fps", "开始投屏", "准备就绪"} {
		if !tt.HasText(text) {
			t.Errorf("missing UI text %q", text)
		}
	}
}
