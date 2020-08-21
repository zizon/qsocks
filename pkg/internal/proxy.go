package internal

import (
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
