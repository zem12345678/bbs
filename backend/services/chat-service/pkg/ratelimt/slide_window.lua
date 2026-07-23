-- 1, 2, 3, 4, 5, 6, 7 这是你的元素
-- ZREMRANGEBYSCORE key1 0 6
-- 7 执行完之后

-- 限流对象
local key = KEYS[1]
-- 窗口大小
local window = tonumber(ARGV[1])
-- 阈值
local threshold = tonumber( ARGV[2])
local now = tonumber(ARGV[3])
local nonce = ARGV[4]
-- 窗口的起始时间
local min = now - window

redis.call('ZREMRANGEBYSCORE', key, '-inf', min)
local cnt = redis.call('ZCOUNT', key, '-inf', '+inf')
-- local cnt = redis.call('ZCOUNT', key, min, '+inf')
if cnt >= threshold then
-- 执行限流
return "true"
else
-- score 使用时间，member 加 nonce，避免同一毫秒内的请求互相覆盖
redis.call('ZADD', key, now, tostring(now) .. ':' .. nonce)
redis.call('PEXPIRE', key, window)
return "false"
end
