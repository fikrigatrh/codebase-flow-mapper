package notify

import "context"

func WithCancel() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx
}
