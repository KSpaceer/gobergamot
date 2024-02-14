package wasm

import (
	"context"
	"fmt"

	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func BuildImports(
	ctx context.Context,
	wasmRuntime wazero.Runtime,
	embindEngine embind.Engine,
	compiledModule wazero.CompiledModule,
) error {
	if wasmRuntime.Module("wasi_snapshot_preview1") != nil {
		// If wasi_snapshot_preview1 was already instantiated, the same wazero runtime is being used for multiple Tesseract clients.
		return nil
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, wasmRuntime)

	env := wasmRuntime.NewHostModuleBuilder("env")

	exporter, err := emscripten.NewFunctionExporterForModule(compiledModule)
	if err != nil {
		return fmt.Errorf("emscripten.NewFunctionExporterForModule %w", err)
	}
	exporter.ExportFunctions(env)

	err = embindEngine.NewFunctionExporterForModule(compiledModule).ExportFunctions(env)
	if err != nil {
		return fmt.Errorf("embind ExportFunctions %w", err)
	}

	// Even with -sFILESYSTEM=0 and -sPURE_WASI emscripten imports these syscalls and aborts them in JavaScript.
	// They should never get called, so they panic/no-op if they do.

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, commandPtr int32) int32 {
		// http://pubs.opengroup.org/onlinepubs/000095399/functions/system.html
		// system is used to exec arbitrary commands. Lol, no.

		// emscripten returns a 0 to indicate it's running in the browser without a shell. We shall do the same.
		return 0
	}).Export("system")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module) {
		panic("unimplemented host func")
	}).Export("__cxa_rethrow")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, buf, len int32) int32 {
		panic("unimplemented host func")
	}).Export("__syscall_getcwd")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, dirfd, path, flags int32) int32 {
		panic("unimplemented host func")
	}).Export("__syscall_unlinkat")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, path int32) int32 {
		panic("unimplemented host func")
	}).Export("__syscall_rmdir")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, fd, dirp, count int32) int32 {
		panic("unimplemented host func")
	}).Export("__syscall_getdents64")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, dirfd, path, buf, bufsize int32) int32 {
		panic("unimplemented host func")
	}).Export("__syscall_readlinkat")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, dirfd, path, buf, bufsize int32) int32 {
		panic("unimplemented host func")
	}).Export("__syscall_faccessat")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, mod api.Module, dirfd, path, buf, bufsize int32) int32 {
		panic("unimplemented host func")
	}).Export("__syscall_renameat")

	env.NewFunctionBuilder().WithFunc(func(ctx context.Context, fd int32) int32 {
		panic("unimplemented host func")
	}).Export("pclose")

	_, err = env.Instantiate(ctx)
	if err != nil {
		return err
	}
	return buildGemmModule(ctx, wasmRuntime, embindEngine, compiledModule)
}

// buildGemmModule implements fallback strategy gemm module for Marian.
// https://github.com/browsermt/marian-dev/blob/master/src/tensors/cpu/wasm_intgemm_interface.h
// https://github.com/browsermt/marian-dev/blob/master/wasm/import-gemm-module.js
func buildGemmModule(
	ctx context.Context,
	wasmRuntime wazero.Runtime,
	embindEngine embind.Engine,
	compiledModule wazero.CompiledModule,
) error {
	gemm := wasmRuntime.NewHostModuleBuilder("wasm_gemm")
	exporter, err := emscripten.NewFunctionExporterForModule(compiledModule)
	if err != nil {
		return fmt.Errorf("emscripten.NewFunctionExporterForModule %w", err)
	}
	exporter.ExportFunctions(gemm)

	err = embindEngine.NewFunctionExporterForModule(compiledModule).ExportFunctions(gemm)
	if err != nil {
		return fmt.Errorf("embind ExportFunctions %w", err)
	}

	gemm.NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		mod api.Module,
		inputA int32,
		scale float32,
		zeroPoint float32,
		rowsA uint32,
		width uint32,
		output int32,
	) {
		fn := mod.ExportedFunction("int8PrepareAFallback")
		_, err := fn.Call(ctx, api.EncodeI32(inputA), api.EncodeF32(scale), api.EncodeF32(zeroPoint),
			api.EncodeU32(rowsA), api.EncodeU32(width), api.EncodeI32(output))
		if err != nil {
			panic("failed to call fallback function")
		}
	}).Export("int8_prepare_a")

	gemm.NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		mod api.Module,
		inputB int32,
		scale float32,
		zeroPoint float32,
		width uint32,
		colsB uint32,
		output int32,
	) {
		fn := mod.ExportedFunction("int8PrepareBFallback")
		_, err := fn.Call(ctx, api.EncodeI32(inputB), api.EncodeF32(scale), api.EncodeF32(zeroPoint),
			api.EncodeU32(width), api.EncodeU32(colsB), api.EncodeI32(output))
		if err != nil {
			panic("failed to call fallback function")
		}
	}).Export("int8_prepare_b")

	gemm.NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		mod api.Module,
		inputBTransposed int32,
		scale float32,
		zeroPoint float32,
		width uint32,
		colsB uint32,
		output int32,
	) {
		fn := mod.ExportedFunction("int8PrepareBFromTransposedFallback")
		_, err := fn.Call(ctx, api.EncodeI32(inputBTransposed), api.EncodeF32(scale), api.EncodeF32(zeroPoint),
			api.EncodeU32(width), api.EncodeU32(colsB), api.EncodeI32(output))
		if err != nil {
			panic("failed to call fallback function")
		}
	}).Export("int8_prepare_b_from_transposed")

	gemm.NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		mod api.Module,
		inputBQuantTransposed int32,
		width uint32,
		colsB uint32,
		output int32,
	) {
		fn := mod.ExportedFunction("int8PrepareBFromQuantizedTransposedFallback")
		_, err := fn.Call(ctx, api.EncodeI32(inputBQuantTransposed), api.EncodeU32(width),
			api.EncodeU32(colsB), api.EncodeI32(output))
		if err != nil {
			panic("failed to call fallback function")
		}
	}).Export("int8_prepare_b_from_quantized_transposed")

	gemm.NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		mod api.Module,
		inputBPrepared int32,
		scaleA float32,
		zeroPointA float32,
		scaleB float32,
		zeroPointB float32,
		width uint32,
		colsB uint32,
		inputBias int32,
		output int32,
	) {
		fn := mod.ExportedFunction("int8PrepareBiasFallback")
		_, err := fn.Call(ctx, api.EncodeI32(inputBPrepared),
			api.EncodeF32(scaleA), api.EncodeF32(zeroPointA), api.EncodeF32(scaleB), api.EncodeU32(width),
			api.EncodeU32(width), api.EncodeU32(colsB), api.EncodeI32(inputBias), api.EncodeI32(output))
		if err != nil {
			panic("failed to call fallback function")
		}
	}).Export("int8_prepare_bias")

	gemm.NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		mod api.Module,
		inputAPrepared int32,
		scaleA float32,
		zeroPointA float32,
		inputBPrepared int32,
		scaleB float32,
		zeroPointB float32,
		inputBiasPrepared int32,
		unquantMultiplier float32,
		rowsA uint32,
		width uint32,
		colsB uint32,
		output int32,
	) {
		fn := mod.ExportedFunction("int8MultiplyAndAddBiasFallback")
		_, err := fn.Call(ctx,
			api.EncodeI32(inputAPrepared), api.EncodeF32(scaleA), api.EncodeF32(zeroPointA),
			api.EncodeI32(inputBPrepared), api.EncodeF32(scaleB), api.EncodeF32(zeroPointB),
			api.EncodeI32(inputBiasPrepared), api.EncodeF32(unquantMultiplier),
			api.EncodeU32(rowsA), api.EncodeU32(width), api.EncodeU32(colsB),
			api.EncodeI32(output),
		)
		if err != nil {
			panic("failed to call fallback function")
		}
	}).Export("int8_multiply_and_add_bias")

	gemm.NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		mod api.Module,
		inputBPrepared int32,
		width uint32,
		colsB uint32,
		cols int32,
		numCols uint32,
		output int32,
	) {
		fn := mod.ExportedFunction("int8SelectColumnsOfBFallback")
		_, err := fn.Call(ctx,
			api.EncodeI32(inputBPrepared), api.EncodeU32(width), api.EncodeU32(colsB),
			api.EncodeI32(cols), api.EncodeU32(numCols),
			api.EncodeI32(output),
		)
		if err != nil {
			panic("failed to call fallback function")
		}
	}).Export("int8_select_columns_of_b")

	mmm, err := gemm.Instantiate(ctx)
	_ = mmm
	return err
}
