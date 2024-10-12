package handler

import (
	"context"
	"github.com/goodrain/rainbond/util/constants"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CheckTenantResource check tenant's resource is support action or not
func CheckTenantResource(ctx context.Context, tenant *dbmodel.Tenants, needMemory, needCPU, needStorage, noMemory, noCPU int) error {
	ts, err := GetServiceManager().GetTenantRes(tenant.UUID)
	if err != nil {
		return err
	}
	logrus.Debugf("tenant limitMemory: %v, usedMemory: %v", tenant.LimitMemory, ts.UsedMEM)
	if tenant.LimitMemory != 0 {
		avaiMemory := tenant.LimitMemory - ts.UsedMEM
		if avaiMemory >= 0 && needMemory > avaiMemory {
			logrus.Errorf("tenant available memory is %d, To apply for %d, not enough", avaiMemory, needMemory)
			return errors.New(constants.TenantLackOfMemory)
		}
	}
	if tenant.LimitCPU != 0 {
		avaiCPU := tenant.LimitCPU - ts.UsedCPU
		if avaiCPU >= 0 && needCPU > avaiCPU {
			logrus.Errorf("tenant available CPU is %d, To apply for %d, not enough", avaiCPU, needCPU)
			return errors.New(constants.TenantLackOfCPU)
		}
	}
	if tenant.LimitStorage != 0 {
		avaiStorage := tenant.LimitStorage - int(ts.UsedDisk)
		if avaiStorage >= 0 && needStorage > avaiStorage {
			logrus.Errorf("tenant available Storage is %d, To apply for %d, not enough", avaiStorage, needStorage)
			return errors.New(constants.TenantLackOfStorage)
		}
	}
	// check tenant resource quota
	err = GetTenantManager().CheckTenantResourceQuotaAndLimitRange(ctx, tenant.Namespace, noMemory, noCPU)
	if err != nil {
		return err
	}
	allcm, err := ClusterAllocMemory(ctx)
	if err != nil {
		return err
	}

	if int64(needMemory) > allcm {
		logrus.Errorf("cluster available memory is %d, To apply for %d, not enough", allcm, needMemory)
		return errors.New(constants.ClusterLackOfMemory)
	}

	return nil
}

// ClusterAllocMemory returns the allocatable memory of the cluster.
func ClusterAllocMemory(ctx context.Context) (int64, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("ClusterAllocMemory")()
	}

	clusterInfo, err := GetTenantManager().GetAllocatableResources(ctx)
	if err != nil {
		return 0, err
	}
	return clusterInfo.AllMemory - clusterInfo.RequestMemory, nil
}
