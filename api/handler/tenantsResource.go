package handler

import (
	"context"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
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

	allcm, err := ClusterAllocMemory(ctx)
	if err != nil {
		return err
	}

	if int64(needMemory) > allcm {
		logrus.Errorf("cluster available memory is %d, To apply for %d, not enough", allcm, needMemory)
		return errors.New("cluster_lack_of_memory")
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
