package controller

import (
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/bitly/go-simplejson"
	"github.com/goodrain/rainbond/pkg/db"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
)

//Event GetLogs
func (e *TenantStruct) Event(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET  /v2/tenants/{tenant_name}/event v2 getevents
	//
	// 获取指定event_ids详细信息
	//
	// get events
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	b, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	j, err := simplejson.NewJson(b)
	if err != nil {
		logrus.Errorf("error decode json,details %s", err.Error())
		httputil.ReturnError(r, w, 400, "bad request")
		return
	}
	eventIDS, err := j.Get("event_ids").StringArray()
	if err != nil {
		logrus.Errorf("error get event_id in json,details %s", err.Error())
		httputil.ReturnError(r, w, 400, "bad request")
		return
	}
	serviceEvents, err := db.GetManager().ServiceEventDao().GetEventByEventIDs(eventIDS)
	if err != nil {
		logrus.Warnf("can't find event by given id ,details %s", err.Error())
		httputil.ReturnError(r, w, 500, err.Error())
	}
	httputil.ReturnSuccess(r, w, serviceEvents)
}
