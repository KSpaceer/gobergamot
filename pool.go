package gobergamot

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/tetratelabs/wazero"

	"github.com/KSpaceer/gobergamot/internal/errgroup"
)

var ErrClosed = errors.New("pool closed")

type PoolConfig struct {
	Config
	PoolSize uint
}

func (cfg PoolConfig) Validate() error {
	var err error
	if cfg.PoolSize == 0 {
		err = errors.Join(err, errors.New("zero pool size"))
	}
	return errors.Join(err, cfg.Config.Validate())
}

// NewPool compiles Translator instances and runs them as workers.
func NewPool(ctx context.Context, cfg PoolConfig) (*Pool, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	if cfg.BergamotOptions == nil {
		cfg.BergamotOptions = DefaultBergamotOptions()
	}
	if cfg.Config.WASMCache == nil {
		// using cache to speed up workers creation
		cfg.Config.WASMCache = wazero.NewCompilationCache()
	}
	p := &Pool{
		cfg:     cfg,
		reqChan: make(chan workerRequest),
		done:    make(chan struct{}),
		eg:      errgroup.New(),
	}
	// converting Config FileBundle into byte slices
	// to share between workers to read
	if err = filesToBytes(p); err != nil {
		return nil, err
	}

	translators, err := p.buildTranslators(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to setup translators: %w", err)
	}

	for i := range translators {
		p.eg.Go(func() error {
			return p.runWorker(translators[i])
		})
	}

	return p, nil
}

type Pool struct {
	cfg PoolConfig

	reqChan chan workerRequest

	eg   errgroup.Errgroup
	done chan struct{}

	modelBytes      []byte
	shortlistBytes  []byte
	vocabularyBytes []byte
}

type workerRequest struct {
	ctx      context.Context
	reqs     []TranslationRequest
	respChan chan workerResponse
}

type workerResponse struct {
	outputs []string
	err     error
}

// Translate is similar to Translator.Translate except the request is asynchronously given
// to any free worker in the pool.
func (p *Pool) Translate(ctx context.Context, request TranslationRequest) (string, error) {
	output, err := p.TranslateMultiple(ctx, request)
	if err != nil {
		return "", err
	}
	if len(output) < 1 {
		return "", fmt.Errorf("expected translated texts to have at least 1 element")
	}
	return output[0], nil
}

// TranslateMultiple is similar to Translator.TranslateMultiple except the requests are asynchronously given
// to any free worker in the pool.
func (p *Pool) TranslateMultiple(ctx context.Context, requests ...TranslationRequest) ([]string, error) {
	req := workerRequest{
		ctx:      ctx,
		reqs:     requests,
		respChan: make(chan workerResponse, 1),
	}
	select {
	case <-p.done:
		return nil, fmt.Errorf("did not found available worker: %w", ErrClosed)
	case <-ctx.Done():
		return nil, fmt.Errorf("did not found available worker: %w", ctx.Err())
	case p.reqChan <- req:
	}

	select {
	case <-p.done:
		return nil, fmt.Errorf("failed to wait response: %w", ErrClosed)
	case <-ctx.Done():
		return nil, fmt.Errorf("failed to wait response: %w", ctx.Err())
	case resp := <-req.respChan:
		return resp.outputs, resp.err
	}
}

// Close closes existing Translator instances and waits for their completion
func (p *Pool) Close(ctx context.Context) error {
	close(p.done)

	errCh := make(chan error)
	go func() {
		errCh <- p.eg.Wait()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (p *Pool) runWorker(translator *Translator) error {
	for {
		select {
		case <-p.done:
			return translator.Close(context.Background())
		case req := <-p.reqChan:
			var resp workerResponse
			resp.outputs, resp.err = translator.TranslateMultiple(req.ctx, req.reqs...)
			req.respChan <- resp
		}
	}
}

func (p *Pool) buildTranslators(ctx context.Context) ([]*Translator, error) {
	eg := errgroup.New()

	translators := make([]*Translator, p.cfg.PoolSize)
	for i := uint(0); i < p.cfg.PoolSize; i++ {
		i := i
		eg.Go(func() error {
			cfg := p.cfg.Config
			cfg.Model = bytes.NewBuffer(p.modelBytes)
			cfg.LexicalShortlist = bytes.NewBuffer(p.shortlistBytes)
			cfg.Vocabulary = bytes.NewBuffer(p.vocabularyBytes)

			translator, err := New(ctx, cfg)
			translators[i] = translator
			return err
		})
	}

	err := eg.Wait()
	return translators, err
}

func filesToBytes(p *Pool) error {
	var err error
	wrappingFile := new(alignedMemoryFile)

	wrappingFile.Reader = p.cfg.Model

	p.modelBytes, err = wrappingFile.readAll()
	if err != nil {
		return fmt.Errorf("failed to read model: %w", err)
	}

	wrappingFile.Reader = p.cfg.LexicalShortlist
	p.shortlistBytes, err = wrappingFile.readAll()
	if err != nil {
		return fmt.Errorf("failed to read shortlist: %w", err)
	}

	wrappingFile.Reader = p.cfg.Vocabulary
	p.vocabularyBytes, err = wrappingFile.readAll()
	if err != nil {
		return fmt.Errorf("failed to read vocabulary: %w", err)
	}

	return nil
}
