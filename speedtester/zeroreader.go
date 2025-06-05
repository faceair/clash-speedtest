package speedtester

import (
	"io"
)

var zeroBytes = make([]byte, 1024*1024)

type ZeroReader struct {
	remainBytes  int64
	writtenBytes int64
}

func NewZeroReader(size int) *ZeroReader {
	return &ZeroReader{
		remainBytes:  int64(size),
		writtenBytes: 0,
	}
}

func (r *ZeroReader) Read(p []byte) (n int, err error) {
	if r.remainBytes <= 0 {
		return 0, io.EOF
	}
	toRead := min(int64(len(p)), r.remainBytes)
	bytesWritten := int64(0)
	for bytesWritten < toRead {
		chunk := min(toRead-bytesWritten, int64(len(zeroBytes)))
		copy(p[bytesWritten:], zeroBytes[:chunk])
		bytesWritten += chunk
	}
	r.remainBytes -= bytesWritten
	r.writtenBytes += bytesWritten
	return int(bytesWritten), nil
}

func (r *ZeroReader) WrittenBytes() int64 {
	return r.writtenBytes
}

func (r *ZeroReader) RemainBytes() int64 {
	return r.remainBytes
}
