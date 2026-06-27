package internal

import (
	"errors"
	"fmt"
	"runtime/debug"
)

// errorWithStack 包装错误并携带堆栈信息
//
//nolint:errname // internal helper type
type errorWithStack struct {
	Err   error
	Stack []byte
}

func (e *errorWithStack) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *errorWithStack) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// WithStack 为错误添加堆栈信息（如果还没有）
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	// 检查是否已经有堆栈
	var existing *errorWithStack
	if errors.As(err, &existing) {
		return err
	}
	return &errorWithStack{
		Err:   err,
		Stack: debug.Stack(),
	}
}

// Errorf 创建一个带格式的错误，并包装原始错误
func Errorf(format string, args ...any) error {
	return WithStack(fmt.Errorf(format, args...))
}
