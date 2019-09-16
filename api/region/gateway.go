package region

import (
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	utilhttp "github.com/goodrain/rainbond/util/http"
)

type GatewayInterface interface {
	AddGwcIp(ip string) *util.APIHandleError
	DelGwcIp(ip string) *util.APIHandleError
}

type gateway struct {
	regionImpl
	prefix string
}

func (g *gateway) AddGwcIp(ip string) *util.APIHandleError {
	var decode utilhttp.ResponseBody
	var gwcIp dbmodel.GwcIP = dbmodel.GwcIP{IP: ip}
	decode.Bean = &gwcIp
	code, err := g.DoRequest(g.prefix, "POST", nil, &decode)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}

func (g *gateway) DelGwcIp(ip string) *util.APIHandleError {
	var decode utilhttp.ResponseBody
	var gwcIp dbmodel.GwcIP = dbmodel.GwcIP{IP: ip}
	decode.Bean = &gwcIp
	code, err := g.DoRequest(g.prefix, "DELETE", nil, &decode)
	if err != nil {
		return util.CreateAPIHandleError(code, err)
	}
	return nil
}

func (g *gateway) UpdateGwcRuleState(state bool) error {
	return nil
}

func (r *regionImpl) Gateway() GatewayInterface {
	return &gateway{prefix: "/v2/gwcip/gwc-ip", regionImpl: *r}
}
