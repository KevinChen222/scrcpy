//go:build !windows && !linux

package main

import "context"

func runMirrorWindow(ctx context.Context, pid int, playback playbackOptions, serial string, cancel context.CancelFunc) error {
	return nil
}
