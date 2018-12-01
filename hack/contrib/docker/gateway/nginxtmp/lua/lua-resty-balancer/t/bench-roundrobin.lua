require "resty.core"
require "jit.opt".start("minstitch=2", "maxtrace=4000",
                        "maxrecord=8000", "sizemcode=64",
                        "maxmcode=4000", "maxirconst=1000")

local local_dir = arg[1]

-- ngx.say("local dir: ", local_dir)

package.path = local_dir .. "/lib/?.lua;" .. package.path
package.cpath = local_dir .. "/?.so;" .. package.cpath

local base_time

-- should run typ = nil first
local function bench(num, name, func, typ, ...)
    ngx.update_time()
    local start = ngx.now()

    for i = 1, num do
        func(...)
    end

    ngx.update_time()
    local elasped = ngx.now() - start

    if typ then
        elasped = elasped - base_time
    end

    ngx.say(name)
    ngx.say(num, " times")
    ngx.say("elasped: ", elasped)
    ngx.say("")

    if not typ then
        base_time = elasped
    end
end


local resty_rr = require "resty.roundrobin"

local servers = {
    ["server1"] = 10,
    ["server2"] = 2,
    ["server3"] = 1,
}

local servers2 = {
    ["server1"] = 100,
    ["server2"] = 20,
    ["server3"] = 10,
}

local servers3 = {
    ["server0"] = 1,
    ["server1"] = 1,
    ["server2"] = 1,
    ["server3"] = 1,
    ["server4"] = 1,
    ["server5"] = 1,
    ["server6"] = 1,
    ["server7"] = 1,
    ["server8"] = 1,
    ["server9"] = 1,
    ["server10"] = 1,
    ["server11"] = 1,
    ["server12"] = 1,
}

local rr = resty_rr:new(servers)

local function gen_func(typ)
    local i = 0

    if typ == 0 then
        return function ()
            i = i + 1

            resty_rr:new(servers)
        end
    end

    if typ == 1 then
        return function ()
            i = i + 1

            local servers = {
                ["server1" .. i] = 10,
                ["server2" .. i] = 2,
                ["server3" .. i] = 1,
            }
            local rr = resty_rr:new(servers)
        end
    end

    if typ == 2 then
        return function ()
            i = i + 1

            local servers = {
                ["server1" .. i] = 10,
                ["server2" .. i] = 2,
                ["server3" .. i] = 1,
            }
            local rr = resty_rr:new(servers)
            rr:incr("server3" .. i)
        end, typ
    end

    if typ == 100 then
        return function ()
            i = i + 1
        end
    end

    if typ == 101 then
        return function ()
            i = i + 1

            rr:find(i)
            i = i + 1
        end, typ
    end
end

bench(10 * 1000, "rr new servers", resty_rr.new, nil, nil, servers)
bench(1 * 1000, "rr new servers2", resty_rr.new, nil, nil, servers2)
bench(10 * 1000, "rr new servers3", resty_rr.new, nil, nil, servers3)
bench(10 * 1000, "new in func", gen_func(0))
bench(10 * 1000, "new dynamic", gen_func(1))
bench(10 * 1000, "incr server3", gen_func(2))

bench(1000 * 1000, "base for find", gen_func(100))

bench(1000 * 1000, "find from 3 servers", gen_func(101))
rr:delete("server2")
bench(1000 * 1000, "find from 2 servers", gen_func(101))
rr:delete("server3")
bench(1000 * 1000, "find from 1 server", gen_func(101))
