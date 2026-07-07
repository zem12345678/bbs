package logger

import (
	"notification-service/pkg/logger/types"
)

// 重新导出类型
type (
	Logger    = types.Logger
	Field     = types.Field
	NopLogger = types.NopLogger
	ZapLogger = types.ZapLogger
)

// 重新导出构造函数
var (
	NewNopLogger = types.NewNopLogger
	NewZapLogger = types.NewZapLogger
)

// 重新导出 Field 构造器
var (
	String = types.String
	Int    = types.Int
	Int64  = types.Int64
	Int32  = types.Int32
	Bool   = types.Bool
	Error  = types.Error
	Any    = types.Any
)
