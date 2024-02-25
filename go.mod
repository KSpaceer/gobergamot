module github.com/KSpaceer/gobergamot

go 1.22.0

require (
	github.com/jerbob92/wazero-emscripten-embind v1.5.0
	github.com/tetratelabs/wazero v1.6.1-0.20240212014225-184a6a0d1ec0
	gopkg.in/yaml.v3 v3.0.1
)

require golang.org/x/text v0.14.0 // indirect

retract (
	v0.1.2 // contains only retractions
	v0.1.1 // contains invalid module name
)
