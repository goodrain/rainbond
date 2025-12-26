package controller

import (
	"fmt"
	"mime/multipart"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	DefaultChunkSize     = 5 * 1024 * 1024  // 5MB
	MinChunkSize         = 1 * 1024 * 1024  // 1MB
	MaxChunkSize         = 20 * 1024 * 1024 // 20MB
	SessionExpiryHours   = 24                // 24小时过期
	MaxConcurrentUploads = 10                // 最大并发上传数
)

// ChunkUploadManager 分片上传管理器
type ChunkUploadManager struct {
	sessionCache    map[string]*model.UploadSession
	cacheMutex      sync.RWMutex
	uploadSemaphore chan struct{}
	cleanupTicker   *time.Ticker
}

var chunkUploadManager *ChunkUploadManager
var chunkUploadOnce sync.Once

// GetChunkUploadManager 获取分片上传管理器单例
func GetChunkUploadManager() *ChunkUploadManager {
	chunkUploadOnce.Do(func() {
		chunkUploadManager = &ChunkUploadManager{
			sessionCache:    make(map[string]*model.UploadSession),
			uploadSemaphore: make(chan struct{}, MaxConcurrentUploads),
			cleanupTicker:   time.NewTicker(1 * time.Hour),
		}
		// 启动清理协程
		go chunkUploadManager.startCleanupWorker()
	})
	return chunkUploadManager
}

// InitUploadSession 初始化上传会话
func (m *ChunkUploadManager) InitUploadSession(eventID, fileName string, fileSize int64, fileMD5 string, chunkSize int) (*model.UploadSession, error) {
	// 验证分片大小
	if chunkSize == 0 {
		chunkSize = DefaultChunkSize
	}
	if chunkSize < MinChunkSize || chunkSize > MaxChunkSize {
		return nil, fmt.Errorf("chunk size must be between %d and %d", MinChunkSize, MaxChunkSize)
	}

	// 检查是否已存在未完成的上传会话
	existingSession, err := db.GetManager().UploadSessionDao().GetByEventIDAndFileName(eventID, fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing session: %v", err)
	}

	if existingSession != nil && existingSession.Status == "uploading" {
		// 返回现有会话（支持断点续传）
		logrus.Infof("Resume existing upload session: %s", existingSession.ID)
		m.cacheSession(existingSession)
		return existingSession, nil
	}

	// 创建新的上传会话
	sessionID := uuid.New().String()
	totalChunks := int((fileSize + int64(chunkSize) - 1) / int64(chunkSize))

	session := &model.UploadSession{
		ID:             sessionID,
		EventID:        eventID,
		FileName:       fileName,
		FileSize:       fileSize,
		FileMD5:        fileMD5,
		ChunkSize:      chunkSize,
		TotalChunks:    totalChunks,
		UploadedChunks: "",
		Status:         "uploading",
		StoragePath:    fmt.Sprintf("/grdata/package_build/temp/events/%s/%s", eventID, fileName),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(SessionExpiryHours * time.Hour),
	}

	if err := db.GetManager().UploadSessionDao().AddModel(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	m.cacheSession(session)
	logrus.Infof("Created new upload session: %s, total chunks: %d", sessionID, totalChunks)
	return session, nil
}

// SaveChunk 保存分片
func (m *ChunkUploadManager) SaveChunk(sessionID string, chunkIndex int, reader multipart.File) error {
	// 获取会话
	session, err := m.getSession(sessionID)
	if err != nil {
		return err
	}

	// 验证会话状态
	if session.Status != "uploading" {
		return fmt.Errorf("session status is %s, cannot upload", session.Status)
	}

	// 验证分片索引
	if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
		return fmt.Errorf("invalid chunk index: %d, total chunks: %d", chunkIndex, session.TotalChunks)
	}

	// 检查分片是否已存在（幂等性）
	if storage.Default().StorageCli.ChunkExists(sessionID, chunkIndex) {
		logrus.Debugf("Chunk %d already exists for session %s, skipping", chunkIndex, sessionID)
		return nil
	}

	// 并发控制
	m.uploadSemaphore <- struct{}{}
	defer func() { <-m.uploadSemaphore }()

	// 保存分片
	_, err = storage.Default().StorageCli.SaveChunk(sessionID, chunkIndex, reader)
	if err != nil {
		return fmt.Errorf("failed to save chunk: %v", err)
	}

	// 更新已上传分片列表
	if err := m.updateUploadedChunks(session, chunkIndex); err != nil {
		return err
	}

	logrus.Infof("Saved chunk %d/%d for session %s", chunkIndex+1, session.TotalChunks, sessionID)
	return nil
}

// CompleteUpload 完成上传，合并所有分片
func (m *ChunkUploadManager) CompleteUpload(sessionID string) (string, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return "", err
	}

	// 验证所有分片是否已上传
	uploadedChunks := m.parseUploadedChunks(session.UploadedChunks)
	if len(uploadedChunks) != session.TotalChunks {
		missingChunks := m.getMissingChunks(uploadedChunks, session.TotalChunks)
		return "", fmt.Errorf("not all chunks uploaded, missing: %v", missingChunks)
	}

	// 合并分片
	logrus.Infof("Merging %d chunks for session %s", session.TotalChunks, sessionID)
	err = storage.Default().StorageCli.MergeChunks(sessionID, session.StoragePath, session.TotalChunks)
	if err != nil {
		session.Status = "failed"
		db.GetManager().UploadSessionDao().UpdateModel(session)
		return "", fmt.Errorf("failed to merge chunks: %v", err)
	}

	// 清理分片文件
	if err := storage.Default().StorageCli.CleanupChunks(sessionID); err != nil {
		logrus.Warnf("Failed to cleanup chunks for session %s: %v", sessionID, err)
	}

	// 更新会话状态
	session.Status = "completed"
	session.UpdatedAt = time.Now()
	if err := db.GetManager().UploadSessionDao().UpdateModel(session); err != nil {
		logrus.Errorf("Failed to update session status: %v", err)
	}

	m.removeFromCache(sessionID)
	logrus.Infof("Upload completed for session %s, file: %s", sessionID, session.StoragePath)
	return session.StoragePath, nil
}

// GetUploadStatus 获取上传状态
func (m *ChunkUploadManager) GetUploadStatus(sessionID string) (*UploadStatusResponse, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	uploadedChunks := m.parseUploadedChunks(session.UploadedChunks)
	missingChunks := m.getMissingChunks(uploadedChunks, session.TotalChunks)
	progress := float64(len(uploadedChunks)) / float64(session.TotalChunks) * 100

	return &UploadStatusResponse{
		SessionID:      session.ID,
		FileName:       session.FileName,
		FileSize:       session.FileSize,
		ChunkSize:      session.ChunkSize,
		TotalChunks:    session.TotalChunks,
		UploadedChunks: uploadedChunks,
		MissingChunks:  missingChunks,
		Progress:       progress,
		Status:         session.Status,
		CreatedAt:      session.CreatedAt,
		UpdatedAt:      session.UpdatedAt,
	}, nil
}

// CancelUpload 取消上传
func (m *ChunkUploadManager) CancelUpload(sessionID string) error {
	_, err := m.getSession(sessionID)
	if err != nil {
		return err
	}

	// 清理分片文件
	if err := storage.Default().StorageCli.CleanupChunks(sessionID); err != nil {
		logrus.Warnf("Failed to cleanup chunks: %v", err)
	}

	// 删除会话记录
	if err := db.GetManager().UploadSessionDao().DeleteByID(sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %v", err)
	}

	m.removeFromCache(sessionID)
	logrus.Infof("Cancelled upload session: %s", sessionID)
	return nil
}

// 内部方法

// getSession 获取会话（优先从缓存）
func (m *ChunkUploadManager) getSession(sessionID string) (*model.UploadSession, error) {
	// 先从缓存获取
	m.cacheMutex.RLock()
	if session, ok := m.sessionCache[sessionID]; ok {
		m.cacheMutex.RUnlock()
		return session, nil
	}
	m.cacheMutex.RUnlock()

	// 从数据库获取
	session, err := db.GetManager().UploadSessionDao().GetByID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %v", err)
	}

	m.cacheSession(session)
	return session, nil
}

// cacheSession 缓存会话
func (m *ChunkUploadManager) cacheSession(session *model.UploadSession) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	m.sessionCache[session.ID] = session
}

// removeFromCache 从缓存移除
func (m *ChunkUploadManager) removeFromCache(sessionID string) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	delete(m.sessionCache, sessionID)
}

// updateUploadedChunks 更新已上传的分片列表
func (m *ChunkUploadManager) updateUploadedChunks(session *model.UploadSession, chunkIndex int) error {
	uploadedChunks := m.parseUploadedChunks(session.UploadedChunks)

	// 检查是否已记录
	for _, idx := range uploadedChunks {
		if idx == chunkIndex {
			return nil // 已存在
		}
	}

	// 添加新的分片索引
	uploadedChunks = append(uploadedChunks, chunkIndex)
	sort.Ints(uploadedChunks)

	// 转换为字符串
	strChunks := make([]string, len(uploadedChunks))
	for i, idx := range uploadedChunks {
		strChunks[i] = strconv.Itoa(idx)
	}
	session.UploadedChunks = strings.Join(strChunks, ",")
	session.UpdatedAt = time.Now()

	// 更新数据库
	if err := db.GetManager().UploadSessionDao().UpdateModel(session); err != nil {
		return fmt.Errorf("failed to update session: %v", err)
	}

	return nil
}

// parseUploadedChunks 解析已上传的分片列表
func (m *ChunkUploadManager) parseUploadedChunks(uploadedChunksStr string) []int {
	if uploadedChunksStr == "" {
		return []int{}
	}

	parts := strings.Split(uploadedChunksStr, ",")
	chunks := make([]int, 0, len(parts))
	for _, part := range parts {
		if idx, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			chunks = append(chunks, idx)
		}
	}
	return chunks
}

// getMissingChunks 获取缺失的分片列表
func (m *ChunkUploadManager) getMissingChunks(uploadedChunks []int, totalChunks int) []int {
	uploaded := make(map[int]bool)
	for _, idx := range uploadedChunks {
		uploaded[idx] = true
	}

	missing := make([]int, 0)
	for i := 0; i < totalChunks; i++ {
		if !uploaded[i] {
			missing = append(missing, i)
		}
	}
	return missing
}

// startCleanupWorker 启动定时清理任务
func (m *ChunkUploadManager) startCleanupWorker() {
	for range m.cleanupTicker.C {
		if err := m.cleanExpiredSessions(); err != nil {
			logrus.Errorf("Failed to clean expired sessions: %v", err)
		}
	}
}

// cleanExpiredSessions 清理过期的会话
func (m *ChunkUploadManager) cleanExpiredSessions() error {
	if err := db.GetManager().UploadSessionDao().CleanExpiredSessions(); err != nil {
		return err
	}

	// 清理缓存中的过期会话
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	now := time.Now()
	for id, session := range m.sessionCache {
		if session.ExpiresAt.Before(now) {
			// 清理分片文件
			storage.Default().StorageCli.CleanupChunks(id)
			delete(m.sessionCache, id)
		}
	}

	logrus.Debug("Cleaned up expired upload sessions")
	return nil
}

// UploadStatusResponse 上传状态响应
type UploadStatusResponse struct {
	SessionID      string    `json:"session_id"`
	FileName       string    `json:"file_name"`
	FileSize       int64     `json:"file_size"`
	ChunkSize      int       `json:"chunk_size"`
	TotalChunks    int       `json:"total_chunks"`
	UploadedChunks []int     `json:"uploaded_chunks"`
	MissingChunks  []int     `json:"missing_chunks"`
	Progress       float64   `json:"progress"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
