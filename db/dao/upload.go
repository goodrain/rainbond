package dao

import (
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// UploadSessionDao 上传会话 DAO 接口
type UploadSessionDao interface {
	Dao
	GetByID(id string) (*model.UploadSession, error)
	GetByEventIDAndFileName(eventID, fileName string) (*model.UploadSession, error)
	ListByEventID(eventID string) ([]*model.UploadSession, error)
	DeleteByID(id string) error
	CleanExpiredSessions() error
}

// UploadSessionDaoImpl 上传会话 DAO 实现
type UploadSessionDaoImpl struct {
	DB *gorm.DB
}

// AddModel 添加上传会话
func (u *UploadSessionDaoImpl) AddModel(mo model.Interface) error {
	session := mo.(*model.UploadSession)
	var old model.UploadSession
	if err := u.DB.Where("id = ?", session.ID).Find(&old).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return u.DB.Create(session).Error
		}
		return err
	}
	return u.DB.Save(session).Error
}

// UpdateModel 更新上传会话
func (u *UploadSessionDaoImpl) UpdateModel(mo model.Interface) error {
	session := mo.(*model.UploadSession)
	return u.DB.Save(session).Error
}

// GetByID 根据 ID 获取上传会话
func (u *UploadSessionDaoImpl) GetByID(id string) (*model.UploadSession, error) {
	var session model.UploadSession
	if err := u.DB.Where("id = ?", id).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// GetByEventIDAndFileName 根据事件ID和文件名获取上传会话
func (u *UploadSessionDaoImpl) GetByEventIDAndFileName(eventID, fileName string) (*model.UploadSession, error) {
	var session model.UploadSession
	if err := u.DB.Where("event_id = ? AND file_name = ? AND status IN (?)",
		eventID, fileName, []string{"uploading", "completed"}).
		Order("created_at DESC").
		First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// ListByEventID 根据事件ID获取所有上传会话
func (u *UploadSessionDaoImpl) ListByEventID(eventID string) ([]*model.UploadSession, error) {
	var sessions []*model.UploadSession
	if err := u.DB.Where("event_id = ?", eventID).Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// DeleteByID 删除上传会话
func (u *UploadSessionDaoImpl) DeleteByID(id string) error {
	return u.DB.Where("id = ?", id).Delete(&model.UploadSession{}).Error
}

// CleanExpiredSessions 清理过期的上传会话
func (u *UploadSessionDaoImpl) CleanExpiredSessions() error {
	now := time.Now()
	return u.DB.Where("expires_at < ? AND status != ?", now, "completed").
		Delete(&model.UploadSession{}).Error
}
