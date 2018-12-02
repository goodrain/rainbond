
local pairs = pairs
local next = next
local tonumber = tonumber
local setmetatable = setmetatable


local _M = {}
local mt = { __index = _M }


local function copy(nodes)
    local newnodes = {}
    for id, weight in pairs(nodes) do
        newnodes[id] = weight
    end

    return newnodes
end


local _gcd
_gcd = function (a, b)
    if b == 0 then
        return a
    end

    return _gcd(b, a % b)
end


local function get_gcd(nodes)
    local first_id, max_weight = next(nodes)
    if not first_id then
        return error("empty nodes")
    end

    local only_key = first_id
    local gcd = max_weight
    for id, weight in next, nodes, first_id do
        only_key = nil
        gcd = _gcd(gcd, weight)
        max_weight = weight > max_weight and weight or max_weight
    end

    return only_key, gcd, max_weight
end


function _M.new(_, nodes)
    local newnodes = copy(nodes)
    local only_key, gcd, max_weight = get_gcd(newnodes)

    local self = {
        nodes = newnodes,  -- it's safer to copy one
        only_key = only_key,
        max_weight = max_weight,
        gcd = gcd,
        cw = max_weight,
        last_id = nil,
    }
    return setmetatable(self, mt)
end


function _M.reinit(self, nodes)
    local newnodes = copy(nodes)
    self.only_key, self.gcd, self.max_weight = get_gcd(newnodes)

    self.nodes = newnodes
    self.last_id = nil
    self.cw = self.max_weight
end


local function _delete(self, id)
    local nodes = self.nodes

    nodes[id] = nil

    self.only_key, self.gcd, self.max_weight = get_gcd(nodes)

    if id == self.last_id then
        self.last_id = nil
    end

    if self.cw > self.max_weight then
        self.cw = self.max_weight
    end
end
_M.delete = _delete


local function _decr(self, id, weight)
    local weight = tonumber(weight) or 1
    local nodes = self.nodes

    local old_weight = nodes[id]
    if not old_weight then
        return
    end

    if old_weight <= weight then
        return _delete(self, id)
    end

    nodes[id] = old_weight - weight

    self.only_key, self.gcd, self.max_weight = get_gcd(nodes)

    if self.cw > self.max_weight then
        self.cw = self.max_weight
    end
end
_M.decr = _decr


local function _incr(self, id, weight)
    local weight = tonumber(weight) or 1
    local nodes = self.nodes

    nodes[id] = (nodes[id] or 0) + weight

    self.only_key, self.gcd, self.max_weight = get_gcd(nodes)
end
_M.incr = _incr



function _M.set(self, id, new_weight)
    local new_weight = tonumber(new_weight) or 0
    local old_weight = self.nodes[id] or 0

    if old_weight == new_weight then
        return
    end

    if old_weight < new_weight then
        return _incr(self, id, new_weight - old_weight)
    end

    return _decr(self, id, old_weight - new_weight)
end


local function find(self)
    local only_key = self.only_key
    if only_key then
        return only_key
    end

    local nodes = self.nodes
    local last_id, cw, weight = self.last_id, self.cw

    while true do
        while true do
            last_id, weight = next(nodes, last_id)
            if not last_id then
                break
            end

            if weight >= cw then
                self.cw = cw
                self.last_id = last_id
                return last_id
            end
        end

        cw = cw - self.gcd
        if cw <= 0 then
            cw = self.max_weight
        end
    end
end
_M.find = find
_M.next = find


return _M
