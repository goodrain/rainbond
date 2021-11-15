package conversion

import (
	"fmt"
	"github.com/goodrain/rainbond/db"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TenantServiceConfigGroup -
func TenantServiceConfigGroup(as *v1.AppService, dbm db.Manager) error {
	logrus.Infof("service id: %s; create config group for service.", as.ServiceID)
	groups, err := dbm.AppConfigGroupDao().ListByServiceID(as.ServiceID)
	if err != nil {
		return fmt.Errorf("[TenantServiceConfigGroup] list config groups: %v", err)
	}

	var secrets []*corev1.Secret
	for _, group := range groups {
		cg := createConfigGroup(as, as.GetNamespace(), group.AppID, group.ConfigGroupName)
		secret, err := cg.secretForConfigGroup()
		if err != nil {
			return fmt.Errorf("create secret for config group: %v", err)
		}
		secrets = append(secrets, secret)
	}

	as.SetEnvVarSecrets(secrets)

	return nil
}

type configGroup struct {
	as *v1.AppService

	namespace       string
	appID           string
	configGroupName string
}

func createConfigGroup(as *v1.AppService, ns, appID, configGroupName string) *configGroup {
	return &configGroup{
		as:              as,
		namespace:       ns,
		appID:           appID,
		configGroupName: configGroupName,
	}
}

func (c *configGroup) secretForConfigGroup() (*corev1.Secret, error) {
	items, err := db.GetManager().AppConfigGroupItemDao().GetConfigGroupItemsByID(c.appID, c.configGroupName)
	if err != nil {
		return nil, err
	}

	labels := c.as.GetCommonLabels()
	delete(labels, "service_id")
	delete(labels, "service_alias")

	data := make(map[string][]byte)
	for _, item := range items {
		data[item.ItemKey] = []byte(item.ItemValue)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", c.configGroupName, c.appID),
			Namespace: c.namespace,
			Labels:    labels,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}, nil
}
