package gobergamot_test

import (
	"context"
	"testing"
	"time"

	"github.com/KSpaceer/gobergamot"
)

func BenchmarkSingleSentence(b *testing.B) {
	b.StopTimer()

	startCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	translator, err := gobergamot.New(startCtx, gobergamot.Config{
		FilesBundle: testBundle(nil),
	})
	if err != nil {
		b.Fatalf("failed to create translator: %v", err)
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		translator.Translate(context.Background(), gobergamot.TranslationRequest{
			Text: "Hello, World!",
		})
	}
}
