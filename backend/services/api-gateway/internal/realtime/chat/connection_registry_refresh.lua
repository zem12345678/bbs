local userKey = KEYS[1]
local ipKey = KEYS[2]
local expiresAt = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
local leaseID = ARGV[3]
local trackUser = tonumber(ARGV[4]) == 1
local trackIP = tonumber(ARGV[5]) == 1

if trackUser and not redis.call('ZSCORE', userKey, leaseID) then
  return 1
end

if trackIP and not redis.call('ZSCORE', ipKey, leaseID) then
  return 1
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
