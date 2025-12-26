package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// ChunkUploadController 分片上传控制器
type ChunkUploadController struct{}

// InitUpload 初始化上传会话
// POST /package_build/component/events/{eventID}/upload/init
func (c *ChunkUploadController) InitUpload(w http.ResponseWriter, r *http.Request) {
	eventID := strings.TrimSpace(chi.URLParam(r, "eventID"))
	if eventID == "" {
		httputil.ReturnError(r, w, 400, "eventID is required")
		return
	}

	// 解析请求参数
	var req struct {
		FileName  string `json:"file_name" validate:"required"`
		FileSize  int64  `json:"file_size" validate:"required"`
		FileMD5   string `json:"file_md5"`
		ChunkSize int    `json:"chunk_size"`
	}

	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}

	// 初始化上传会话
	manager := GetChunkUploadManager()
	session, err := manager.InitUploadSession(eventID, req.FileName, req.FileSize, req.FileMD5, req.ChunkSize)
	if err != nil {
		logrus.Errorf("Failed to init upload session: %v", err)
		httputil.ReturnError(r, w, 500, "Failed to initialize upload: "+err.Error())
		return
	}

	// 解析已上传的分片
	uploadedChunks := manager.parseUploadedChunks(session.UploadedChunks)

	response := map[string]interface{}{
		"session_id":      session.ID,
		"chunk_size":      session.ChunkSize,
		"total_chunks":    session.TotalChunks,
		"uploaded_chunks": uploadedChunks,
		"status":          session.Status,
	}
	origin := r.Header.Get("Origin")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
	logrus.Infof("Initialized upload session: %s for event: %s, file: %s", session.ID, eventID, req.FileName)
	httputil.ReturnSuccess(r, w, response)
}

// UploadChunk 上传分片
// POST /package_build/component/events/{eventID}/upload/chunk
func (c *ChunkUploadController) UploadChunk(w http.ResponseWriter, r *http.Request) {
	eventID := strings.TrimSpace(chi.URLParam(r, "eventID"))
	if eventID == "" {
		httputil.ReturnError(r, w, 400, "eventID is required")
		return
	}

	// 解析表单参数
	sessionID := r.FormValue("session_id")
	chunkIndexStr := r.FormValue("chunk_index")

	if sessionID == "" || chunkIndexStr == "" {
		httputil.ReturnError(r, w, 400, "session_id and chunk_index are required")
		return
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, "invalid chunk_index")
		return
	}

	// 获取上传的文件
	reader, header, err := r.FormFile("file")
	if err != nil {
		logrus.Errorf("Failed to parse chunk file: %v", err)
		httputil.ReturnError(r, w, 400, "Failed to parse chunk file")
		return
	}
	defer reader.Close()

	logrus.Debugf("Receiving chunk %d for session %s, size: %d", chunkIndex, sessionID, header.Size)

	// 保存分片
	manager := GetChunkUploadManager()
	if err := manager.SaveChunk(sessionID, chunkIndex, reader); err != nil {
		logrus.Errorf("Failed to save chunk: %v", err)
		httputil.ReturnError(r, w, 500, "Failed to save chunk: "+err.Error())
		return
	}

	// 获取最新状态
	status, err := manager.GetUploadStatus(sessionID)
	if err != nil {
		logrus.Errorf("Failed to get upload status: %v", err)
		httputil.ReturnError(r, w, 500, "Failed to get status")
		return
	}

	response := map[string]interface{}{
		"session_id":      sessionID,
		"chunk_index":     chunkIndex,
		"received_size":   header.Size,
		"uploaded_chunks": status.UploadedChunks,
		"progress":        status.Progress,
	}
	origin := r.Header.Get("Origin")

	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
	httputil.ReturnSuccess(r, w, response)
}

// CompleteUpload 完成上传，合并分片
// POST /package_build/component/events/{eventID}/upload/complete
func (c *ChunkUploadController) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	eventID := strings.TrimSpace(chi.URLParam(r, "eventID"))
	if eventID == "" {
		httputil.ReturnError(r, w, 400, "eventID is required")
		return
	}

	// 解析请求参数
	var req struct {
		SessionID string `json:"session_id" validate:"required"`
	}

	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}

	// 完成上传
	manager := GetChunkUploadManager()
	filePath, err := manager.CompleteUpload(req.SessionID)
	if err != nil {
		logrus.Errorf("Failed to complete upload: %v", err)
		httputil.ReturnError(r, w, 500, "Failed to complete upload: "+err.Error())
		return
	}

	response := map[string]interface{}{
		"file_path": filePath,
		"status":    "completed",
	}

	logrus.Infof("Upload completed for session: %s, file: %s", req.SessionID, filePath)
	origin := r.Header.Get("Origin")

	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
	httputil.ReturnSuccess(r, w, response)
}

// GetUploadStatus 查询上传状态
// GET /package_build/component/events/{eventID}/upload/status/{sessionID}
func (c *ChunkUploadController) GetUploadStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		httputil.ReturnError(r, w, 400, "sessionID is required")
		return
	}

	manager := GetChunkUploadManager()
	status, err := manager.GetUploadStatus(sessionID)
	if err != nil {
		logrus.Errorf("Failed to get upload status: %v", err)
		httputil.ReturnError(r, w, 404, "Session not found")
		return
	}
	origin := r.Header.Get("Origin")

	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
	httputil.ReturnSuccess(r, w, status)
}

// CancelUpload 取消上传
// DELETE /package_build/component/events/{eventID}/upload/{sessionID}
func (c *ChunkUploadController) CancelUpload(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if sessionID == "" {
		httputil.ReturnError(r, w, 400, "sessionID is required")
		return
	}

	manager := GetChunkUploadManager()
	if err := manager.CancelUpload(sessionID); err != nil {
		logrus.Errorf("Failed to cancel upload: %v", err)
		httputil.ReturnError(r, w, 500, "Failed to cancel upload: "+err.Error())
		return
	}

	logrus.Infof("Cancelled upload session: %s", sessionID)
	origin := r.Header.Get("Origin")

	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
	httputil.ReturnSuccess(r, w, map[string]string{"message": "Upload cancelled"})
}

// HandleOptions 处理 CORS 预检请求
func (c *ChunkUploadController) HandleOptions(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Custom-Header, X_TEAM_NAME, X_REGION_NAME")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "3600")
	w.WriteHeader(http.StatusOK)
}
