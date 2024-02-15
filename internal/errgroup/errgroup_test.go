package errgroup_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/KSpaceeR/gobergamot/internal/errgroup"
)

func TestErrgroup(t *testing.T) {
	tests := []struct {
		name           string
		functions      []func() error
		wantErr        bool
		expectedErrors []error
	}{
		{
			name: "ok single",
			functions: []func() error{func() error {
				return nil
			}},
			wantErr: false,
		},
		{
			name: "ok multiple",
			functions: []func() error{
				func() error { return nil },
				func() error { return nil },
				func() error { return nil },
			},
			wantErr: false,
		},
		{
			name: "err single",
			functions: []func() error{
				func() error { return io.EOF },
			},
			wantErr:        true,
			expectedErrors: []error{io.EOF},
		},
		{
			name: "err multiple",
			functions: []func() error{
				func() error { return io.EOF },
				func() error { return context.Canceled },
			},
			wantErr:        true,
			expectedErrors: []error{io.EOF, context.Canceled},
		},
		{
			name: "single error among correct calls",
			functions: []func() error{
				func() error { return nil },
				func() error { return io.EOF },
				func() error { return nil },
			},
			wantErr:        true,
			expectedErrors: []error{io.EOF},
		},
		{
			name: "multiple errors among correct calls",
			functions: []func() error{
				func() error { return nil },
				func() error { return io.EOF },
				func() error { return nil },
				func() error { return context.Canceled },
				func() error { return nil },
				func() error { return nil },
			},
			wantErr:        true,
			expectedErrors: []error{io.EOF, context.Canceled},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			eg := errgroup.New()
			for i := range tt.functions {
				eg.Go(tt.functions[i])
			}
			err := eg.Wait()
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				for _, expectedErr := range tt.expectedErrors {
					if !errors.Is(err, expectedErr) {
						t.Fatalf("wanted error to contain error %v, but it does not; got err %v", expectedErr, err)
					}
				}
			}
		})
	}
}
