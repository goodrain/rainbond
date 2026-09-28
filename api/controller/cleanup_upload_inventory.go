package controller

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/component/storage"
	httputil "github.com/goodrain/rainbond/util/http"
)

var uploadInventoryEventID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type uploadInventoryItem struct {
	ID         string    `json:"id"`
	EventID    string    `json:"event_id"`
	FileName   string    `json:"file_name"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Bytes      *int64    `json:"bytes"`
	Objects    *int      `json:"objects"`
	SizeStatus string    `json:"size_status"`
}

func systemMeasureUploadChunks(ctx context.Context, id string) (storage.UploadChunkUsage, error) {
	configured := storage.Default()
	if configured == nil {
		return storage.UploadChunkUsage{}, guard.ErrCoordinationUnavailable
	}
	measurer, ok := configured.StorageCli.(interface {
		MeasureUploadChunks(context.Context, string) (storage.UploadChunkUsage, error)
	})
	if !ok {
		return storage.UploadChunkUsage{}, guard.ErrCoordinationUnavailable
	}
	return measurer.MeasureUploadChunks(ctx, id)
}

// UploadInventory observes chunks of explicitly requested upload events. It is
// internal to the trusted Console and requires CleanupIdentity on its route.
// Console must derive event IDs from its own enterprise/team records. Neither
// expiration nor this measurement proves that an upload is safe to delete.
func (h *CleanupCoordinationHandler) UploadInventory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EventIDs []string `json:"event_ids"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	seen := map[string]bool{}
	if len(body.EventIDs) == 0 || len(body.EventIDs) > 50 {
		httputil.ReturnError(r, w, 400, "INVALID_UPLOAD_SCOPE")
		return
	}
	for _, id := range body.EventIDs {
		if !uploadInventoryEventID.MatchString(id) || seen[id] {
			httputil.ReturnError(r, w, 400, "INVALID_UPLOAD_SCOPE")
			return
		}
		seen[id] = true
	}
	if h.database == nil || h.measureUploadChunks == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	database := h.database()
	if database == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	rows := []model.UploadSession{}
	if err := database.Select("id, event_id, file_name, status, created_at, updated_at, expires_at").Where("event_id IN (?)", body.EventIDs).Order("id ASC").Limit(501).Find(&rows).Error; err != nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	if len(rows) > 500 {
		httputil.ReturnError(r, w, 413, "UPLOAD_INVENTORY_LIMIT")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	items := make([]uploadInventoryItem, 0, len(rows))
	for _, row := range rows {
		item := uploadInventoryItem{ID: row.ID, EventID: row.EventID, FileName: row.FileName, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ExpiresAt: row.ExpiresAt, SizeStatus: "unavailable"}
		if ctx.Err() == nil {
			usage, err := h.measureUploadChunks(ctx, row.ID)
			if err == nil && usage.Bytes >= 0 && usage.Objects >= 0 {
				item.Bytes = &usage.Bytes
				item.Objects = &usage.Objects
				item.SizeStatus = "measured"
			}
		}
		items = append(items, item)
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int                   `json:"protocol"`
		Scope    string                `json:"scope"`
		Items    []uploadInventoryItem `json:"items"`
	}{1, "upload_chunks", items})
}
