package studio

import "embed"

//go:embed all:frontend/dist
var uiFiles embed.FS
