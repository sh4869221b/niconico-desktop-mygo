package main

import _ "embed"

//go:generate magick assets/mygo-app-icon.png -filter Lanczos -resize 256x256 -strip PNG32:assets/window-icon.png
//go:generate magick assets/mygo-app-icon.png -background none -define icon:auto-resize=256,128,64,48,32,24,16 -strip assets/mygo-app.ico
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in assets/windows-resources.json --arch amd64,arm64 --out app_icon

//go:embed assets/window-icon.png
var appIconPNG []byte
