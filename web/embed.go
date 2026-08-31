// Package web 打包前端资源。
package web

import "embed"

//go:embed *.html *.js *.css *.svg
var FS embed.FS
