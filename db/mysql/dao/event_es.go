package dao

import (
	"context"
	"encoding/json"
	"fmt"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/es"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"time"
)

type EventDaoESImpl struct {
	DB *gorm.DB
}

// AddModel AddModel
func (c *EventDaoESImpl) AddModel(mo model.Interface) error {
	result := mo.(*model.ServiceEvent)
	body, _ := json.Marshal(result)
	_, err := es.Default().POST(fmt.Sprintf("/appstore_tenant_services_event/_doc/%s", result.EventID), string(body))
	if err != nil {
		logrus.Errorf("eventDaoImpl addModel error: %s", err.Error())
	}
	return err
}

// UpdateModel UpdateModel
func (c *EventDaoESImpl) UpdateModel(mo model.Interface) error {
	update := mo.(*model.ServiceEvent)
	body, _ := json.Marshal(update)
	_, err := es.Default().PUT(fmt.Sprintf("/appstore_tenant_services_event/_doc/%s", update.EventID), string(body))
	return err
}

// DeleteModel DeleteModel
func (c *EventDaoESImpl) DeleteModel(id string, args ...interface{}) error {
	_, err := es.Default().DELETE(fmt.Sprintf("/appstore_tenant_services_event/_doc/%s", id))
	return err
}

// CreateEventsInBatch creates events in batch.
func (c *EventDaoESImpl) CreateEventsInBatch(events []*model.ServiceEvent) error {
	for _, event := range events {
		_ = c.AddModel(event)
	}
	return nil
}

// DeleteEvents delete event
func (c *EventDaoESImpl) DeleteEvents(eventIDs []string) error {
	eventIds, _ := json.Marshal(eventIDs)
	query := fmt.Sprintf(`
    {
      "query": {
        "terms": {
          "event_id": %s
        }
      }
    }`, string(eventIds))
	_, err := es.Default().POST("/appstore_tenant_services_event/_delete_by_query", query)
	return err
}

// UpdateReason update reasion.
func (c *EventDaoESImpl) UpdateReason(eventID string, reason string) error {
	body := fmt.Sprintf(`{
    "script": {
        "source": "ctx._source.reason = params.reason",
        "params": {
            "reason": "%s"
        }
    },
    "query": {
        "term": {
            "event_id": "%s"
        }
    }
}`, reason, eventID)
	_, err := es.Default().POST("/appstore_tenant_services_event/_update_by_query", body)
	return err
}

// GetEventByEventID get event log message
func (c *EventDaoESImpl) GetEventByEventID(eventID string) (*model.ServiceEvent, error) {
	var result model.ServiceEvent

	get, err := es.Default().GET(fmt.Sprintf("/appstore_tenant_services_event/_doc/%s", eventID))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(gjson.Get(get, "_source").Raw), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEventByEventIDs get event info
func (c *EventDaoESImpl) GetEventByEventIDs(eventIDs []string) ([]*model.ServiceEvent, error) {
	eventIds, _ := json.Marshal(eventIDs)
	query := fmt.Sprintf(`
{
    "query": {
        "terms": {
            "event_id": %s
        }
    }
}`, eventIds)
	return c.array(query)
}

func (c *EventDaoESImpl) array(query string) ([]*model.ServiceEvent, error) {
	get, err := es.Default().POST("/appstore_tenant_services_event/_search", query)
	if err != nil {
		return nil, err
	}
	var result []*model.ServiceEvent
	err = json.Unmarshal([]byte(gjson.Get(get, "hits.hits").Raw), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateInBatch -
func (c *EventDaoESImpl) UpdateInBatch(events []*model.ServiceEvent) error {
	for i := range events {
		_ = c.UpdateModel(events[i])
	}
	return nil
}

// GetEventByServiceID get event log message
func (c *EventDaoESImpl) GetEventByServiceID(serviceID string) ([]*model.ServiceEvent, error) {
	body := fmt.Sprintf(`{
  "query": {
    "match": {
      "service_id": "%s"
    }
  },
  "sort": [
    {
      "start_time": {
        "order": "desc"
      }
    }
  ]
}`, serviceID)
	return c.array(body)
}

// DelEventByServiceID delete event log
func (c *EventDaoESImpl) DelEventByServiceID(serviceID string) error {
	query := fmt.Sprintf(`
    {
      "query": {
        "match": {
          "service_id": "%s"
        }
      }
    }`, serviceID)
	_, err := es.Default().POST("/appstore_tenant_services_event/_delete_by_query", query)
	return err
}

// ListByTargetID -
func (c *EventDaoESImpl) ListByTargetID(targetID string) ([]*model.ServiceEvent, error) {
	query := fmt.Sprintf(`
    {
      "query": {
        "match": {
          "target_id": "%s"
        }
      }
    }`, targetID)
	return c.array(query)
}

// GetEventsByTarget get event by target with page
func (c *EventDaoESImpl) GetEventsByTarget(target, targetID string, offset, limit int) ([]*model.ServiceEvent, int, error) {
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "match": { "target": "%s" } },
        { "match": { "target_id": "%s" } }
      ]
    }
  },
  "sort": [
    { "create_time": { "order": "desc" } },
    { "id": { "order": "desc" } }
  ],
  "from": %d, 
  "size": %d 
}
`, target, targetID, offset, limit)

	array, err := c.array(body)
	if err != nil {
		return nil, 0, err
	}
	post, _ := es.Default().POST("/appstore_tenant_services_event/_count", body)

	return array, (int)(gjson.Get(post, "count").Int()), nil
}

// GetEventsByTenantID get event by tenantID
func (c *EventDaoESImpl) GetEventsByTenantID(tenantID string, offset, limit int) ([]*model.ServiceEvent, int, error) {
	query := fmt.Sprintf(`
    {
           "query": {
             "match": {
               "tenant_id": "%s"
             }
           },
           "sort": [
             {"start_time": "desc"},
             {"id": "desc"}
           ],
           "from": %d,
           "size": %d
         }`, tenantID, offset, limit)

	post, _ := es.Default().POST("/appstore_tenant_services_event/_count", query)
	count := (int)(gjson.Get(post, "count").Int())

	array, err := c.array(query)
	if err != nil {
		return nil, 0, err
	}
	return array, count, nil
}

// GetEventsByTenantIDs get my teams all event by tenantIDs
func (c *EventDaoESImpl) GetEventsByTenantIDs(tenantIDs []string, offset, limit int) ([]*model.EventAndBuild, error) {
	tenants, _ := json.Marshal(tenantIDs)
	body := fmt.Sprintf(`{
  "sort": [
    { "id": "desc" }
  ],
  "from": %d, 
  "size": %d,
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "target": "service"
          }
        },
        {
          "terms": {
            "tenant_id": %s
          }
        }
      ]
    }
  }
}`, offset, limit, string(tenants))
	array, err := c.array(body)
	if err != nil {
		return nil, err
	}
	res := make([]*model.EventAndBuild, 0)
	for _, item := range array {

		var version model.VersionInfo
		if err := c.DB.Where("event_id=?", item.EventID).Find(&version).Error; err != nil {
			e := &model.EventAndBuild{
				CreateTime:  item.CreatedAt.Format("2006-01-02 15:04:05"),
				TenantID:    item.TenantID,
				Target:      item.Target,
				TargetID:    item.TargetID,
				UserName:    item.UserName,
				StartTime:   item.StartTime,
				EndTime:     item.EndTime,
				OptType:     item.OptType,
				SynType:     string(rune(item.SynType)),
				Status:      item.Status,
				FinalStatus: item.FinalStatus,
				Message:     item.Message,
				Reason:      item.Reason,
			}
			res = append(res, e)

		} else {
			e := &model.EventAndBuild{
				CreateTime:       item.CreatedAt.Format("2006-01-02 15:04:05"),
				TenantID:         item.TenantID,
				Target:           item.Target,
				TargetID:         item.TargetID,
				UserName:         item.UserName,
				StartTime:        item.StartTime,
				EndTime:          item.EndTime,
				OptType:          item.OptType,
				SynType:          string(rune(item.SynType)),
				Status:           item.Status,
				FinalStatus:      item.FinalStatus,
				Message:          item.Message,
				Reason:           item.Reason,
				BuildVersion:     version.BuildVersion,
				Kind:             version.Kind,
				DeliveredType:    version.DeliveredType,
				DeliveredPath:    version.DeliveredPath,
				ImageName:        version.ImageName,
				Cmd:              version.Cmd,
				RepoURL:          version.RepoURL,
				CodeVersion:      version.CodeVersion,
				CodeBranch:       version.CodeBranch,
				CodeCommitMsg:    version.CommitMsg,
				CodeCommitAuthor: version.Author,
				PlanVersion:      version.PlanVersion,
			}
			res = append(res, e)
		}
	}

	return res, nil
}

// GetLastASyncEvent get last sync event
func (c *EventDaoESImpl) GetLastASyncEvent(target, targetID string) (*model.ServiceEvent, error) {

	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "match": { "target": "%s" }},
        { "match": { "target_id": "%s" }},
        { "term": { "syn_type": 0 }}
      ]
    }
  },
  "sort": [
    { "id": "desc" }
  ],
  "size": 1
}`, target, targetID)
	array, err := c.array(body)
	if err != nil || len(array) == 0 {
		return nil, err
	}
	return array[0], nil
}

// UnfinishedEvents returns unfinished events.
func (c *EventDaoESImpl) UnfinishedEvents(target, targetID string, optTypes ...string) ([]*model.ServiceEvent, error) {
	op, _ := json.Marshal(optTypes)
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "match": { "target": "%s" }},
        { "match": { "target_id": "%s" }},
        { "match": { "status": "%s" }}
      ],
      "filter": [
        { "terms": { "opt_type": %s }}
      ]
    }
  }
}`, target, targetID, model.EventStatusFailure.String(), string(op))
	return c.array(body)
}

// LatestFailurePodEvent returns the latest failure pod event.
func (c *EventDaoESImpl) LatestFailurePodEvent(podName string) (*model.ServiceEvent, error) {
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "term": { "target": "%s" } },
        { "term": { "target_id": "%s" } },
        { "term": { "status": "%s" } },
        { "bool": {
            "must_not": { "term": { "final_status": "%s" } }
          }
        }
      ]
    }
  },
  "sort": [
    { "id": { "order": "desc" } }
  ],
  "size": 1
}`, model.TargetTypePod, podName, model.EventStatusFailure.String(), model.EventFinalStatusEmptyComplete.String())
	array, err := c.array(body)
	if err != nil {
		return nil, err
	}
	return array[0], nil
}

// GetAppointEvent get event log message
func (c *EventDaoESImpl) GetAppointEvent(serviceID, status, Opt string) (*model.ServiceEvent, error) {
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "term": { "service_id": "%s" } },
        { "term": { "status": "%s" } },
        { "term": { "opt_type": "%s" } }
      ]
    }
  },
  "sort": [
    { "id": { "order": "desc" } }
  ],
  "size": 1
}`, serviceID, status, Opt)
	array, err := c.array(body)
	if err != nil {
		return nil, err
	}
	return array[0], nil
}

// AbnormalEvent Abnormal event in components.
func (c *EventDaoESImpl) AbnormalEvent(serviceID, Opt string) (*model.ServiceEvent, error) {
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "term": { "target": "%s" } },
        { "term": { "service_id": "%s" } },
        { "term": { "opt_type": "%s" } },
        { "term": { "status": "%s" } }
      ]
    }
  },
  "sort": [
    { "id": { "order": "desc" } }
  ],
  "size": 1
}`, model.TargetTypePod, serviceID, Opt, model.EventStatusFailure.String())
	array, err := c.array(body)
	if err != nil {
		return nil, err
	}
	return array[0], nil
}

// DelAbnormalEvent delete Abnormal event in components.
func (c *EventDaoESImpl) DelAbnormalEvent(serviceID, Opt string) error {
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "term": { "target": "%s" } },
        { "term": { "service_id": "%s" } },
        { "term": { "opt_type": "%s" } },
        { "term": { "status": "%s" } }
      ]
    }
  }
}`, model.TargetTypePod, serviceID, Opt, model.EventStatusFailure.String())
	_, err := es.Default().POST("/appstore_tenant_services_event/_delete_by_query", body)
	if err != nil {
		return err
	}
	return nil
}

// DelAllAbnormalEvent delete all Abnormal event in components when stop.
func (c *EventDaoESImpl) DelAllAbnormalEvent(serviceID string, Opts []string) error {
	optsJson, _ := json.Marshal(Opts)
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "match": { "target": "%s" }},
        { "match": { "service_id": "%s" }},
        { "terms": { "opt_type": %s }},
        { "match": { "status": "%s" }}
      ]
    }
  }
}`, model.TargetTypePod, serviceID, string(optsJson), model.EventStatusFailure.String())
	_, err := es.Default().POST("/appstore_tenant_services_event/_delete_by_query", body)
	if err != nil {
		return err
	}
	return nil
}

// SetEventStatus -
func (c *EventDaoESImpl) SetEventStatus(ctx context.Context, status model.EventStatus) error {
	event, _ := ctx.Value(ctxutil.ContextKey("event")).(*model.ServiceEvent)
	if event != nil {
		event.FinalStatus = "complete"
		event.Status = string(status)
		return c.UpdateModel(event)
	}
	return nil
}

// GetExceptionEventsByTime -
func (c *EventDaoESImpl) GetExceptionEventsByTime(eventTypes []string, createTime time.Time) ([]*model.ServiceEvent, error) {
	eventTypesJson, _ := json.Marshal(eventTypes)
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "terms": { "opt_type": %s } },
        { "range": { "create_time": { "gt": "%s" } } }
      ]
    }
  }
}`, string(eventTypesJson), createTime.Format("2006-01-02 15:04:05"))
	return c.array(body)
}

// CountEvents -
func (c *EventDaoESImpl) CountEvents(tenantID, serviceID, eventType string) int64 {
	body := fmt.Sprintf(`{
  "query": {
    "bool": {
      "must": [
        { "term": { "tenant_id": "%s" } },
        { "term": { "service_id": "%s" } },
        { "term": { "opt_type": "%s" } }
      ]
    }
  }
}`, tenantID, serviceID, eventType)
	post, _ := es.Default().POST("/appstore_tenant_services_event/_count", body)
	return gjson.Get(post, "count").Int()
}
