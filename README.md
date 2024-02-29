# gobergamot [![Go Report Card](https://goreportcard.com/badge/github.com/KSpaceer/gobergamot)](https://goreportcard.com/report/github.com/KSpaceer/gobergamot) [![Go Reference](https://pkg.go.dev/badge/github.com/KSpaceer/gobergamot.svg)](https://pkg.go.dev/github.com/KSpaceer/gobergamot)

Implementation of local text translator (for i18n) with [Bergamot Translator project](https://github.com/browsermt/bergamot-translator) compiled to WebAssembly with Emscripten via Wazero.

## Usage example

Using single Translator to translate Spanish text to an English one.

```go
modelFile, err := os.Open("model.esen.intgemm.alphas.bin")
handleError(err)
defer modelFile.Close()
shortlistFile, err := os.Open("lex.50.50.esen.s2t.bin")
handleError(err)
defer shortlistFile.Close()
vocabularyFile, err := os.Open("vocab.esen.spm")
handleError(err)
defer vocabularyFile.Close()

cfg := gobergamot.Config{FilesBundle: gobergamot.FilesBundle{modelFile, shortlistFile, vocabularyFile}}
translator, err := gobergamot.New(ctx, cfg)
handleError(err)

englishText, err := translator.Translate(ctx, gobergamot.TranslationRequest{Text: "¡Hola, Mundo!"})
handleError(err)

// Hello, World
fmt.Println(englishText)

// Release resources associated with translator and WASM module
handleError(translator.Close(ctx))
```

Using a pool of Translators for concurrent translating.

```go
cfg := gobergamot.PoolConfig{
  FilesBundle: filesBundle,
  PoolSize: 5,
}

pool, err := gobergamot.NewPool(ctx, cfg)
handleError(err)

translatedText, err := pool.Translate(ctx, gobergamot.TranslationRequest{Text: originalText})
handleError(err)

// releasing pool resources
handleError(pool.Close(ctx))
```

## Installation

Just run following command:

```go get github.com/KSpaceer/gobergamot@latest```

## Where do I find files for models, shortlists and vocabularies?

Files for many languages are available at [Firefox translation models](https://github.com/mozilla/firefox-translations-models).

## How do I recompile WebAssembly Bergamot module?

There is a Makefile target for this - ```make recompile-bergamot```.

## Debug build

```recompile-bergamot``` compiles two versions of WebAssembly binaries - for release and debug. If you need to debug this library, you can put the debug binary into internal/wasm/bergamot-translator-worker.debug.wasm
and run the library with ```gobergamot_debug``` build tag. 

## Gratitudes

Thanks to [Bergamot Project](https://browser.mt/) for awesome idea of local translation.

Thanks to [Danlock](https://github.com/Danlock) for brilliant [Gogosseract project](https://github.com/Danlock/gogosseract) which served as an example of binding C++ projects, WebAssembly and Go.

Thanks to [Jerbob](https://github.com/jerbob92) for great tool [wazero-emscripten-embind](https://github.com/jerbob92/wazero-emscripten-embind) which connects Wazero and Embind.

Thanks to [Tetrate Labs](https://github.com/tetratelabs) for zero-dependency WebAssembly implementation in Go - [Wazero](https://github.com/tetratelabs/wazero).

And, finally, thanks to [Eliah](https://github.com/franchb) for giving me the opportunity to make the project!



