package handler

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/api/model"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/require"
)

func TestBatchBindServiceLoadsRequestedComponentsInOneQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao := dbdao.NewMockTenantServiceDao(ctrl)
	requested := []string{"component-b", "missing", "component-a", "component-a"}
	dao.EXPECT().GetServicesByServiceIDs(requested).Return([]*dbmodel.TenantServices{
		{ServiceID: "component-a"},
		{ServiceID: "component-b"},
	}, nil)
	dao.EXPECT().BindAppByServiceIDs("app-id", []string{"component-b", "component-a", "component-a"}).Return(nil)

	err := batchBindService(dao, "app-id", model.BindServiceRequest{ServiceIDs: requested})

	require.NoError(t, err)
}

func TestBatchBindServiceDoesNotBindWhenBulkLookupFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao := dbdao.NewMockTenantServiceDao(ctrl)
	expectedErr := errors.New("database unavailable")
	dao.EXPECT().GetServicesByServiceIDs([]string{"component-a"}).Return(nil, expectedErr)

	err := batchBindService(dao, "app-id", model.BindServiceRequest{ServiceIDs: []string{"component-a"}})

	require.ErrorIs(t, err, expectedErr)
}
