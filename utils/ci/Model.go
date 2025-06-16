package ci

import (
	"fmt"
	"gorm.io/gorm"
)

// Model 自定义基础模型，包含通用字段
type Model struct {
	gorm.Model
	TenantID string `gorm:"type:varchar(32);not null;column:tenant_id"`
}

// BeforeCreate 创建前从上下文中获取 TenantID
func (m *Model) BeforeCreate(tx *gorm.DB) error {
	// 从 GORM 事务的上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		m.TenantID = tenantID
	} else {
		// 处理 TenantID 未找到的情况，例如返回错误
		return fmt.Errorf("tenant ID not found in context")
	}
	return nil
}
