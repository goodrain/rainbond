package conversion

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
)

func TestDependOnEachOther(t *testing.T) {
	type service struct {
		serviceID     string
		depServiceIDs []string
	}

	type depService struct {
		serviceID       string
		interdependence bool
	}

	tests := []struct {
		name        string
		serviceID   string
		services    []service
		depServices []depService
	}{
		{
			name:      "direct interdependence",
			serviceID: "apple",
			services: []service{
				{
					serviceID:     "banana",
					depServiceIDs: []string{"apple"},
				},
			},
			depServices: []depService{
				{
					serviceID:       "banana",
					interdependence: true,
				},
			},
		},
		{
			name:      "no interdependence",
			serviceID: "apple",
			services: []service{
				{
					serviceID: "banana",
				},
			},
			depServices: []depService{
				{
					serviceID:       "banana",
					interdependence: false,
				},
			},
		},
		{
			//  __ _ _ _ _ _ _ _ ___
			//  ↓  	               ↑
			// apply -> banana -> cat
			//  ↓ - - -> bag
			name:      "indirect interdependence",
			serviceID: "apple",
			services: []service{
				{
					serviceID:     "banana",
					depServiceIDs: []string{"cat"},
				},
				{
					serviceID:     "cat",
					depServiceIDs: []string{"apple"},
				},
				{
					serviceID: "bag",
				},
			},
			depServices: []depService{
				{
					serviceID:       "banana",
					interdependence: true,
				},
				{
					serviceID:       "bag",
					interdependence: false,
				},
			},
		},
		{
			//  __ _ _ _ _ _ _ _ _ _ _ _ _ __
			//  ↓  	                        ↑
			// apply -> banana -> cat -> elephant
			//  ↓ - - -> bag -> flower
			name:      "complex indirect interdependence",
			serviceID: "apple",
			services: []service{
				{
					serviceID:     "banana",
					depServiceIDs: []string{"cat"},
				},
				{
					serviceID:     "cat",
					depServiceIDs: []string{"elephant"},
				},
				{
					serviceID:     "elephant",
					depServiceIDs: []string{"apple"},
				},
				{
					serviceID:     "bag",
					depServiceIDs: []string{"flower"},
				},
				{
					serviceID: "flower",
				},
			},
			depServices: []depService{
				{
					serviceID:       "banana",
					interdependence: true,
				},
				{
					serviceID:       "bag",
					interdependence: false,
				},
			},
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			dbm := db.NewMockManager(ctrl)

			relationDao := dao.NewMockTenantServiceRelationDao(ctrl)
			for _, depService := range tc.services {
				var relations []*model.TenantServiceRelation
				for _, depServiceID := range depService.depServiceIDs {
					relations = append(relations, &model.TenantServiceRelation{
						ServiceID:       depService.serviceID,
						DependServiceID: depServiceID,
					})
				}
				relationDao.EXPECT().GetTenantServiceRelations(depService.serviceID).Return(relations, nil)
			}

			dbm.EXPECT().TenantServiceRelationDao().Return(relationDao).AnyTimes()

			detector := newStartupSequenceDetector(tc.serviceID, dbm)
			for _, depService := range tc.depServices {
				got, err := detector.dependOnEachOther(depService.serviceID)
				if err != nil {
					t.Errorf("Unexpected err: %v", err)
				}
				if depService.interdependence != got {
					t.Errorf("Expected %v for interdependence, but got %v", depService.interdependence, got)
				}
			}
		})
	}
}
