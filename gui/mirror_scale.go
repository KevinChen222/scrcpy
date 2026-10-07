package main

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"math"
	"sync"
)

type scaleMode string

const (
	scaleFit     scaleMode = "保持比例"
	scaleAuto    scaleMode = "自适应（识别黑边）"
	scaleFill    scaleMode = "裁剪铺满"
	scaleStretch scaleMode = "拉伸铺满"
)

var scaleModes = []string{string(scaleFit), string(scaleAuto), string(scaleFill), string(scaleStretch)}

// Only frame dimensions are retained, independently of optional session logs.
type mirrorFrames struct {
	mu            sync.Mutex
	line          []byte
	width, height int
}

func (f *mirrorFrames) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range p {
		if b == '\n' {
			if i := bytes.Index(f.line, []byte("Texture")); i >= 0 {
				if colon := bytes.Index(f.line[i:], []byte(": ")); colon >= 0 {
					var w, h int
					if _, err := fmt.Sscanf(string(f.line[i+colon+2:]), "%dx%d", &w, &h); err == nil && w > 0 && h > 0 {
						f.width, f.height = w, h
					}
				}
			}
			f.line = f.line[:0]
		} else if len(f.line) < 2048 {
			f.line = append(f.line, b)
		}
	}
	return len(p), nil
}

func (f *mirrorFrames) size() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.width, f.height
}

var _ io.Writer = (*mirrorFrames)(nil)

type scaleRect struct{ X, Y, W, H int }

// The SDL child renders stretched into this rectangle. Windows clips it to
// the viewport, so SDL maps pointer positions to the original phone correctly.
func scaledMirrorRect(width, height, frameW, frameH int, crop image.Rectangle, mode scaleMode) scaleRect {
	if width <= 0 || height <= 0 || frameW <= 0 || frameH <= 0 {
		return scaleRect{W: max(1, width), H: max(1, height)}
	}
	if mode == scaleStretch {
		return scaleRect{W: width, H: height}
	}
	if mode != scaleAuto || crop.Empty() {
		crop = image.Rect(0, 0, frameW, frameH)
	}
	s := math.Min(float64(width)/float64(crop.Dx()), float64(height)/float64(crop.Dy()))
	if mode == scaleFill {
		s = math.Max(float64(width)/float64(frameW), float64(height)/float64(frameH))
	}
	w, h := int(math.Round(float64(frameW)*s)), int(math.Round(float64(frameH)*s))
	x := int(math.Round((float64(width)-float64(crop.Dx())*s)/2 - float64(crop.Min.X)*s))
	y := int(math.Round((float64(height)-float64(crop.Dy())*s)/2 - float64(crop.Min.Y)*s))
	return scaleRect{x, y, w, h}
}

// Sample whole edge lines instead of a single corner. Allow sparse overlays
// such as a phone's gesture indicator over a black bar. All-black frames and
// very small surviving regions leave the previous framing unchanged.
func blackBarCrop(img image.Image) image.Rectangle {
	b := img.Bounds()
	blackLine := func(horizontal bool, pos int) bool {
		length := b.Dx()
		if !horizontal {
			length = b.Dy()
		}
		black := 0
		for n := 0; n < 160; n++ {
			x, y := b.Min.X+n*(length-1)/159, pos
			if !horizontal {
				x, y = pos, b.Min.Y+n*(length-1)/159
			}
			r, g, blue, _ := img.At(x, y).RGBA()
			if r <= 12*257 && g <= 12*257 && blue <= 12*257 {
				black++
			}
		}
		return black >= 144 // At least 90% near-black; tolerate small edge overlays.
	}
	c := b
	for c.Min.Y < c.Max.Y && blackLine(true, c.Min.Y) {
		c.Min.Y++
	}
	for c.Max.Y > c.Min.Y && blackLine(true, c.Max.Y-1) {
		c.Max.Y--
	}
	for c.Min.X < c.Max.X && blackLine(false, c.Min.X) {
		c.Min.X++
	}
	for c.Max.X > c.Min.X && blackLine(false, c.Max.X-1) {
		c.Max.X--
	}
	if c.Dx() < b.Dx()/3 || c.Dy() < b.Dy()/3 {
		return image.Rectangle{}
	}
	// Ignore a one-pixel encoder edge.
	if c.Min.X-b.Min.X < 2 {
		c.Min.X = b.Min.X
	}
	if b.Max.X-c.Max.X < 2 {
		c.Max.X = b.Max.X
	}
	if c.Min.Y-b.Min.Y < 2 {
		c.Min.Y = b.Min.Y
	}
	if b.Max.Y-c.Max.Y < 2 {
		c.Max.Y = b.Max.Y
	}
	return c
}
