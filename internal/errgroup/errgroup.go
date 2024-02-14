package errgroup

import (
	"errors"
	"sync"
)

// Errgroup is similar to golang.org/x/sync/errgroup.Errgroup,
// but this Errgroup does wait for all goroutines to complete
// and returns joined error.
type Errgroup struct {
	wg    sync.WaitGroup
	errCh chan error
}

func New() Errgroup {
	return Errgroup{errCh: make(chan error)}
}

// Go calls given function in a new goroutine.
// Returned error will be a part of error in Wait
func (eg *Errgroup) Go(f func() error) {
	eg.wg.Add(1)
	go func() {
		defer eg.wg.Done()
		eg.errCh <- f()
	}()
}

// Wait waits for completion of all launched goroutines
// and returns composite error formed with errors.Join
func (eg *Errgroup) Wait() error {
	go func() {
		defer close(eg.errCh)
		eg.wg.Wait()
	}()

	var joinedErr error

	for err := range eg.errCh {
		joinedErr = errors.Join(joinedErr, err)
	}
	return joinedErr
}
