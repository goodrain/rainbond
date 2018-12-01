--
-- some of them borrow from https://github.com/cloudflare/lua-resty-json
--

local bit = require "bit"
local ffi = require 'ffi'


local ffi_new = ffi.new
local C = ffi.C
local crc32 = ngx.crc32_short
local setmetatable = setmetatable
local floor = math.floor
local pairs = pairs
local tostring = tostring
local tonumber = tonumber
local bxor = bit.bxor


ffi.cdef[[
typedef unsigned int uint32_t;

typedef struct {
    uint32_t hash;
    uint32_t id;
} chash_point_t;

void chash_point_init(chash_point_t *points, uint32_t base_hash, uint32_t start,
    uint32_t num, uint32_t id);
void chash_point_sort(chash_point_t *points, uint32_t size);

void chash_point_add(chash_point_t *old_points, uint32_t old_length,
    uint32_t base_hash, uint32_t from, uint32_t num, uint32_t id,
    chash_point_t *new_points);
void chash_point_reduce(chash_point_t *old_points, uint32_t old_length,
    uint32_t base_hash, uint32_t from, uint32_t num, uint32_t id);
void chash_point_delete(chash_point_t *old_points, uint32_t old_length,
    uint32_t id);
]]


local ok, new_tab = pcall(require, "table.new")
if not ok or type(new_tab) ~= "function" then
    new_tab = function (narr, nrec) return {} end
end


--
-- Find shared object file package.cpath, obviating the need of setting
-- LD_LIBRARY_PATH
-- Or we should add a little patch for ffi.load ?
--
local function load_shared_lib(so_name)
    local string_gmatch = string.gmatch
    local string_match = string.match
    local io_open = io.open
    local io_close = io.close

    local cpath = package.cpath

    for k, _ in string_gmatch(cpath, "[^;]+") do
        local fpath = string_match(k, "(.*/)")
        fpath = fpath .. so_name

        -- Don't get me wrong, the only way to know if a file exist is trying
        -- to open it.
        local f = io_open(fpath)
        if f ~= nil then
            io_close(f)
            return ffi.load(fpath)
        end
    end
end


local _M = {}
local mt = { __index = _M }


local clib = load_shared_lib("librestychash.so")
if not clib then
    error("can not load librestychash.so")
end

local CONSISTENT_POINTS = 160   -- points per server
local pow32 = math.pow(2, 32)

local chash_point_t = ffi.typeof("chash_point_t[?]")


local function _precompute(nodes)
    local n, total_weight = 0, 0
    for id, weight in pairs(nodes) do
        n = n + 1
        total_weight = total_weight + weight
    end

    local newnodes = new_tab(0, n)
    for id, weight in pairs(nodes) do
        newnodes[id] = weight
    end

    local ids = new_tab(n, 0)
    local npoints = total_weight * CONSISTENT_POINTS
    local points = ffi_new(chash_point_t, npoints)

    local start, index = 0, 0
    for id, weight in pairs(nodes) do
        local num = weight * CONSISTENT_POINTS
        local base_hash = bxor(crc32(tostring(id)), 0xffffffff)

        index = index + 1
        ids[index] = id

        clib.chash_point_init(points, base_hash, start, num, index)

        start = start + num
    end

    clib.chash_point_sort(points, npoints)

    return ids, points, npoints, newnodes
end


function _M.new(_, nodes)
    local ids, points, npoints, newnodes = _precompute(nodes)

    local self = {
        nodes = newnodes,  -- it's safer to copy one
        ids = ids,
        points = points,
        npoints = npoints,    -- points number
        size = npoints,
    }
    return setmetatable(self, mt)
end


function _M.reinit(self, nodes)
    self.ids, self.points, self.npoints, self.newnodes = _precompute(nodes)
    self.size = self.npoints
end


local function _delete(self, id)
    local nodes = self.nodes
    local ids = self.ids
    local old_weight = nodes[id]

    if not old_weight then
        return
    end

    local index = 1
    -- find the index: O(n)
    while ids[index] ~= id do
        index = index + 1
    end

    nodes[id] = nil
    ids[index] = nil

    clib.chash_point_delete(self.points, self.npoints, index)

    self.npoints = self.npoints - CONSISTENT_POINTS * old_weight
end
_M.delete = _delete


local function _incr(self, id, weight)
    local weight = tonumber(weight) or 1
    local nodes = self.nodes
    local ids = self.ids
    local old_weight = nodes[id]

    local index = 1
    if old_weight then
        -- find the index: O(n)
        while ids[index] ~= id do
            index = index + 1
        end

    else
        old_weight = 0

        index = #ids + 1
        ids[index] = id
    end

    nodes[id] = old_weight + weight

    local new_points = self.points
    local new_npoints = self.npoints + weight * CONSISTENT_POINTS
    if self.size < new_npoints then
        new_points = ffi_new(chash_point_t, new_npoints)
        self.size = new_npoints
    end

    local base_hash = bxor(crc32(tostring(id)), 0xffffffff)
    clib.chash_point_add(self.points, self.npoints, base_hash,
                         old_weight * CONSISTENT_POINTS,
                         weight * CONSISTENT_POINTS,
                         index, new_points)

    self.points = new_points
    self.npoints = new_npoints
end
_M.incr = _incr


local function _decr(self, id, weight)
    local weight = tonumber(weight) or 1
    local nodes = self.nodes
    local ids = self.ids
    local old_weight = nodes[id]

    if not old_weight then
        return
    end

    if old_weight <= weight then
        return _delete(self, id)
    end

    local index = 1
    -- find the index: O(n)
    while ids[index] ~= id do
        index = index + 1
    end

    local base_hash = bxor(crc32(tostring(id)), 0xffffffff)
    clib.chash_point_reduce(self.points, self.npoints, base_hash,
                            (old_weight - weight) * CONSISTENT_POINTS,
                            CONSISTENT_POINTS * weight,
                            index)

    nodes[id] = old_weight - weight
    self.npoints = self.npoints - CONSISTENT_POINTS * weight
end
_M.decr = _decr


function _M.set(self, id, new_weight)
    local new_weight = tonumber(new_weight) or 0
    local old_weight = self.nodes[id] or 0

    if old_weight == new_weight then
        return true
    end

    if old_weight < new_weight then
        return _incr(self, id, new_weight - old_weight)
    end

    return _decr(self, id, old_weight - new_weight)
end


local function _find_id(points, npoints, hash)
    local step = pow32 / npoints
    local index = floor(hash / step)

    local max_index = npoints - 1

    -- it seems safer to do this
    if index > max_index then
        index = max_index
    end

    -- find the first points >= hash
    if points[index].hash >= hash then
        for i = index, 1, -1 do
            if points[i - 1].hash < hash then
                return points[i].id, i
            end
        end

        return points[0].id, 0
    end

    for i = index + 1, max_index do
        if hash <= points[i].hash then
            return points[i].id, i
        end
    end

    return points[0].id, 0
end


function _M.find(self, key)
    local hash = crc32(tostring(key))

    local id, index = _find_id(self.points, self.npoints, hash)

    return self.ids[id], index
end


function _M.next(self, index)
    local new_index = (index + 1) % self.npoints
    local id = self.points[new_index].id

    return self.ids[id], new_index
end


return _M
