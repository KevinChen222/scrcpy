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
	scaleManual  scaleMode = "手动选区"
	scaleFill    scaleMode = "裁剪铺满"
	scaleStretch scaleMode = "拉伸铺满"
)

var scaleModes = []string{string(scaleFit), string(scaleFill), string(scaleStretch)}

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
	crop = crop.Intersect(image.Rect(0, 0, frameW, frameH))
	if mode != scaleManual || crop.Empty() {
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

// The selection is made over the complete fitted frame. Convert viewport
// pixels back to decoded-frame coordinates, clipping any surrounding borders.
func selectedMirrorCrop(selection image.Rectangle, rendered scaleRect, frameW, frameH int) image.Rectangle {
	if rendered.W <= 0 || rendered.H <= 0 || frameW <= 0 || frameH <= 0 {
		return image.Rectangle{}
	}
	selection = selection.Intersect(image.Rect(rendered.X, rendered.Y, rendered.X+rendered.W, rendered.Y+rendered.H))
	if selection.Dx() < 8 || selection.Dy() < 8 {
		return image.Rectangle{}
	}
	return image.Rect(
		(selection.Min.X-rendered.X)*frameW/rendered.W,
		(selection.Min.Y-rendered.Y)*frameH/rendered.H,
		((selection.Max.X-rendered.X)*frameW+rendered.W-1)/rendered.W,
		((selection.Max.Y-rendered.Y)*frameH+rendered.H-1)/rendered.H,
	)
}
