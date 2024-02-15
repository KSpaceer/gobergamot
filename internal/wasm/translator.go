package wasm

import (
	"context"
	"fmt"
	"io"

	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/KSpaceeR/gobergamot/internal/gen"
)

type CompileConfig struct {
	// Stderr and Stdout enable redirection of any logs. If left nil they point at os.Stderr and os.Stdout. Turn off by setting them to io.Discard
	Stderr, Stdout io.Writer
}

func CompileBergamot(
	ctx context.Context,
	wasmRuntime wazero.Runtime,
	embindEng embind.Engine,
	cfg CompileConfig,
) (api.Module, error) {
	bergamotCompiledModule, err := wasmRuntime.CompileModule(ctx, bergamotTranslatorWorkerWASM)
	if err != nil {
		return nil, fmt.Errorf("CompileModule: %w", err)
	}

	if err := BuildImports(ctx, wasmRuntime, embindEng, bergamotCompiledModule); err != nil {
		return nil, fmt.Errorf("BuildImports: %w", err)
	}

	moduleConfig := wazero.NewModuleConfig().
		WithStderr(cfg.Stderr).
		WithStdout(cfg.Stdout).
		WithStartFunctions("_initialize")

	bergamotModule, err := wasmRuntime.InstantiateModule(ctx, bergamotCompiledModule, moduleConfig)
	if err != nil {
		return nil, fmt.Errorf("InstantiateModule: %w", err)
	}

	return bergamotModule, gen.Attach(embindEng)

}
