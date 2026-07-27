local userKey = KEYS[1]
local ipKey = KEYS[2]
local leaseID = ARGV[1]
local trackUser = tonumber(ARGV[2]) == 1
local trackIP = tonumber(ARGV[3]) == 1

if trackUser then
  redis.call('ZREM', userKey, leaseID)
end

if trackIP then
  redis.call('ZREM', ipKey, leaseID)
end

return 0
