-- Redis LeakyBucket：Hash 存 water / last_leak_ms
-- KEYS[1]  = 限流 key
-- ARGV[1]  = capacity（桶容量）
-- ARGV[2]  = rate（每秒漏水量）
-- ARGV[3]  = key TTL 毫秒（<=0 不设 TTL）
-- 返回 {allowed, remaining, retry_after_ms}
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)

local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local ttl_ms = tonumber(ARGV[3])

local fields = redis.call('HMGET', KEYS[1], 'water', 'last_leak_ms')
local water, last
if not fields[1] then
    water = 0
    last = now
else
    water = tonumber(fields[1])
    last = tonumber(fields[2])
end

-- 漏水：elapsed * rate，水量不低于 0
local elapsed = now - last
if elapsed > 0 then
    water = water - elapsed * rate / 1000
    if water < 0 then
        water = 0
    end
end

local allowed = 0
local remaining = 0
local retry_after_ms = 0
if water + 1 <= capacity then
    water = water + 1
    allowed = 1
    remaining = math.floor(capacity - water)
else
    local overflow = water + 1 - capacity
    retry_after_ms = math.ceil(overflow * 1000 / rate)
end

redis.call('HSET', KEYS[1], 'water', water, 'last_leak_ms', now)
if ttl_ms > 0 then
    redis.call('PEXPIRE', KEYS[1], ttl_ms)
end
return {allowed, remaining, retry_after_ms}
