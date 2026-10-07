//go:build !windows

package main

import "context"

func runMirrorShortcuts(ctx context.Context, pid int) error { return nil }
