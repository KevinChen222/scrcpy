package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type preset struct {
	Name    string
	Size    int
	FPS     int
	Bitrate string
	Detail  string
	USBOnly bool
}

var presets = []preset{
	{"流畅 · 30 fps", 1024, 30, "2M", "长边 1024 px · 2 Mbps · 适合较弱 Wi-Fi", false},
	{"均衡 · 60 fps", 1600, 60, "6M", "长边 1600 px · 6 Mbps · 日常使用推荐", false},
	{"高清 · 60 fps", 1920, 60, "12M", "长边 1920 px · 12 Mbps · 适合稳定的 5 GHz Wi-Fi", false},
	{"原画 · 60 fps", 0, 60, "20M", "设备原生分辨率 · 20 Mbps · 适合高速局域网", false},
	{"有线高规格 · 120 fps", 0, 120, "40M", "设备原生分辨率 · 40 Mbps · 需手机支持高帧率", true},
}

type customPreset struct {
	Size, FPS, BitrateMbps string
}

func (c customPreset) preset() (preset, error) {
	size, err := strconv.Atoi(strings.TrimSpace(c.Size))
	if err != nil || size < 0 || size > 65535 {
		return preset{}, fmt.Errorf("分辨率长边需为 0–65535 的整数，0 表示设备原生分辨率")
	}
	fps, err := strconv.Atoi(strings.TrimSpace(c.FPS))
	if err != nil || fps < 1 || fps > 65535 {
		return preset{}, fmt.Errorf("帧率上限需为 1–65535 的整数")
	}
	bitrate, err := strconv.Atoi(strings.TrimSpace(c.BitrateMbps))
	if err != nil || bitrate < 1 || bitrate > 2147 {
		return preset{}, fmt.Errorf("视频码率需为 1–2147 的整数，单位为 Mbps")
	}
	return preset{Name: "自定义", Size: size, FPS: fps, Bitrate: strconv.Itoa(bitrate) + "M"}, nil
}

type playbackOptions struct {
	BufferMS         int
	Fullscreen       bool
	ScaleMode        string
	Frames           *mirrorFrames
	TurnScreenOff    bool
	MuteOnStop       bool
	KeyboardUHID     bool
	AudioOnly        bool
	AudioCodec       string
	AudioBitrateKbps int
}

func (p playbackOptions) audioCodec() string {
	if p.AudioCodec == "" {
		return "opus"
	}
	return p.AudioCodec
}

func (p playbackOptions) losslessAudio() bool {
	return p.audioCodec() == "flac" || p.audioCodec() == "raw"
}

type audioPreset struct {
	Name        string
	BitrateKbps int
}

var audioPresets = []audioPreset{
	{"省流 · 64 Kbps", 64},
	{"标准 · 128 Kbps", 128},
	{"较高 · 192 Kbps", 192},
	{"高品质 · 256 Kbps", 256},
	{"高码率 · 320 Kbps", 320},
}

func customAudioBitrate(value string) (int, error) {
	bitrate, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || bitrate < 6 || bitrate > 9000 {
		return 0, fmt.Errorf("音频码率需为 6–9000 的整数，单位为 Kbps")
	}
	return bitrate, nil
}

func customBufferMS(value string) (int, error) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	ms := math.Round(seconds * 1000)
	if err != nil || math.IsNaN(seconds) || seconds < 0 || seconds > 60 || math.Abs(seconds*1000-ms) > 0.000001 {
		return 0, fmt.Errorf("缓存时间需为 0–60 秒的数字，最多三位小数；0 关闭额外缓存")
	}
	return int(ms), nil
}

func bufferSeconds(ms int) string {
	return strconv.FormatFloat(float64(ms)/1000, 'f', -1, 64)
}

func (p preset) args(serial string, playback playbackOptions) []string {
	args := []string{"--serial=" + serial}
	if playback.AudioOnly {
		args = append(args, "--no-video", "--no-window", "--require-audio", "--audio-codec="+playback.audioCodec())
		if !playback.losslessAudio() {
			args = append(args, fmt.Sprintf("--audio-bit-rate=%dK", playback.AudioBitrateKbps))
		}
		if !playback.TurnScreenOff {
			args = append(args, "--no-control")
		}
	} else {
		args = append(args,
			fmt.Sprintf("--max-size=%d", p.Size),
			fmt.Sprintf("--max-fps=%d", p.FPS),
			"--video-bit-rate="+p.Bitrate,
			"--video-codec=h264",
			"--window-title=scrcpy LAN · "+serial)
		if playback.KeyboardUHID {
			args = append(args, "--keyboard=uhid")
		}
		if playback.BufferMS > 0 {
			args = append(args, fmt.Sprintf("--video-buffer=%d", playback.BufferMS))
		}
		if playback.Fullscreen && playback.Frames == nil {
			args = append(args, "--fullscreen")
		}
		if playback.Frames != nil {
			args = append(args, "--render-fit=stretched", "--no-window-aspect-ratio-lock")
		} else if playback.ScaleMode == string(scaleStretch) {
			args = append(args, "--render-fit=stretched")
		}
	}
	if playback.BufferMS > 0 {
		args = append(args, fmt.Sprintf("--audio-buffer=%d", playback.BufferMS))
	}
	if playback.TurnScreenOff {
		args = append(args, "--turn-screen-off")
	}
	return args
}
