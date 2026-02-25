package ci

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Model 自定义基础模型，包含通用字段
// 不直接嵌入 gorm.Model，以便自定义 json tag
type Model struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	TenantID  string         `gorm:"type:varchar(32);not null;column:tenant_id" json:"tenant_id"`
}

// SetTenant 设置 TenantID，支持链式调用，用于异步任务中手动设置。
// 用法：record.SetTenant(tenantID).Create()
func (m *Model) SetTenant(tenantID string) *Model {
	m.TenantID = tenantID
	return m
}

// GetTenant 获取当前模型的 TenantID
func (m *Model) GetTenant() string {
	return m.TenantID
}

// BeforeCreate 创建前从上下文中获取 TenantID
// 支持三种方式：
//  1. 正常请求：自动从 context 获取
//  2. 异步任务：使用 ci.Go(c, func(db){...}) 自动传递
//  3. 手动设置：record.TenantID = "xxx" 后再 Create
func (m *Model) BeforeCreate(tx *gorm.DB) error {
	// 如果 TenantID 已经有值，跳过从 context 获取（支持手动预设）
	if m.TenantID != "" {
		return nil
	}

	// 从 GORM 事务的上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		m.TenantID = tenantID
	} else {
		return fmt.Errorf("【BeforeCreate】tenant ID not found in context")
	}
	return nil
}

// BeforeQuery 查询前从上下文中获取 TenantID 并添加到查询条件
// 支持三种方式：
//  1. 正常请求：自动从 context 获取
//  2. 异步任务：使用 ci.Go(c, func(db){...}) 或 ci.MT(tenantID, model)
//  3. 链式调用：ci.NewAsync(c).Go(func(db){...})
func (m *Model) BeforeQuery(tx *gorm.DB) error {
	// 从上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		tx.Where("tenant_id = ?", tenantID)
	} else {
		return fmt.Errorf("【BeforeQuery】tenant ID not found in context")
	}
	return nil
}

// BeforeDelete 删除前从上下文中获取 TenantID 并添加到删除条件
// 支持三种方式：
//  1. 正常请求：自动从 context 获取
//  2. 异步任务：使用 ci.Go(c, func(db){...}) 或 ci.MT(tenantID, model)
//  3. 链式调用：ci.NewAsync(c).Go(func(db){...})
func (m *Model) BeforeDelete(tx *gorm.DB) error {
	// 从上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		tx.Where("tenant_id = ?", tenantID)
	} else {
		return fmt.Errorf("【BeforeDelete】tenant ID not found in context")
	}
	return nil
}

// BeforeUpdate 更新前从上下文中获取 TenantID 并添加到更新条件
// 支持三种方式：
//  1. 正常请求：自动从 context 获取
//  2. 异步任务：使用 ci.Go(c, func(db){...}) 或 ci.MT(tenantID, model)
//  3. 链式调用：ci.NewAsync(c).Go(func(db){...})
func (m *Model) BeforeUpdate(tx *gorm.DB) error {
	// 从上下文中获取 TenantID
	tenantID, ok := tx.Statement.Context.Value("tenant_id").(string)
	if ok && tenantID != "" {
		tx.Where("tenant_id = ?", tenantID)
	} else {
		return fmt.Errorf("【BeforeUpdate】tenant ID not found in context")
	}
	return nil
}
