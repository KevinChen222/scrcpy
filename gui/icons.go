package main

import "github.com/egoist/mygo/ui"

// Inline stroke SVG icons rendered natively by mygo at any DPI scale.
var (
	iconBrand = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<rect x="6" y="2" width="12" height="20" rx="2.5"/>` +
			`<path d="M10 18h4"/>` +
			`<path d="M2 8.5v7"/>` +
			`<path d="M22 8.5v7"/>` +
			`</svg>`))

	iconWifi = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<path d="M12 20h.01"/>` +
			`<path d="M2 8.82a15 15 0 0 1 20 0"/>` +
			`<path d="M5 12.859a10 10 0 0 1 14 0"/>` +
			`<path d="M8.5 16.429a5 5 0 0 1 7 0"/>` +
			`</svg>`))

	iconUSB = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<circle cx="10" cy="7" r="1"/>` +
			`<circle cx="4" cy="20" r="1"/>` +
			`<path d="M4.7 19.3 19 5"/>` +
			`<path d="m21 3-3 1 2 2Z"/>` +
			`<path d="M9.26 7.68 5 12l2 5"/>` +
			`<path d="m10 14 5 2 3.5-3.5"/>` +
			`<path d="m18 12 1-1 1 1-1 1Z"/>` +
			`</svg>`))

	iconRefresh = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/>` +
			`<path d="M3 3v5h5"/>` +
			`<path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"/>` +
			`<path d="M16 16h5v5"/>` +
			`</svg>`))

	iconPhone = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<rect width="14" height="20" x="5" y="2" rx="2" ry="2"/>` +
			`<path d="M12 18h.01"/>` +
			`</svg>`))

	iconLink = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>` +
			`<path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>` +
			`</svg>`))

	iconQR = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<rect width="5" height="5" x="3" y="3" rx="1"/>` +
			`<rect width="5" height="5" x="16" y="3" rx="1"/>` +
			`<rect width="5" height="5" x="3" y="16" rx="1"/>` +
			`<path d="M21 16h-3a2 2 0 0 0-2 2v3"/>` +
			`<path d="M21 21v.01"/>` +
			`<path d="M12 7v3a2 2 0 0 1-2 2H7"/>` +
			`<path d="M3 12h.01"/>` +
			`<path d="M12 3h.01"/>` +
			`<path d="M12 16v.01"/>` +
			`<path d="M16 12h1"/>` +
			`<path d="M21 12v.01"/>` +
			`<path d="M12 21v-1"/>` +
			`</svg>`))

	iconMonitor = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<rect width="20" height="14" x="2" y="3" rx="2"/>` +
			`<line x1="8" x2="16" y1="21" y2="21"/>` +
			`<line x1="12" x2="12" y1="17" y2="21"/>` +
			`</svg>`))

	iconAudio = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<path d="M3 14h3a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-7a9 9 0 0 1 18 0v7a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2h3"/>` +
			`</svg>`))

	iconPlay = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">` +
			`<path d="M6 4.75a1.25 1.25 0 0 1 1.9-.1l11.2 7.25a1.25 1.25 0 0 1 0 2.1l-11.2 7.25A1.25 1.25 0 0 1 6 20.2V4.75Z"/>` +
			`</svg>`))

	iconStop = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">` +
			`<rect x="5" y="5" width="14" height="14" rx="2.5"/>` +
			`</svg>`))

	iconTerminal = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<polyline points="4 17 10 11 4 5"/>` +
			`<line x1="12" x2="20" y1="19" y2="19"/>` +
			`</svg>`))

	iconSun = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<circle cx="12" cy="12" r="4"/>` +
			`<path d="M12 2v2"/><path d="M12 20v2"/>` +
			`<path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/>` +
			`<path d="M2 12h2"/><path d="M20 12h2"/>` +
			`<path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/>` +
			`</svg>`))

	iconMoon = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>` +
			`</svg>`))

	iconCopy = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<rect width="14" height="14" x="8" y="8" rx="2" ry="2"/>` +
			`<path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>` +
			`</svg>`))

	iconTrash = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/>` +
			`<path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/>` +
			`</svg>`))

	iconAlert = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			`<circle cx="12" cy="12" r="10"/>` +
			`<line x1="12" x2="12" y1="8" y2="12"/>` +
			`<line x1="12" x2="12.01" y1="16" y2="16"/>` +
			`</svg>`))
)
