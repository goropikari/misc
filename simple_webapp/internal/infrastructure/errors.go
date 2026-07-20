package infrastructure

import "errors"

// ErrNotImplemented は、このインフラ層のメソッドが未実装のスタブであることを示します。
var ErrNotImplemented = errors.New("infrastructure method is not implemented")
