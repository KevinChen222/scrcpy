//go:build !windows

package main

import "context"

func runMirrorWindow(ctx context.Context, pid int, playback playbackOptions, b *backend, serial string, cancel context.CancelFunc) error {
	return nil
}
