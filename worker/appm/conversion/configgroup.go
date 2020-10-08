package conversion

import (
	"fmt"

	"github.com/goodrain/rainbond/db"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type configGroup struct {
	sid string
}

func CreateConfigGroup(sid string) *configGroup {
	return &configGroup{
		sid: sid,
	}
}

func (c *configGroup) createEnvs() ([]corev1.EnvVar, error) {
	// list all config group items
	items, err := db.GetManager().AppConfigGroupItemDao().ListByServiceID(c.sid)
	if err != nil {
		return nil, fmt.Errorf("list config group items: %v", err)
	}

	var envs []corev1.EnvVar
	logrus.Debugf("service id: %s; %d config group items were found", c.sid, len(items))
	for _, item := range items {
		addOrUpdateEnvs(&envs, item.ItemKey, item.ItemValue)
	}

	return envs, nil
}
