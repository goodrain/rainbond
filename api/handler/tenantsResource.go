package handler

import (
	"context"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CheckTenantResource check tenant's resource is support action or not
func CheckTenantResource(ctx context.Context, tenant *dbmodel.Tenants, needMemory int) error {
	ts, err := GetServiceManager().GetTenantRes(tenant.UUID)
	if err != nil {
		return err
	}
	logrus.Debugf("tenant limitMemory: %v, usedMemory: %v", tenant.LimitMemory, ts.UsedMEM)
	if tenant.LimitMemory != 0 {
		avaiMemory := tenant.LimitMemory - ts.UsedMEM
		if needMemory > avaiMemory {
			logrus.Errorf("tenant available memory is %d, To apply for %d, not enough", avaiMemory, needMemory)
			return errors.New("tenant_lack_of_memory")
		}
	}
	clusterInfo, err := GetTenantManager().GetAllocatableResources(ctx)
	if err != nil {
		logrus.Errorf("get cluster resources failure for check tenant resource: %v", err.Error())
	}
	if clusterInfo != nil {
		clusterAvailMemory := clusterInfo.AllMemory - clusterInfo.RequestMemory
		logrus.Debugf("cluster allocatedMemory: %v, availmemory %d tenantsUsedMemory; %v", clusterInfo.RequestMemory, clusterAvailMemory, clusterInfo.RequestMemory)
		if int64(needMemory) > clusterAvailMemory {
			logrus.Errorf("cluster available memory is %d, To apply for %d, not enough", clusterAvailMemory, needMemory)
			return errors.New("cluster_lack_of_memory")
		}
	}
	return nil
}
