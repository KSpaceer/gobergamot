package gobergamot_test

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"io"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"

	"github.com/KSpaceer/gobergamot"
	"github.com/KSpaceer/gobergamot/internal/wasm"
)

const (
	testModelPath     = "testdata/model.enru.intgemm.alphas.bin"
	testShortlistPath = "testdata/lex.50.50.enru.s2t.bin"
	testVocabPath     = "testdata/vocab.enru.spm"
)

func TestTranslator_New(t *testing.T) {
	base64.StdEncoding.EncodeToString()
	ctx := context.Background()
	cache := wazero.NewCompilationCache()
	tests := []struct {
		name    string
		cfg     gobergamot.Config
		wantErr bool
	}{
		{
			name: "no model",
			cfg: gobergamot.Config{
				WASMCache: cache,
			},
			wantErr: true,
		},
		{
			name: "no lexical shortlist",
			cfg: gobergamot.Config{
				FilesBundle: gobergamot.FilesBundle{
					Model:            bytes.NewReader(nil),
					LexicalShortlist: nil,
					Vocabulary:       bytes.NewReader(nil),
				},
				WASMCache: cache,
			},
			wantErr: true,
		},
		{
			name: "no vocabulary",
			cfg: gobergamot.Config{
				FilesBundle: gobergamot.FilesBundle{
					Model:            bytes.NewReader(nil),
					LexicalShortlist: bytes.NewReader(nil),
					Vocabulary:       nil,
				},
				WASMCache: cache,
			},
			wantErr: true,
		},
		{
			name: "invalid files",
			cfg: gobergamot.Config{
				FilesBundle: gobergamot.FilesBundle{
					Model:            bytes.NewReader(nil),
					LexicalShortlist: bytes.NewReader(nil),
					Vocabulary:       bytes.NewReader(nil),
				},
				WASMCache: cache,
			},
			wantErr: true,
		},
		{
			name: "valid",
			cfg: gobergamot.Config{
				FilesBundle: testBundle(t),
				WASMCache:   cache,
			},
			wantErr: false,
		},
		{
			name: "valid with slow paths",
			cfg: gobergamot.Config{
				FilesBundle: strictReaderWrapper(testBundle(t)),
				WASMCache:   cache,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
			tt.cfg.CompileConfig.Stdout = stdout
			tt.cfg.CompileConfig.Stderr = stderr
			translator, err := gobergamot.New(ctx, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"New() error = %v, wantErr %t\n\nstdout: %s\n\nstderr: %s",
					err,
					tt.wantErr,
					stdout.String(),
					stderr.String(),
				)
			}
			if tt.wantErr {
				return
			}
			if err := translator.Close(ctx); err != nil {
				t.Fatalf("Translator.Close() error = %v", err)
			}
		})
	}
}

func TestTranslator_Translate(t *testing.T) {
	ctx := context.Background()

	stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)

	translator, err := gobergamot.New(ctx, gobergamot.Config{
		CompileConfig: wasm.CompileConfig{
			Stderr: stderr,
			Stdout: stdout,
		},
		FilesBundle: testBundle(t),
	})
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}
	defer func() {
		if err := translator.Close(ctx); err != nil {
			t.Fatalf("failed to close translator: %v", err)
		}
	}()

	tests := []struct {
		name         string
		request      gobergamot.TranslationRequest
		wantErr      bool
		wantedOutput string
	}{
		{
			name: "Hello World!",
			request: gobergamot.TranslationRequest{
				Text: "Hello, World!",
			},
			wantedOutput: "Здравствуйте, Мир!",
		},
		{
			name: "error",
			request: gobergamot.TranslationRequest{
				Text: "Yaml: line 2: mapping values are not allowed in this context",
			},
			wantedOutput: "Ямл: строка 2: картирование значений не допускается в этом контексте",
		},
		{
			name: "error 2",
			request: gobergamot.TranslationRequest{
				Text: "Invalid format: invalid regex",
			},
			wantedOutput: "Неверный формат: недействительный regex",
		},
		{
			name: "text",
			request: gobergamot.TranslationRequest{
				Text: "Computers have become an integral part of our daily lives. They have a great impact on the way we live, work, and communicate. Computers have opened up new possibilities. Due to the Internet, students have access to information beyond traditional textbooks. They can conduct research, collaborate with peers on projects, expanding their knowledge horizons. In today’s world, being computer literate is essential for future success. By integrating computers into education, students can learn how to navigate digital tools, analyze and evaluate online information, and develop problem-solving and coding skills. ",
			},
			wantedOutput: "Компьютеры стали неотъемлемой частью нашей повседневной жизни. Они оказывают большое влияние на то, как мы живем, работаем и общаемся. Компьютеры открыли новые возможности. В связи с Интернетом, студенты имеют доступ к информации, помимо традиционных учебников. Они могут проводить исследования, сотрудничать с коллегами по проектам, расширяя горизонты своих знаний. В современном мире быть грамотным компьютером имеет важное значение для будущего успеха. Интегрируя компьютеры в образование, студенты могут узнать, как ориентироваться в цифровых инструментах, анализировать и оценивать онлайн-информацию, а также развивать навыки решения проблем и кодирования. ",
		},
		{
			name: "html hello world",
			request: gobergamot.TranslationRequest{
				Text:    "<a href=\"link.com/path/endpoint?query=parameter\">Hello, World!</a>",
				Options: gobergamot.TranslationOptions{HTML: true},
			},
			wantedOutput: "<a href=\"link.com/path/endpoint?query=parameter\">Здравствуйте, Мир!</a>",
		},
	}

	for _, tt := range tests {
		stdout.Reset()
		stderr.Reset()
		t.Run(tt.name, func(t *testing.T) {
			output, err := translator.Translate(ctx, tt.request)
			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"got error %v, expected wantErr %t\n\nstdout: %s\n\nstderr: %s",
					err,
					tt.wantErr,
					stdout.String(),
					stderr.String(),
				)
			}
			if output != tt.wantedOutput {
				t.Errorf("\nexpected: %s\ngot: %s", tt.wantedOutput, output)
			}
		})
	}
}

func testBundle(t *testing.T) gobergamot.FilesBundle {
	t.Helper()
	return gobergamot.FilesBundle{
		Model:            mustOpenFile(t, testModelPath),
		LexicalShortlist: mustOpenFile(t, testShortlistPath),
		Vocabulary:       mustOpenFile(t, testVocabPath),
	}
}

// wrapping files readers to avoid readers type assertion which is done for fast size/data access.
func strictReaderWrapper(bundle gobergamot.FilesBundle) gobergamot.FilesBundle {
	return gobergamot.FilesBundle{
		Model:            readerWrapper{r: bundle.Model},
		LexicalShortlist: readerWrapper{r: bundle.LexicalShortlist},
		Vocabulary:       readerWrapper{r: bundle.Vocabulary},
	}
}

type readerWrapper struct {
	r io.Reader
}

func (r readerWrapper) Read(p []byte) (n int, err error) {
	return r.r.Read(p)
}

func mustOpenFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		f.Close()
	})
	return f
}
