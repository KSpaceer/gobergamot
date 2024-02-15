package gobergamot_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/KSpaceeR/gobergamot"
	"github.com/KSpaceeR/gobergamot/internal/wasm"
)

const (
	helloWorldTranslation   = "Здравствуйте Мир"
	goodbyeWorldTranslation = "Прощание с миром"
)

func TestPool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	_, err := gobergamot.NewPool(ctx, gobergamot.PoolConfig{
		Config: gobergamot.Config{
			FilesBundle: gobergamot.FilesBundle{
				Model:            bytes.NewBuffer([]byte{}),
				LexicalShortlist: bytes.NewBuffer([]byte{}),
				Vocabulary:       bytes.NewBuffer([]byte{}),
			},
		},
		PoolSize: 3,
	})
	if err == nil {
		t.Fatalf("NewPool should have failed")
	}

	stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)

	pool, err := gobergamot.NewPool(ctx, gobergamot.PoolConfig{
		Config: gobergamot.Config{
			CompileConfig: wasm.CompileConfig{
				Stderr: stderr,
				Stdout: stdout,
			},
			CacheSize:   100,
			FilesBundle: testBundle(t),
		},
		PoolSize: 3,
	})
	if err != nil {
		t.Fatalf("NewPool returned error %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(ctx); err != nil {
			t.Fatalf("failed to close pool: %v", err)
		}
	})

	requests := [...]gobergamot.TranslationRequest{
		{Text: "Hello World"},
		{Text: "Hello World"},
		{Text: "Hello World"},
		{Text: "Hello World"},
		{Text: "Hello World"},
		{Text: "Goodbye World"},
		{Text: "Goodbye World"},
		{Text: "Goodbye World"},
		{Text: "Goodbye World"},
		{Text: "Goodbye World"},
	}

	textChan := make(chan string, len(requests))

	for _, req := range requests {
		go func(req gobergamot.TranslationRequest) {
			output, err := pool.Translate(ctx, req)
			if err != nil {
				panic(err)
			}
			textChan <- output
		}(req)
	}

	for range requests {
		select {
		case <-ctx.Done():
			t.Fatal("context timeout waiting for responses")
		case output := <-textChan:
			if output != helloWorldTranslation && output != goodbyeWorldTranslation {
				t.Errorf("unexpected output %s", output)
			}
		}
	}
}
