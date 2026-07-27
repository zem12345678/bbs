local userKey = KEYS[1]
local ipKey = KEYS[2]
local now = tonumber(ARGV[1])
local maxUser = tonumber(ARGV[2])
local maxIP = tonumber(ARGV[3])
local trackUser = tonumber(ARGV[4]) == 1
local trackIP = tonumber(ARGV[5]) == 1

if trackUser then
  redis.call('ZREMRANGEBYSCORE', userKey, '-inf', now)
  if redis.call('ZCARD', userKey) >= maxUser then
    return 1
  end
end

if trackIP then
  redis.call('ZREMRANGEBYSCORE', ipKey, '-inf', now)
  if redis.call('ZCARD', ipKey) >= maxIP then
    return 2
  end
end

return 0
