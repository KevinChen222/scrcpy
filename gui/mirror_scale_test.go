package main

import (
	"image"
	"image/color"
	"image/draw"
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
		{scaleAuto, scaleRect{-240, 0, 2400, 1080}},
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
	if got := scaledMirrorRect(1080, 1920, 1080, 2400, image.Rectangle{}, scaleAuto); got != (scaleRect{108, 0, 864, 1920}) {
		t.Fatalf("portrait aspect ratio: %+v", got)
	}
	if got := scaledMirrorRect(1280, 720, 2400, 1080, crop, scaleAuto); got != (scaleRect{-160, 0, 1600, 720}) {
		t.Fatalf("resized viewport: %+v", got)
	}
}

func TestBlackBarDetection(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 240, 140))
	want := image.Rect(24, 16, 216, 124)
	draw.Draw(img, want, &image.Uniform{color.NRGBA{80, 120, 160, 255}}, image.Point{}, draw.Src)
	if got := blackBarCrop(img); got != want {
		t.Fatalf("nested bars: %v", got)
	}
	// A sparse gesture indicator does not hide a real black bar.
	img.SetNRGBA(0, 70, color.NRGBA{255, 255, 255, 255})
	if got := blackBarCrop(img); got != want {
		t.Fatalf("sparse overlay: %v", got)
	}
	// A substantial control at the edge must remain visible.
	for y := 40; y < 90; y++ {
		img.SetNRGBA(0, y, color.NRGBA{255, 255, 255, 255})
	}
	if got := blackBarCrop(img); got.Min.X != 0 {
		t.Fatalf("cropped an edge control: %v", got)
	}
	if got := blackBarCrop(image.NewNRGBA(img.Bounds())); !got.Empty() {
		t.Fatal("all-black scene was treated as bars")
	}
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	if got := blackBarCrop(img); got != img.Bounds() {
		t.Fatal("cropped a borderless image")
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
