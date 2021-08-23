
local ngx_balancer = require("ngx.balancer")
local round_robin = require("balancer.round_robin")
local chash = require("balancer.chash")
local sticky = require("balancer.sticky")
local ewma = require("balancer.ewma")
local json = require("cjson")
local config = require("config")

local DEFAULT_LB_ALG = "round_robin"
local IMPLEMENTATIONS = {
  round_robin = round_robin,
  chash = chash,
  sticky = sticky,
  ewma = ewma,
}

local _M = {}
-- save all backend balancer data
local balancers = {}

-- measured in seconds
-- for an Nginx worker to pick up the new list of upstream peers
-- it will take <the delay until controller POSTed the backend object to the Nginx endpoint> + BACKENDS_SYNC_INTERVAL
local BACKENDS_SYNC_INTERVAL = 1

-- get_implementation get backend balance type 
-- if set sessionAffinityConfig.name is cookie, type is sticky
-- if set upstream-hash-by, type is chash
local function get_implementation(backend)
  local name = backend["load-balance"] or DEFAULT_LB_ALG

  if backend["sessionAffinityConfig"] and backend["sessionAffinityConfig"]["name"] == "cookie" then
    name = "sticky"
  elseif backend["upstream-hash-by"] then
    name = "chash"
  end

  local implementation = IMPLEMENTATIONS[name]
  if not implementation then
    ngx.log(ngx.WARN, string.format("%s is not supported, falling back to %s", backend["load-balance"], DEFAULT_LB_ALG))
    implementation = IMPLEMENTATIONS[DEFAULT_LB_ALG]
  end

  return implementation
end

local function get_balancer()
  local backend_name = ngx.var.target
  return balancers[backend_name]
end

--  sync_backend sync define backend data 
local function sync_backend(backend)
  local implementation = get_implementation(backend)
  local balancer = balancers[backend.name]

  if not balancer then
    balancers[backend.name] = implementation:new(backend)
    return
  end

  -- every implementation is the metatable of its instances (see .new(...) functions)
  -- here we check if `balancer` is the instance of `implementation`
  -- if it is not then we deduce LB algorithm has changed for the backend
  if getmetatable(balancer) ~= implementation then
    ngx.log(ngx.INFO,
      string.format("LB algorithm changed from %s to %s, resetting the instance", balancer.name, implementation.name))
    balancers[backend.name] = implementation:new(backend)
    return
  end
  balancer:sync(backend)
end

-- sync_backends sync all backend every 1 seconds
local function sync_backends()
  local backends_data = config.get_backends_data()
  if not backends_data then
    balancers = {}
    return
  end

  local ok, new_backends = pcall(json.decode, backends_data)
  if not ok then
    ngx.log(ngx.ERR,  "could not parse backends data: " .. tostring(new_backends))
    return
  end

  local balancers_to_keep = {}
  for _, new_backend in ipairs(new_backends) do
    sync_backend(new_backend)
    balancers_to_keep[new_backend.name] = balancers[new_backend.name]
  end

  for backend_name, _ in pairs(balancers) do
    if not balancers_to_keep[backend_name] then
      balancers[backend_name] = nil
    end
  end
end

function _M.init_worker()
  sync_backends() -- when worker starts, sync backends without delay
  local _, err = ngx.timer.every(BACKENDS_SYNC_INTERVAL, sync_backends)
  if err then
    ngx.log(ngx.ERR, string.format("error when setting up timer.every for sync_backends: %s", tostring(err)))
  end
end

function _M.rewrite()
  local balancer = get_balancer()
  if not balancer then
    ngx.status = ngx.HTTP_SERVICE_UNAVAILABLE
    return ngx.exit(ngx.status)
  end
end

function _M.balance()
  local balancer = get_balancer()
  if not balancer then
    return
  end

  local peer = balancer:balance()
  if not peer then
    ngx.log(ngx.WARN, "no peer was returned, balancer: " .. balancer.name)
    return
  end

  ngx_balancer.set_more_tries(1)

  local ok, err = ngx_balancer.set_current_peer(peer)
  if not ok then
    ngx.log(ngx.ERR, string.format("error while setting current upstream peer %s: %s", peer, err))
  end
  ngx.log(ngx.INFO, string.format("successfully set current upstream peer %s: %s", peer, err))
end

function _M.log()
  local balancer = get_balancer()
  if not balancer then
    return
  end

  if not balancer.after_balance then
    return
  end

  balancer:after_balance()
end

return _M