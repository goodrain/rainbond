package clients

import (
	"rainbond/pkg/api/region"
	"rainbond/cmd/grctl/option"
)


var RegionClient *region.Region

func InitRegionClient(reg option.RegionAPI) error {
	region.NewRegion(reg.URL,reg.Token,reg.Type)
	RegionClient=region.GetRegion()
	return nil
}

