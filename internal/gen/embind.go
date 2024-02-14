// because Bergamot uses Marian, which uses SentencePiece, which uses multithreading, it is now hard to generate
// embind code for given module as Wazero does support thread feature only experimentally. If it is needed to
// regenerate code, the Bergamot project can be built with -DUSE_SENTENCE_PIECE=off CMake option.
//
//go:generate go run github.com/jerbob92/wazero-emscripten-embind/generator -v -wasm=../wasm/bergamot-translator-worker.wasm
package gen
