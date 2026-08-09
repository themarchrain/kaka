-- Redis SlidingWindowLog：ZSet，score=毫秒时间戳，member=唯一盐
-- KEYS[1]  = 限流 key
-- ARGV[1]  = limit（窗口内最大请求数）
-- ARGV[2]  = window_ms（窗口大小）
-- ARGV[3]  = key TTL 毫秒（<=0 不设 TTL）
-- ARGV[4]  = member（唯一盐，由调用方生成，防同毫秒 ZADD 覆盖）
-- 返回 {allowed, remaining, retry_after_ms}
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)

local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local ttl_ms = tonumber(ARGV[3])
local member = ARGV[4]

-- 清理窗口外条目（score <= now - window_ms，与 memory 的 After 严格大于对齐）
local window_start = now - window_ms
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', window_start)
local count = redis.call('ZCARD', KEYS[1])

local allowed = 0
local remaining = 0
local retry_after_ms = 0
if count < limit then
    redis.call('ZADD', KEYS[1], now, member)
    allowed = 1
    remaining = limit - count - 1
else
    local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
    retry_after_ms = tonumber(oldest[2]) + window_ms - now
end

if ttl_ms > 0 then
    redis.call('PEXPIRE', KEYS[1], ttl_ms)
end
return {allowed, remaining, retry_after_ms}
