local json = require("cjson")

local configuration_data = ngx.shared.configuration_data

local _M = {
  nameservers = {}
}

function _M.get_backends_data()
  return configuration_data:get("backends")
end

local function fetch_request_body()
  ngx.req.read_body()
  local body = ngx.req.get_body_data()

  if not body then
    -- request body might've been written to tmp file if body > client_body_buffer_size
    local file_name = ngx.req.get_body_file()
    local file = io.open(file_name, "rb")

    if not file then
      return nil
    end

    body = file:read("*all")
    file:close()
  end

  return body
end

function _M.call()
  if ngx.var.request_method ~= "POST" and ngx.var.request_method ~= "GET" then
    ngx.log(ngx.ERR, "Only POST and GET requests are allowed!")
    ngx.status = ngx.HTTP_BAD_REQUEST
    return
  end

  if ngx.var.request_uri ~= "/config/backends" then
    ngx.status = ngx.HTTP_NOT_FOUND
    ngx.print("Not found!")
    return
  end

  if ngx.var.request_method == "GET" then
    ngx.status = ngx.HTTP_OK
    ngx.print(_M.get_backends_data())
    return
  end

  local backends = fetch_request_body()
  if not backends then
    ngx.log(ngx.ERR, "dynamic-configuration: unable to read valid request body")
    ngx.status = ngx.HTTP_BAD_REQUEST
    return
  end

  local success, err = configuration_data:set("backends", backends)
  if not success then
    ngx.log(ngx.ERR, "dynamic-configuration: error updating configuration: " .. tostring(err))
    ngx.status = ngx.HTTP_BAD_REQUEST
    return
  end

  ngx.log(ngx.INFO, "successfully read or update backend data ")
  ngx.status = ngx.HTTP_CREATED
end

return _M
