package conversion

import (
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

type hostNetworkTestManager struct {
	db.Manager
	attributeDao dbdao.ComponentK8sAttributeDao
}

func (m hostNetworkTestManager) ComponentK8sAttributeDao() dbdao.ComponentK8sAttributeDao {
	return m.attributeDao
}

type hostNetworkAttributeDao struct {
	dbdao.ComponentK8sAttributeDao
	attributes map[string]*dbmodel.ComponentK8sAttributes
	err        error
}

func (d hostNetworkAttributeDao) GetByComponentIDAndName(_, name string) (*dbmodel.ComponentK8sAttributes, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.attributes[name], nil
}

func TestCreateHostNetworkUsesK8sAttribute(t *testing.T) {
	tests := []struct {
		name         string
		extensionSet map[string]string
		attribute    *dbmodel.ComponentK8sAttributes
		daoErr       error
		want         bool
	}{
		{
			name: "attribute true enables host network",
			attribute: &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameHostNetwork,
				AttributeValue: "true",
			},
			want: true,
		},
		{
			name:         "attribute false overrides legacy extension",
			extensionSet: map[string]string{"hostnetwork": ""},
			attribute: &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameHostNetwork,
				AttributeValue: "false",
			},
			want: false,
		},
		{
			name:         "legacy extension still enables host network",
			extensionSet: map[string]string{"hostnetwork": ""},
			want:         true,
		},
		{
			name: "invalid attribute value disables host network",
			attribute: &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameHostNetwork,
				AttributeValue: "invalid",
			},
			want: false,
		},
		{
			name:   "dao error disables host network",
			daoErr: errors.New("lookup failed"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &typesv1.AppService{
				AppServiceBase: typesv1.AppServiceBase{
					ServiceID:    "service-1",
					ExtensionSet: tt.extensionSet,
				},
			}
			attributes := map[string]*dbmodel.ComponentK8sAttributes{}
			if tt.attribute != nil {
				attributes[tt.attribute.Name] = tt.attribute
			}
			manager := hostNetworkTestManager{
				attributeDao: hostNetworkAttributeDao{
					attributes: attributes,
					err:        tt.daoErr,
				},
			}

			if got := createHostNetwork(app, manager); got != tt.want {
				t.Fatalf("createHostNetwork() = %v, want %v", got, tt.want)
			}
		})
	}
}
