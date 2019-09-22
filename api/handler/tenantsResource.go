package handler

import (
	"github.com/Sirupsen/logrus"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/pkg/errors"
)

// CheckTenantResource check tenant's resource is support action or not
func CheckTenantResource(tenant *dbmodel.Tenants, needMemory int) error {
	ts, err := GetServiceManager().GetTenantRes(tenant.UUID)
	if err != nil {
		return err
	}
	logrus.Debugf("tenant limitMemory: %v, usedMemory: %v", tenant.LimitMemory, ts.UsedMEM)
	if tenant.LimitMemory != 0 {
		//tenant.LimitMemory: 租户的总资源 ts.UsedMEM: 租户使用的资源
		avaiMemory := tenant.LimitMemory - ts.UsedMEM

		if needMemory > avaiMemory {
			logrus.Error("超出租户可用资源")

			return errors.New("tenant_lack_of_memory")
		}
	}

	clusterInfo, err := GetTenantManager().GetAllocatableResources() //节点可用资源
	if err != nil {
		return err
	}

	logrus.Debugf("cluster allocatedMemory: %v, tenantsUsedMemory; %v", clusterInfo.AllMemory, clusterInfo.RequestMemory)

	// clusterInfo.AllMemory: 集群总资源 clusterInfo.RequestMemory: 集群已使用资源
	clusterAvailMemory := clusterInfo.AllMemory - clusterInfo.RequestMemory
	if int64(needMemory) > clusterAvailMemory {
		logrus.Error("超出集群可用资源")
		return errors.New("cluster_lack_of_memory")
	}

	return nil
}
