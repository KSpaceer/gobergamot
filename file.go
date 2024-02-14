package gobergamot

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
)

type alignedMemoryFile struct {
	Reader    io.Reader
	Alignment uint
}

type readerWithLen interface {
	io.Reader
	Len() int
}

var _ readerWithLen = (*bytes.Buffer)(nil)

func (f *alignedMemoryFile) size() (uint32, error) {
	var size uint64
	if f.Reader == nil {
		return 0, errors.New("reader is nil")
	}

	switch reader := f.Reader.(type) {
	case readerWithLen:
		size = uint64(reader.Len())
	case io.Seeker:
		streamLen, err := reader.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, err
		}
		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
		size = uint64(streamLen)
	default:
		// We have failed to avoid copying the data into memory to get the size...
		// git commit sudoku
		buf := new(bytes.Buffer)
		bufLen, err := io.Copy(buf, reader)
		if err != nil {
			return 0, err
		}
		// reader's been read, point it at buf now.
		f.Reader = buf
		size = uint64(bufLen)
	}

	if size > math.MaxUint32 {
		return 0, fmt.Errorf("file with size %d too large", size)
	}
	return uint32(size), nil
}

type readerWithBytes interface {
	io.Reader
	Bytes() []byte
}

var _ readerWithBytes = (*bytes.Buffer)(nil)

func (f *alignedMemoryFile) readAll() ([]byte, error) {
	if f.Reader == nil {
		return nil, errors.New("reader is nil")
	}

	var data []byte

	switch reader := f.Reader.(type) {
	case readerWithBytes:
		data = reader.Bytes()
	default:
		buf := new(bytes.Buffer)
		_, err := io.Copy(buf, reader)
		if err != nil {
			return nil, err
		}
		// reader's been read, point it at buf now.
		f.Reader = buf
		data = buf.Bytes()
	}

	return data, nil
}
