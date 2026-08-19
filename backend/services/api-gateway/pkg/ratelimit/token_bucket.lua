local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_interval = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local state = redis.call('HMGET', key, 'tokens', 'last')
local tokens = tonumber(state[1])
local last = tonumber(state[2])

if tokens == nil or last == nil then
    tokens = capacity
    last = now
elseif now > last then
    tokens = math.min(capacity, tokens + ((now - last) / refill_interval))
    last = now
end

local limited = tokens < 1
if not limited then
    tokens = tokens - 1
end

redis.call('HSET', key, 'tokens', tostring(tokens), 'last', tostring(last))
redis.call('PEXPIRE', key, math.ceil(capacity * refill_interval * 2))

if limited then
    return 1
end
return 0
