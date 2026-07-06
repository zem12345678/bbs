# 业务异常 (Exception)

`exception` 包提供了统一的业务异常处理机制，支持错误链追踪、错误分类、堆栈追踪等高级特性。

## 特性

✨ **HTTP友好**：自动映射到HTTP状态码  
🔗 **错误链支持**：兼容Go 1.13+的errors.Is/As  
📊 **错误分类**：自动判断错误类型和是否可重试  
🔍 **堆栈追踪**：可选的函数调用栈记录  
🔒 **线程安全**：支持并发访问元数据  
🎯 **构建器模式**：灵活的错误构造方式  
✅ **完全向后兼容**：保留所有旧版API

## 快速开始

### 基础使用

```go
package main

import "gateway/pkg/exception"

func main() {
    // 创建常见的业务异常
    err := exception.NewNotFound("用户 %s 不存在", "alice")
    
    // 添加元数据
    err.WithMeta("user_id", "123")
    
    // 获取HTTP状态码
    statusCode := err.GetHttpCode() // 404
}
```

### 错误链（新功能）

```go
import (
    "errors"
    "gateway/pkg/exception"
)

// 包装底层错误
dbErr := sql.ErrNoRows
apiErr := exception.Wrap(dbErr, exception.CODE_NOT_FOUND, "查询失败")

// 支持 errors.Is 判断
if errors.Is(apiErr, sql.ErrNoRows) {
    // 可以追溯到原始错误
}

// 查看完整错误链
fmt.Println(apiErr.ErrorChainString())
// 输出: 查询失败: sql: no rows in result set → sql: no rows in result set
```

### 构建器模式（新功能）

```go
// 使用构建器创建复杂异常
err := exception.NewBuilder(exception.CODE_BAD_REQUEST).
    WithReason("参数验证失败").
    WithMessage("邮箱格式不正确").
    WithMeta("field", "email").
    WithTraceID("trace-123").
    WithStack().  // 添加堆栈追踪
    Build()
```

### 错误分类（新功能）

```go
err := exception.NewInternalServerError("系统异常")

// 判断错误类型
if err.IsServerError() {
    // 服务端错误
}

if err.IsRetryable() {
    // 可以重试
}

// 获取错误类型
errType := err.Type() // ErrorTypeServer
```

## 常用异常

| 函数 | HTTP状态码 | 说明 |
|------|-----------|------|
| `NewBadRequest(msg, args...)` | 400 | 请求参数错误 |
| `NewUnauthorized(msg, args...)` | 401 | 未认证 |
| `NewForbidden(msg, args...)` | 403 | 无权限 |
| `NewNotFound(msg, args...)` | 404 | 资源不存在 |
| `NewConflict(msg, args...)` | 409 | 资源冲突 |
| `NewInternalServerError(msg, args...)` | 500 | 服务器内部错误 |

## 新增功能

### 错误链支持

```go
// 包装错误
func Wrap(err error, code int, reason string) *ApiException

// 包装错误并格式化消息
func Wrapf(err error, code int, reason, format string, args ...any) *ApiException

// 获取原始错误
err.Unwrap() error
err.Cause() error

// 获取完整错误链
err.ErrorChain() []string
err.ErrorChainString() string
```

### 堆栈追踪

```go
// 添加堆栈信息（默认3层调用栈）
err.WithStack() *ApiException

// 指定调用栈深度
err.WithStackDepth(10) *ApiException

// 获取堆栈信息
stack := err.GetStack()
```

### 追踪标识

TraceID 和 RequestID 是**直接字段**，类型安全，自动序列化到 JSON：

```go
// 设置追踪ID（用于分布式追踪）
err.WithTraceID("trace-xxx-xxx")

// 设置请求ID
err.WithRequestID("req-xxx-xxx")

// 获取方法
traceID := err.GetTraceID()
requestID := err.GetRequestID()

// 也可以直接访问字段
traceID := err.TraceID
requestID := err.RequestID
```

### 错误类型判断

```go
// 错误类型枚举
const (
    ErrorTypeUnknown     // 未知
    ErrorTypeClient      // 客户端错误（4xx）
    ErrorTypeServer      // 服务端错误（5xx）
    ErrorTypeAuth        // 认证授权错误
    ErrorTypeValidation  // 验证错误
    ErrorTypeNotFound    // 资源不存在
    ErrorTypeConflict    // 资源冲突
)

// 判断方法
err.Type() ErrorType        // 获取错误类型
err.IsRetryable() bool      // 是否可重试
err.IsClientError() bool    // 是否客户端错误
err.IsServerError() bool    // 是否服务端错误
err.IsAuthError() bool      // 是否认证错误
```

### 构建器API

```go
// 创建构建器
builder := exception.NewBuilder(code)

// 链式调用
builder.
    WithReason(reason).
    WithMessage(message).
    WithMessagef(format, args...).
    WithHTTPCode(httpCode).
    WithMeta(key, value).
    WithData(data).
    WithCause(err).
    WithService(serviceName).
    WithTraceID(traceID).
    WithRequestID(requestID).
    WithStack().
    Build()
```

## 向后兼容

所有旧版API完全保留，现有代码无需修改：

```go
// ✅ 所有旧的创建方法继续支持
err := exception.NewNotFound("resource not found")
err.WithMeta("key", "value")
err.WithData(data)

// ✅ 所有旧的判断方法继续支持
if exception.IsNotFoundError(err) {
    // ...
}

if exception.IsApiException(err, exception.CODE_NOT_FOUND) {
    // ...
}

// ✅ 所有常量继续可用
code := exception.CODE_NOT_FOUND
```

## 性能考虑

### 堆栈跟踪开销

堆栈跟踪有一定性能开销，建议按需使用：

```go
// 仅在关键路径或难以调查的错误时收集堆栈
if criticalOperation {
    return exception.NewInternalServerError("critical error").WithStack()
}

// 简单的业务错误无需堆栈
return exception.NewBadRequest("invalid input")
```

### 元数据线程安全

`Meta` 并发访问已加锁保护，但频繁操作时考虑批量设置：

```go
// 好的做法：构建时一次性设置
return exception.NewBadRequest("error").
    WithMeta("field1", val1).
    WithMeta("field2", val2).
    WithMeta("field3", val3)

// 避免：多处并发修改同一个异常实例
```

### 错误链性能

错误链遍历是O(n)操作，避免过深的嵌套：

```go
// ✅ 合理：2-3层嵌套
err := exception.Wrap(lowLevelErr, "high level context")

// ⚠️ 避免：过深嵌套（如循环中多次Wrap）
```

## 实际使用示例

### HTTP Handler 错误处理

```go
func (h *Handler) GetUser(c *gin.Context) {
    userID := c.Param("id")
    
    user, err := h.service.GetUser(c.Request.Context(), userID)
    if err != nil {
        // 自动映射到HTTP状态码
        c.JSON(err.Code(), gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, user)
}

// Service层
func (s *UserService) GetUser(ctx context.Context, id string) (*User, exception.APIException) {
    user, err := s.repo.FindByID(ctx, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // 404错误
            return nil, exception.NewNotFound("用户不存在").
                WithMeta("user_id", id).
                WithTraceID(ctx)
        }
        // 500错误，保留原始错误链
        return nil, exception.Wrap(err, "查询用户失败").
            WithMeta("user_id", id).
            WithStack()
    }
    return user, nil
}
```

### 微服务调用错误传播

```go
func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderReq) (*Order, error) {
    // 调用库存服务
    if err := s.inventoryClient.Reserve(ctx, req.Items); err != nil {
        if apiErr, ok := err.(exception.APIException); ok {
            // 保留原始错误码并添加上下文
            return nil, exception.Wrap(apiErr, "库存预留失败").
                WithMeta("order_items", req.Items).
                WithTraceID(ctx)
        }
        return nil, exception.NewInternalServerError("库存服务异常").
            WithCause(err).
            WithStack()
    }
    
    // 调用支付服务
    if err := s.paymentClient.Charge(ctx, req.Amount); err != nil {
        // 回滚库存
        s.inventoryClient.Rollback(ctx, req.Items)
        
        return nil, exception.Wrap(err, "支付失败").
            WithMeta("amount", req.Amount).
            WithType(exception.ErrorTypeRetryable) // 可重试错误
    }
    
    // 创建订单...
    return order, nil
}
```

### 错误分类与重试逻辑

```go
func (c *Client) CallWithRetry(ctx context.Context, fn func() error) error {
    var lastErr error
    
    for i := 0; i < 3; i++ {
        err := fn()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // 检查是否可重试
        if apiErr, ok := err.(exception.APIException); ok {
            if !apiErr.IsRetryable() {
                // 客户端错误或永久性错误，不重试
                return err
            }
        }
        
        // 等待后重试
        time.Sleep(time.Second * time.Duration(i+1))
    }
    
    return exception.Wrap(lastErr, "重试3次后仍失败")
}
```

## 最佳实践

### 1. 选择合适的错误创建方式

```go
// ✅ 使用语义明确的构造函数
return exception.NewBadRequest("用户名不能为空")

// ✅ 使用Builder构建复杂错误
return exception.NewBuilder().
    BadRequest("参数验证失败").
    WithMeta("field", "email").
    WithMeta("reason", "格式错误").
    Build()

// ❌ 避免：通用错误码损失语义
return exception.New(400, "错误")
```

### 2. 何时使用 Wrap

```go
// ✅ 跨层调用时添加上下文
func (s *Service) Process(id string) error {
    data, err := s.repo.Get(id)
    if err != nil {
        // Wrap保留底层错误，添加业务上下文
        return exception.Wrap(err, "处理失败")
    }
    return nil
}

// ❌ 避免：包装已经是APIException的错误而不添加信息
if apiErr := s.doSomething(); apiErr != nil {
    return exception.Wrap(apiErr, "") // 无意义
}
```

### 3. 合理添加元数据

```go
// ✅ 添加有助于调查的信息
return exception.NewNotFound("订单不存在").
    WithMeta("order_id", orderID).
    WithMeta("user_id", userID).
    WithMeta("query_time", time.Now())

// ❌ 避免：敏感信息泄露
return exception.NewUnauthorized("认证失败").
    WithMeta("password", userPassword) // 危险！
```

### 4. 堆栈跟踪使用时机

```go
// ✅ 关键错误或难以复现的问题
if err := criticalOperation(); err != nil {
    return exception.Wrap(err, "关键操作失败").WithStack()
}

// ✅ 系统级错误
if err := db.Connect(); err != nil {
    return exception.NewInternalServerError("数据库连接失败").
        WithCause(err).
        WithStack()
}

// ❌ 避免：常见业务错误收集堆栈（性能开销）
if user == nil {
    return exception.NewNotFound("用户不存在").WithStack() // 不必要
}
```

### 5. 错误类型分类

```go
// ✅ 为可重试错误添加标记
if networkErr := callRemote(); networkErr != nil {
    return exception.NewServiceUnavailable("服务暂时不可用").
        WithCause(networkErr).
        WithType(exception.ErrorTypeRetryable)
}

// ✅ 明确客户端错误
if !validateInput(req) {
    return exception.NewBadRequest("输入验证失败").
        WithType(exception.ErrorTypeClient)
}
```

## 常见问题

### Q1: 什么时候用 `Wrap` 什么时候用 `NewXxx`？

**A:** 
- 使用 `Wrap`: 当你有一个底层错误需要添加上下文时
- 使用 `NewXxx`: 当你创建新的业务错误时

```go
// Wrap - 包装已有错误
dbErr := db.Query(...)
return exception.Wrap(dbErr, "查询用户失败")

// NewXxx - 创建新错误
if user == nil {
    return exception.NewNotFound("用户不存在")
}
```

### Q2: `WithCause` 和 `Wrap` 有什么区别？

**A:**
- `Wrap`: 包装错误时会尝试保留原错误的HTTP状态码（如果是APIException）
- `WithCause`: 仅存储原始错误，不影响当前错误的状态码

```go
// Wrap - 保留底层APIException的状态码
lowLevelErr := exception.NewNotFound("resource not found") // 404
err := exception.Wrap(lowLevelErr, "operation failed")
// err.Code() == 404

// WithCause - 使用新的状态码
err2 := exception.NewInternalServerError("system error"). // 500
    WithCause(lowLevelErr)
// err2.Code() == 500
```

### Q3: 错误链会影响性能吗？

**A:** 错误链本身开销很小，但堆栈跟踪有一定开销：
- 错误链（Wrap/Unwrap）：仅存储指针，开销可忽略
- 堆栈跟踪（WithStack）：需要收集调用栈，有一定开销
- 建议：仅在关键路径或难以调查的错误时使用 `WithStack()`

### Q4: 如何在日志中记录完整错误信息？

**A:** 使用 `GetStack()` 和 `ErrorChain()` 获取详细信息：

```go
if err != nil {
    log.Error().
        Str("error", err.Error()).
        Str("trace_id", err.GetMeta("trace_id")).
        Interface("error_chain", err.ErrorChain()).
        Str("stack", err.GetStack()).
        Msg("操作失败")
}
```

### Q5: 是否线程安全？

**A:** 
- ✅ `WithMeta/GetMeta`: 线程安全（已加锁）
- ✅ 其他方法：不可变操作，天然线程安全
- ⚠️ 建议：错误创建后避免修改，使用建造者模式一次性构建

### Q6: 如何与标准库 `errors` 包协作？

**A:** 完全兼容 Go 1.13+ 错误处理：

```go
// errors.Is
if errors.Is(err, sql.ErrNoRows) { ... }

// errors.As
var apiErr exception.APIException
if errors.As(err, &apiErr) {
    log.Printf("HTTP Code: %d", apiErr.Code())
}

// 错误链遍历
for e := err; e != nil; e = errors.Unwrap(e) {
    log.Println(e)
}
```

## 许可证

Apache License 2.0
