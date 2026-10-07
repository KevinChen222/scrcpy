package main

import "fmt"

type preset struct {
	Name    string
	Size    int
	FPS     int
	Bitrate string
	Detail  string
}

var presets = []preset{
	{"流畅 · 30 fps", 1024, 30, "2M", "长边 1024 px · 2 Mbps · 适合较弱 Wi-Fi"},
	{"均衡 · 60 fps", 1600, 60, "6M", "长边 1600 px · 6 Mbps · 日常使用推荐"},
	{"高清 · 60 fps", 1920, 60, "12M", "长边 1920 px · 12 Mbps · 适合稳定的 5 GHz Wi-Fi"},
	{"原画 · 60 fps", 0, 60, "20M", "设备原生分辨率 · 20 Mbps · 适合高速局域网"},
}

func (p preset) args(serial string) []string {
	return []string{
		"--serial=" + serial,
		fmt.Sprintf("--max-size=%d", p.Size),
		fmt.Sprintf("--max-fps=%d", p.FPS),
		"--video-bit-rate=" + p.Bitrate,
		"--video-codec=h264",
		"--window-title=scrcpy LAN · " + serial,
	}
}
