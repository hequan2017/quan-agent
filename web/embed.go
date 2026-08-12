package web

import "embed"

// Assets 包含单文件 EXE 所需的前端资源。
//
//go:embed index.html app.js
var Assets embed.FS
