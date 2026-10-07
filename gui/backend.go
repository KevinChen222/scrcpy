package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type device struct {
	Serial  string
	Address string
	Name    string
	State   string
	Source  string
	Pairing bool
}

func (d device) key() string {
	if d.Address != "" {
		return d.Address
	}
	return d.Serial
}

type backend struct {
	adb    string
	scrcpy string
	run    func(context.Context, ...string) (string, error)
}

func newBackend() (*backend, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(exe)
	b := &backend{}
	for name, target := range map[string]*string{"adb": &b.adb, "scrcpy": &b.scrcpy} {
		filename := name
		if filepath.Ext(exe) == ".exe" {
			filename += ".exe"
		}
		for _, candidate := range []string{filepath.Join(dir, filename), filepath.Join(dir, "runtime", filename)} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				*target = candidate
				break
			}
		}
		if *target == "" {
			*target, err = exec.LookPath(filename)
			if err != nil {
				return nil, fmt.Errorf("找不到 %s，请完整解压 Release 的便携包后再启动", filename)
			}
		}
	}
	b.run = b.runADB
	return b, nil
}

func (b *backend) runADB(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, b.adb, args...)
	hideConsole(cmd)
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), fmt.Errorf("ADB 操作超时或已取消: %w", ctx.Err())
	}
	if err != nil {
		return string(out), fmt.Errorf("ADB: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

// Android's wireless-debugging and pairing ports differ. Never connect to
// a pairing service as though it were a screen-mirroring transport.
func parseMDNS(out string) []device {
	var devices []device
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.HasPrefix(fields[1], "_adb") {
			continue
		}
		kind := strings.TrimSuffix(fields[1], ".")
		if kind != "_adb-tls-connect._tcp" && kind != "_adb-tls-pairing._tcp" && kind != "_adb._tcp" {
			continue
		}
		address, err := normalizeAddress(fields[2], false)
		if err != nil {
			continue
		}
		pairing := kind == "_adb-tls-pairing._tcp"
		state := "可连接"
		if pairing {
			state = "待配对"
		}
		devices = append(devices, device{Serial: fields[0] + "." + kind, Address: address, Name: fields[0], State: state, Source: "无线调试", Pairing: pairing})
	}
	return devices
}

func parseDevices(out string) []device {
	var devices []device
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(line, "List ") || strings.HasPrefix(line, "*") {
			continue
		}
		d := device{Serial: fields[0], Name: fields[0], State: fields[1], Source: "USB / ADB"}
		if address, err := normalizeAddress(fields[0], false); err == nil {
			d.Address, d.Source = address, "局域网"
		} else if strings.Contains(fields[0], "_adb-tls-connect._tcp") {
			d.Source = "无线调试"
		}
		for _, field := range fields[2:] {
			if model, ok := strings.CutPrefix(field, "model:"); ok {
				d.Name = strings.ReplaceAll(model, "_", " ")
			}
		}
		devices = append(devices, d)
	}
	return devices
}

func mergeDevices(groups ...[]device) []device {
	var result []device
	for _, group := range groups {
		for _, d := range group {
			found := -1
			for i, old := range result {
				if old.key() == d.key() || (old.Serial != "" && d.Serial != "" && strings.TrimSuffix(old.Serial, ".") == strings.TrimSuffix(d.Serial, ".")) {
					found = i
					break
				}
			}
			if found < 0 {
				result = append(result, d)
			} else {
				old := result[found]
				if d.Address == "" {
					d.Address = old.Address
				}
				result[found] = d
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].key() < result[j].key() })
	return result
}

func normalizeAddress(value string, defaultPort bool) (string, error) {
	value = strings.TrimSpace(value)
	if defaultPort {
		if ip, err := netip.ParseAddr(value); err == nil {
			value = net.JoinHostPort(ip.String(), "5555")
		}
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", fmt.Errorf("请输入局域网 IP:端口，例如 192.168.1.10:5555")
	}
	ip, err := netip.ParseAddr(host)
	p, portErr := strconv.Atoi(port)
	if err != nil || (!ip.IsPrivate() && !ip.IsLinkLocalUnicast()) || portErr != nil || p < 1 || p > 65535 {
		return "", fmt.Errorf("需要有效的局域网 IP 和 1–65535 范围内的端口")
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(p)), nil
}

// Limit legacy TCP probing to at most 254 hosts in the local /24 segment.
// Android 11+ random ports are discovered through mDNS instead of port scans.
func localNetworks() []string {
	interfaces, _ := net.Interfaces()
	seen := map[string]bool{}
	var networks []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() {
				continue
			}
			if prefix.Bits() < 24 {
				prefix = netip.PrefixFrom(prefix.Addr(), 24)
			}
			network := prefix.Masked().String()
			if !seen[network] {
				seen[network] = true
				networks = append(networks, network)
			}
		}
	}
	return networks
}

func subnetHosts(network string) ([]string, error) {
	prefix, err := netip.ParsePrefix(network)
	if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() < 24 {
		return nil, fmt.Errorf("扫描范围必须是本地 IPv4 /24 或更小网段")
	}
	prefix = prefix.Masked()
	var hosts []string
	for ip := prefix.Addr().Next(); prefix.Contains(ip); ip = ip.Next() {
		if !prefix.Contains(ip.Next()) {
			break // broadcast
		}
		hosts = append(hosts, net.JoinHostPort(ip.String(), "5555"))
	}
	return hosts, nil
}

func scanLegacy(ctx context.Context, network string) []device {
	hosts, err := subnetHosts(network)
	if err != nil {
		return nil
	}
	jobs := make(chan string, len(hosts))
	for _, address := range hosts {
		jobs <- address
	}
	close(jobs)
	var mu sync.Mutex
	var result []device
	var workers sync.WaitGroup
	for range 32 {
		workers.Go(func() {
			for address := range jobs {
				if ctx.Err() != nil {
					return
				}
				conn, err := (&net.Dialer{Timeout: 250 * time.Millisecond}).DialContext(ctx, "tcp", address)
				if err == nil {
					conn.Close()
					mu.Lock()
					result = append(result, device{Address: address, Name: address, State: "待验证", Source: "5555 端口"})
					mu.Unlock()
				}
			}
		})
	}
	workers.Wait()
	return result
}

func (b *backend) discover(ctx context.Context, network string) ([]device, string, error) {
	if _, err := b.run(ctx, "start-server"); err != nil {
		return nil, "", err
	}
	// Give the ADB server time to receive multicast announcements on first use.
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	out, mdnsErr := b.run(ctx, "mdns", "services")
	mdns := parseMDNS(out)
	var legacy []device
	if network != "" {
		legacy = scanLegacy(ctx, network)
	}
	out, err := b.run(ctx, "devices", "-l")
	if err != nil {
		return nil, "", err
	}
	note := ""
	if mdnsErr != nil {
		note = "mDNS 发现不可用；可用 IP:端口手动连接。"
	}
	return mergeDevices(legacy, mdns, parseDevices(out)), note, nil
}

func (b *backend) connect(ctx context.Context, value string) (string, error) {
	address, err := normalizeAddress(value, true)
	if err != nil {
		return "", err
	}
	out, err := b.run(ctx, "connect", address)
	if err != nil {
		return "", err
	}
	if !strings.Contains(out, "connected to "+address) {
		return "", fmt.Errorf("连接失败: %s；请检查无线调试、配对及手机授权", strings.TrimSpace(out))
	}
	out, err = b.run(ctx, "-s", address, "get-state")
	if err != nil || strings.TrimSpace(out) != "device" {
		return "", fmt.Errorf("设备尚未就绪，请解锁手机并允许调试连接: %s", strings.TrimSpace(out))
	}
	return address, nil
}

func (b *backend) pair(ctx context.Context, value, code string) error {
	address, err := normalizeAddress(value, false)
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		return fmt.Errorf("请输入手机显示的 6 位配对码")
	}
	out, err := b.run(ctx, "pair", address, code)
	if err != nil {
		return err
	}
	if !strings.Contains(out, "Successfully paired") {
		return fmt.Errorf("配对失败: %s", strings.TrimSpace(out))
	}
	return nil
}

// Keep the last diagnostics without growing memory during a long session.
type sessionLog struct {
	mu   sync.Mutex
	data []byte
}

func (l *sessionLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data = append(l.data, p...)
	if len(l.data) > 8192 {
		l.data = l.data[len(l.data)-8192:]
	}
	return len(p), nil
}

func (l *sessionLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.TrimSpace(string(l.data))
}

func (b *backend) start(ctx context.Context, serial string, p preset) (*exec.Cmd, *sessionLog, error) {
	cmd := exec.CommandContext(ctx, b.scrcpy, p.args(serial)...)
	cmd.Dir = filepath.Dir(b.scrcpy)
	cmd.Env = append(os.Environ(), "ADB="+b.adb, "SCRCPY_SERVER_PATH="+filepath.Join(cmd.Dir, "scrcpy-server"))
	hideConsole(cmd)
	log := &sessionLog{}
	cmd.Stdout, cmd.Stderr = log, log
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("启动投屏失败: %w", err)
	}
	return cmd, log, nil
}
