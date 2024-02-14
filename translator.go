package gobergamot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"unsafe"

	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"gopkg.in/yaml.v3"

	"github.com/KSpaceer/gobergamot/internal/errgroup"
	"github.com/KSpaceer/gobergamot/internal/gen"
	"github.com/KSpaceer/gobergamot/internal/wasm"
)

type FilesBundle struct {
	// Byte array of model. Required
	Model io.Reader
	// Byte array of shortlist. Required
	LexicalShortlist io.Reader
	// Byte array of vocabulary to translate between source and target languages. Required
	Vocabulary io.Reader
}

type Config struct {
	wasm.CompileConfig

	// From Bergamot sources:
	//
	// Size in History items to be stored in the cache. A value of 0 means no caching. Loosely corresponds to sentences
	// to cache in the real world. Note that cache has a random-eviction policy. The peak storage at full occupancy is
	// controlled by this parameter. However, whether we attain full occupancy or not is controlled by random factors -
	// specifically how uniformly the hash distributes.
	CacheSize uint

	// Data to load into translator
	FilesBundle

	// From Bergamot sources:
	//
	// Equivalent to options based constructor, where `options` is parsed from string configuration. Configuration can be
	// JSON or YAML. Keys expected correspond to those of `marian-decoder`, available at
	// https://marian-nmt.github.io/docs/cmd/marian-decoder/
	BergamotOptions map[string]any

	WASMCache wazero.CompilationCache

	// WASMUseContext defines if WASM functions execution must be canceled upon context.Context cancellation.
	// Equivalent to wazero.RuntimeConfig WithCloseOnContextDone method parameter.
	WASMUseContext bool
}

var (
	ErrModelMissing            = errors.New("model is required")
	ErrVocabularyMissing       = errors.New("vocabulary is required")
	ErrLexicalShortlistMissing = errors.New("lexical shortlist is required")
)

func (cfg Config) Validate() error {
	var err error
	if cfg.Model == nil {
		err = errors.Join(err, ErrModelMissing)
	}
	if cfg.Vocabulary == nil {
		err = errors.Join(err, ErrVocabularyMissing)
	}
	if cfg.LexicalShortlist == nil {
		err = errors.Join(err, ErrLexicalShortlistMissing)
	}
	return err
}

// DefaultBergamotOptions provides default options for WASM Bergamot translator worker
// like in https://github.com/browsermt/bergamot-translator/blob/v0.4.5/wasm/node-test.js#L66
func DefaultBergamotOptions() map[string]any {
	return map[string]any{
		"beam-size":          uint32(1),
		"normalize":          float64(1.0),
		"word-penalty":       uint32(0),
		"alignment":          "soft",
		"max-length-break":   uint32(128),
		"mini-batch-words":   uint32(1024),
		"workspace":          uint32(128),
		"max-length-factor":  float64(2.0),
		"skip-cost":          true,
		"gemm-precision":     "int8shiftAll",
		"tied-embedding-all": true,
	}
}

// Translator represents a Bergamot translator worker in Go.
type Translator struct {
	embindEngine embind.Engine
	wasmRuntime  wazero.Runtime
	cfg          Config

	model *gen.ClassTranslationModel
	svc   *gen.ClassBlockingService

	module api.Module
}

// New compiles Bergamot module and creates TranslationModel and BlockingService instances
// to be used in Translator
func New(ctx context.Context, cfg Config) (*Translator, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	if cfg.BergamotOptions == nil {
		cfg.BergamotOptions = DefaultBergamotOptions()
	}

	tr := &Translator{
		embindEngine: embind.CreateEngine(embind.NewConfig()),
		cfg:          cfg,
	}

	wasmRuntimeConfig := wazero.NewRuntimeConfig().
		// sentencepiece uses multithreading - so we need WASM threads feature to use Bergamot
		WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesThreads).
		WithCloseOnContextDone(cfg.WASMUseContext)
	if cfg.WASMCache != nil {
		wasmRuntimeConfig = wasmRuntimeConfig.WithCompilationCache(cfg.WASMCache)
	}
	tr.wasmRuntime = wazero.NewRuntimeWithConfig(ctx, wasmRuntimeConfig)

	ctx = tr.embindEngine.Attach(ctx)

	tr.module, err = wasm.CompileBergamot(ctx, tr.wasmRuntime, tr.embindEngine, cfg.CompileConfig)
	if err != nil {
		return nil, fmt.Errorf("CompileBergamot: %w", err)
	}
	bundle, err := enrichAlignedMemoriesBundle(
		ctx,
		tr.embindEngine,
		tr.module,
		newAlignedMemoryDataBundle(cfg.Model, cfg.LexicalShortlist, cfg.Vocabulary),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get aligned memory views: %w", err)
	}

	tr.svc, err = gen.NewClassBlockingService(tr.embindEngine, ctx, map[string]any{"cacheSize": uint32(cfg.CacheSize)})
	if err != nil {
		return nil, fmt.Errorf("failed to get blocking service: %w", err)
	}

	vocabularies, err := gen.NewClassAlignedMemoryList(tr.embindEngine, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create aligned memory list: %w", err)
	}
	if err := vocabularies.Push_back(ctx, bundle[vocabularyIndex].asEmbindClass()); err != nil {
		return nil, fmt.Errorf("failed to push back vocabulary: %w", err)
	}
	bergamotCfg, err := yaml.Marshal(cfg.BergamotOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to convert bergamot (marian) options to YAML: %w", err)
	}
	tr.model, err = gen.NewClassTranslationModel(
		tr.embindEngine,
		ctx,
		string(bergamotCfg),
		bundle[modelIndex].asEmbindClass(),
		bundle[shortlistIndex].asEmbindClass(),
		vocabularies,
		// quality estimation is not used
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create translation model: %w", err)
	}

	return tr, nil
}

// TranslationOptions are equivalent to ResponseOptions in Bergamot.
// From sources:
//
// ResponseOptions dictate how to construct a Response for an input string of
// text to be translated.
type TranslationOptions struct {
	// HTML defines if the Translator should remove HTML tags from text and insert them in output.
	HTML bool
}

type TranslationRequest struct {
	// Text to be translated
	Text string

	// Options for translation
	Options TranslationOptions
}

// Translate translates text provided in the request into a model target language.
func (t *Translator) Translate(ctx context.Context, request TranslationRequest) (string, error) {
	translatedTexts, err := t.TranslateMultiple(ctx, request)
	if err != nil {
		return "", err
	}
	if len(translatedTexts) < 1 {
		return "", fmt.Errorf("expected translated texts to have at least 1 element")
	}
	return translatedTexts[0], nil
}

// TranslateMultiple translates a batch of text provided in the requests into a model target language.
func (t *Translator) TranslateMultiple(ctx context.Context, requests ...TranslationRequest) ([]string, error) {
	input, err := gen.NewClassVectorString(t.embindEngine, ctx)
	if err != nil {
		return nil, err
	}
	defer input.Delete(ctx)
	options, err := gen.NewClassVectorResponseOptions(t.embindEngine, ctx)
	if err != nil {
		return nil, err
	}
	defer options.Delete(ctx)
	if err := convertToInput(ctx, input, options, requests); err != nil {
		return nil, err
	}
	resp, err := t.svc.Translate(ctx, t.model, input, options)
	if err != nil {
		return nil, err
	}
	return processResponse(ctx, resp)
}

// Close deletes created objects and stops the WASM runtime
func (t *Translator) Close(ctx context.Context) error {
	if err := t.model.Delete(ctx); err != nil {
		return err
	}
	if err := t.svc.Delete(ctx); err != nil {
		return err
	}
	return t.wasmRuntime.Close(ctx)
}

func convertToInput(
	ctx context.Context,
	input *gen.ClassVectorString,
	options *gen.ClassVectorResponseOptions,
	requests []TranslationRequest,
) error {
	for i := range requests {
		if err := input.Push_back(ctx, requests[i].Text); err != nil {
			return err
		}

		requestOptions := map[string]any{
			// qualityScores and alignment info is not used, so we are disabling them
			"qualityScores": false,
			"alignment":     false,
			"html":          requests[i].Options.HTML,
		}

		if err := options.Push_back(ctx, requestOptions); err != nil {
			return err
		}
	}
	return nil
}

func processResponse(ctx context.Context, resp embind.ClassBase) ([]string, error) {
	responseVector, ok := resp.(*gen.ClassVectorResponse)
	if !ok {
		return nil, fmt.Errorf("expected response to be a Response vector but got %T", resp)
	}
	defer responseVector.Delete(ctx)
	n, err := responseVector.Size(ctx)
	if err != nil {
		return nil, err
	}
	output := make([]string, 0, n)
	for i := uint32(0); i < n; i++ {
		rawResponse, err := responseVector.Get(ctx, i)
		if err != nil {
			return nil, err
		}
		response, ok := rawResponse.(*gen.ClassResponse)
		if !ok {
			return nil, fmt.Errorf("expected response vector element to be a Response but got %T", rawResponse)
		}
		translatedText, err := response.GetTranslatedText(ctx)
		if err != nil {
			return nil, err
		}
		output = append(output, translatedText)
	}
	return output, nil
}

type alignedMemoryInfo struct {
	file   *alignedMemoryFile
	memory *gen.ClassAlignedMemory
	size   uint32
	view   []int8
}

func (i alignedMemoryInfo) isEmpty() bool {
	return i.file == nil
}

func (i alignedMemoryInfo) asEmbindClass() embind.ClassBase {
	if i.memory == nil {
		return nil
	}
	return i.memory
}

type alignedMemoriesBundle [3]alignedMemoryInfo

const (
	modelIndex = iota
	shortlistIndex
	vocabularyIndex
)

const (
	modelAlignment      = 256
	shortlistAlignment  = 64
	vocabularyAlignment = 64
)

func newAlignedMemoryDataBundle(model, shortlist, vocab io.Reader) alignedMemoriesBundle {
	return alignedMemoriesBundle{
		modelIndex: {file: &alignedMemoryFile{
			Reader:    model,
			Alignment: modelAlignment,
		}},
		shortlistIndex: {file: &alignedMemoryFile{
			Reader:    shortlist,
			Alignment: shortlistAlignment,
		}},
		vocabularyIndex: {file: &alignedMemoryFile{
			Reader:    vocab,
			Alignment: vocabularyAlignment,
		}},
	}
}

func enrichAlignedMemoriesBundle(
	ctx context.Context,
	embindEng embind.Engine,
	mod api.Module,
	bundle alignedMemoriesBundle,
) (alignedMemoriesBundle, error) {
	// getting file sizes concurrently
	bundle, err := enrichBundleWithSizes(ctx, bundle)
	if err != nil {
		return alignedMemoriesBundle{}, err
	}
	// growing module memory to avoid extra allocations
	growModuleMemory(mod, bundle)

	for i := range bundle {
		if bundle[i].isEmpty() {
			continue
		}
		bundle[i].memory, err = gen.NewClassAlignedMemory(embindEng, ctx, bundle[i].size, uint32(bundle[i].file.Alignment))
		if err != nil {
			return alignedMemoriesBundle{}, err
		}
	}

	// after allocations byte views may become invalid,
	// so we're getting them after creating all AlignedMemory instances
	for i := range bundle {
		if bundle[i].isEmpty() {
			continue
		}
		bundle[i].view, err = getAlignedMemoryByteView(ctx, bundle[i].memory)
		if err != nil {
			return alignedMemoriesBundle{}, err
		}
	}

	return fillBundleViews(ctx, bundle)
}

func enrichBundleWithSizes(ctx context.Context, bundle alignedMemoriesBundle) (alignedMemoriesBundle, error) {
	eg := errgroup.New()

	for i := range bundle {
		if bundle[i].isEmpty() {
			continue
		}
		i := i
		eg.Go(func() error {
			size, err := bundle[i].file.size()
			bundle[i].size = size
			return err
		})
	}

	err := eg.Wait()
	return bundle, err
}

const wasmMemoryPageSize = uint32(65536)

func growModuleMemory(mod api.Module, bundle alignedMemoriesBundle) {
	var size uint32
	for i := range bundle {
		if bundle[i].isEmpty() {
			continue
		}
		size += bundle[i].size
	}

	mem := mod.Memory()
	availableSize := mem.Size()
	if availableSize >= size {
		return
	}

	requiredSize := size - availableSize

	pages := requiredSize / wasmMemoryPageSize
	if pages*wasmMemoryPageSize < requiredSize {
		pages += 1
	}
	mem.Grow(pages)
}

func getAlignedMemoryByteView(ctx context.Context, memory *gen.ClassAlignedMemory) ([]int8, error) {
	anyView, err := memory.GetByteArrayView(ctx)
	if err != nil {
		return nil, err
	}
	view, ok := anyView.([]int8)
	if !ok {
		return nil, fmt.Errorf("unexpected type in byte array view %T", anyView)
	}
	return view, nil
}

func fillBundleViews(ctx context.Context, bundle alignedMemoriesBundle) (alignedMemoriesBundle, error) {
	eg := errgroup.New()
	for i := range bundle {
		if bundle[i].isEmpty() {
			continue
		}
		i := i
		eg.Go(func() error {
			return fillByteArrayView(ctx, bundle[i].view, bundle[i].file.Reader, bundle[i].size)
		})
	}
	err := eg.Wait()
	return bundle, err
}

func fillByteArrayView(ctx context.Context, view []int8, input io.Reader, size uint32) error {
	viewBytes := *(*[]byte)(unsafe.Pointer(&view))
	if bytesProvider, ok := input.(readerWithBytes); ok {
		copy(viewBytes, bytesProvider.Bytes())
		return nil
	}

	buf := bytes.NewBuffer(viewBytes)
	buf.Reset()

	written, err := io.Copy(buf, input)
	if err != nil {
		return err
	}
	if int64(size) != written {
		return fmt.Errorf("only wrote %d/%d bytes", written, size)
	}
	return nil
}
