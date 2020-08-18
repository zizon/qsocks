package internal

import (
	"io"
)

// BiCopy copy bidirectional for both first and second
func BiCopy(ctx CanclableContext, first io.ReadWriter, second io.ReadWriter) CanclableContext {
	biCtx := ctx.Derive(nil)

	goCopy := func(reader io.Reader, writer io.Writer) {
		go func() {
			if _, err := io.Copy(writer, reader); err != nil {
				biCtx.CancleWithError(err)
				return
			}

			biCtx.Cancle()
		}()
	}

	goCopy(first, second)
	goCopy(second, first)

	return biCtx
}
