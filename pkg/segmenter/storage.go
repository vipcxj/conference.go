package segmenter

import (
	"io"
	"os"

	"github.com/aler9/writerseeker"
)

type Storage struct {
	fpath string
	f     *os.File
	offset uint64
	partIdx int
	size uint64
	buffer *writerseeker.WriterSeeker
	closed bool
}

func NewStorage(fpath string) *Storage {
	f, err := os.Create(fpath)
	if err != nil {
		panic(err)
	}
	return &Storage{
		fpath: fpath,
		f:     f,
	}
}

func (f *Storage) CurrentPartOffset() uint64 {
	return f.offset
}

func (f *Storage) NextPart() int {
	if f.buffer != nil {
		b := f.buffer.Bytes()
		size := len(b)
		_, err := f.f.Write(b)
		if err != nil {
			panic(err)
		}
		err = f.f.Sync()
		if err != nil {
			panic(err)
		}
		f.offset += uint64(size)
		f.size += uint64(size)
		f.partIdx += 1
		f.buffer = nil
		return size
	} else {
		return 0
	}
}

func (f *Storage) Writer() io.WriteSeeker {
	if f.buffer != nil {
		return f.buffer
	} else {
		w := &writerseeker.WriterSeeker{}
		f.buffer = w
		return w
	}
}

func (f *Storage) Close() {
	f.NextPart()
	f.closed = true
	f.Close()
}
