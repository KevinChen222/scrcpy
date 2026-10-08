package main

import (
	"image"
	"strings"
	"testing"
)

func TestMirrorScalingAndPointerMapping(t *testing.T) {
	crop := image.Rect(240, 0, 2160, 1080)
	for _, test := range []struct {
		mode scaleMode
		want scaleRect
	}{
		{scaleFit, scaleRect{0, 108, 1920, 864}},
		{scaleManual, scaleRect{-240, 0, 2400, 1080}},
		{scaleFill, scaleRect{-240, 0, 2400, 1080}},
		{scaleStretch, scaleRect{0, 0, 1920, 1080}},
	} {
		got := scaledMirrorRect(1920, 1080, 2400, 1080, crop, test.mode)
		if got != test.want {
			t.Errorf("%s: %+v, want %+v", test.mode, got, test.want)
		}
		// SDL converts child-relative positions; viewport center must continue
		// to map to the phone center even when the child extends past its parent.
		if (960-got.X)*2400/got.W != 1200 || (540-got.Y)*1080/got.H != 540 {
			t.Errorf("%s: pointer center was displaced", test.mode)
		}
	}
	if got := scaledMirrorRect(1080, 1920, 1080, 2400, image.Rectangle{}, scaleManual); got != (scaleRect{108, 0, 864, 1920}) {
		t.Fatalf("portrait aspect ratio: %+v", got)
	}
	if got := scaledMirrorRect(1280, 720, 2400, 1080, crop, scaleManual); got != (scaleRect{-160, 0, 1600, 720}) {
		t.Fatalf("resized viewport: %+v", got)
	}
}

func TestManualSelectionAndOffCenterPointerMapping(t *testing.T) {
	rendered := scaledMirrorRect(1920, 1080, 2400, 1080, image.Rectangle{}, scaleFit)
	selection := image.Rect(240, 216, 1440, 864)
	crop := selectedMirrorCrop(selection, rendered, 2400, 1080)
	if crop != image.Rect(300, 135, 1800, 945) {
		t.Fatalf("selection was not mapped to decoded frame: %v", crop)
	}
	for _, size := range [][2]int{{1920, 1080}, {1280, 720}, {840, 600}} {
		r := scaledMirrorRect(size[0], size[1], 2400, 1080, crop, scaleManual)
		// The center of an asymmetric selection maps to its own source center,
		// rather than to the full phone center. Test both axes and resized views.
		x, y := (size[0]/2-r.X)*2400/r.W, (size[1]/2-r.Y)*1080/r.H
		if x < 1049 || x > 1051 || y < 539 || y > 541 {
			t.Fatalf("%v: pointer mapped to %d,%d, want 1050,540", size, x, y)
		}
	}
	if got := selectedMirrorCrop(image.Rect(-20, -20, 2000, 1200), rendered, 2400, 1080); got != image.Rect(0, 0, 2400, 1080) {
		t.Fatalf("borders were not clipped: %v", got)
	}
	for _, selection := range []image.Rectangle{image.Rect(0, 0, 100, 100), image.Rect(240, 240, 245, 245)} {
		if got := selectedMirrorCrop(selection, rendered, 2400, 1080); !got.Empty() {
			t.Fatalf("empty/tiny selection accepted: %v", got)
		}
	}
}

func TestFrameSizeUpdatesWithoutLogs(t *testing.T) {
	f := &mirrorFrames{}
	f.Write([]byte("INFO: Tex"))
	f.Write([]byte("ture: 1600x720\n"))
	if w, h := f.size(); w != 1600 || h != 720 {
		t.Fatalf("split size: %dx%d", w, h)
	}
	f.Write([]byte(strings.Repeat("x", 5000) + "\nINFO: Texture: 720x1600\n"))
	if w, h := f.size(); w != 720 || h != 1600 {
		t.Fatalf("rotated size: %dx%d", w, h)
	}
	f.Write([]byte("INFO: Texture (D3D11VA): 592x1280\n"))
	if w, h := f.size(); w != 592 || h != 1280 {
		t.Fatalf("hardware texture: %dx%d", w, h)
	}
	if len(f.line) != 0 || cap(f.line) > 4096 {
		t.Fatal("frame metadata retained unbounded output")
	}
	args := strings.Join(presets[0].args("phone", playbackOptions{Fullscreen: true, Frames: f}), " ")
	if !strings.Contains(args, "--render-fit=stretched") || !strings.Contains(args, "--no-window-aspect-ratio-lock") || strings.Contains(args, "--fullscreen") {
		t.Fatalf("SDL child startup flags: %s", args)
	}
}
