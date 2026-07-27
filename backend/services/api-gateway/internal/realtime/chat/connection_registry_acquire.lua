local userKey = KEYS[1]
local ipKey = KEYS[2]
local now = tonumber(ARGV[1])
local expiresAt = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local leaseID = ARGV[4]
local maxUser = tonumber(ARGV[5])
local maxIP = tonumber(ARGV[6])
local trackUser = tonumber(ARGV[7]) == 1
local trackIP = tonumber(ARGV[8]) == 1

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

if trackUser then
  redis.call('ZADD', userKey, expiresAt, leaseID)
  redis.call('PEXPIRE', userKey, ttl)
end

if trackIP then
  redis.call('ZADD', ipKey, expiresAt, leaseID)
  redis.call('PEXPIRE', ipKey, ttl)
end

return 0
