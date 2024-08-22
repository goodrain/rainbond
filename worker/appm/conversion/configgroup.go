/*
本文件提供了与应用服务配置相关的功能，包括创建配置组和生成 Kubernetes Secret 对象。

主要功能：
1. `TenantServiceConfigGroup` 函数：为指定的应用服务创建配置组，并生成相应的 Kubernetes Secret。
   - 输入：应用服务对象 (`*v1.AppService`) 和数据库管理器 (`db.Manager`)。
   - 输出：可能返回的错误。

2. `createConfigGroup` 函数：根据应用服务、命名空间、应用 ID 和配置组名称创建一个配置组对象。
   - 输入：应用服务对象、命名空间、应用 ID 和配置组名称。
   - 输出：配置组对象 (`*configGroup`)。

3. `secretForConfigGroup` 方法：为配置组生成一个 Kubernetes Secret 对象。
   - 输入：无。
   - 输出：生成的 Secret 对象 (`*corev1.Secret`) 和可能发生的错误。

文件中使用的主要库：
- `fmt`：格式化输入输出。
- `logrus`：日志记录。
- `k8s.io/api/core/v1` 和 `k8s.io/apimachinery/pkg/apis/meta/v1`：Kubernetes API 类型和工具。
- `github.com/goodrain/rainbond/db` 和 `github.com/goodrain/rainbond/worker/appm/types/v1`：Rainbond 相关的数据库和类型定义。
*/

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
