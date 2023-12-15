package segmenter

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/aler9/writerseeker"
)

type DumyWriterSeeker struct {}

func (ws *DumyWriterSeeker) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("dumy writer can not be written")
}

func (ws *DumyWriterSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("dumy seeker can not be seek")
}

func MakeSureDirOf(fpath string) error {
	dir := path.Dir(fpath)
	if dir != "." {
		return os.MkdirAll(dir, os.ModePerm)
	} else {
		return nil
	}
}

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
	err := MakeSureDirOf(fpath)
	if err != nil {
		panic(err)
	}
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
	if f.closed {
		return &DumyWriterSeeker{}
	}
	if f.buffer != nil {
		return f.buffer
	} else {
		w := &writerseeker.WriterSeeker{}
		f.buffer = w
		return w
	}
}

func (f *Storage) Close() {
	f.closed = true
	f.NextPart()
	f.f.Close()
}
