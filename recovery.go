package pyongo

import "fmt"

func Recovery(ctx *Context) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Error(fmt.Errorf("panic: %v", err))
		}
	}()
	ctx.Next()
}
