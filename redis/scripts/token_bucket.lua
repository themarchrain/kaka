-- Redis TokenBucket：Hash 存 tokens / last_refilled_ms
-- KEYS[1]  = 限流 key
-- ARGV[1]  = capacity
-- ARGV[2]  = rate（每秒补充令牌数）
-- ARGV[3]  = key TTL 毫秒（<=0 不设 TTL）
-- 返回 {allowed, remaining, retry_after_ms}
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)

local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local ttl_ms = tonumber(ARGV[3])

local fields = redis.call('HMGET', KEYS[1], 'tokens', 'last_refilled_ms')
local tokens, last
if not fields[1] then
    tokens = capacity
    last = now
else
    tokens = tonumber(fields[1])
    last = tonumber(fields[2])
    -- 时钟回拨防护：服务器时间回拨时不改写状态时间，
    -- 用状态里的 last 继续（elapsed=0，不追补不超限）
    if now < last then
        now = last
    end
end

local elapsed = now - last
if elapsed > 0 then
    tokens = tokens + elapsed * rate / 1000
    if tokens > capacity then
        tokens = capacity
    end
end

local allowed = 0
local remaining = 0
local retry_after_ms = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
    remaining = math.floor(tokens)
else
    retry_after_ms = math.ceil((1 - tokens) * 1000 / rate)
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'last_refilled_ms', now)
if ttl_ms > 0 then
    redis.call('PEXPIRE', KEYS[1], ttl_ms)
end
return {allowed, remaining, retry_after_ms}
