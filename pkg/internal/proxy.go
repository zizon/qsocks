package internal

import (
	"fmt"
	"io"
)

// BiCopy copy bidirectional for both first and second
func BiCopy(ctx CanclableContext, first io.ReadWriter, second io.ReadWriter, copyIntoFrom func(io.Writer, io.Reader) (int64, error)) {
	goCopy := func(reader io.Reader, writer io.Writer) {
		go func() {
			if _, err := copyIntoFrom(writer, reader); err != nil {
				ctx.CancleWithError(err)
				return
			}

			ctx.Cancle()
		}()
	}

	goCopy(first, second)
	goCopy(second, first)
}

func DebugPrintCopyFrom(w io.Writer, r io.Reader) (int64, error) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		fmt.Printf("read:%s \n", string(buf[:n]))
		if err != nil {
			return 0, err
		}

		w.Write(buf[:n])
		if err != nil {
			return 0, err
		}
	}
}
