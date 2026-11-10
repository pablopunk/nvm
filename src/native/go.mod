module github.com/pablopunk/nvm/native

go 1.27.3

require (
	github.com/benedoc-inc/onnxer v0.7.1-0.20260807135226-d3f86f897e42
	github.com/ebitengine/purego v0.11.1
	github.com/egoist/mygo v0.7.4
	github.com/traefik/yaegi v0.16.1
	golang.org/x/sys v0.49.0
)

require (
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

tool github.com/egoist/mygo/cmd/mygo

replace github.com/egoist/mygo => github.com/pablopunk/mygo v0.0.0-20261110123521-f8aacb76a0d6
