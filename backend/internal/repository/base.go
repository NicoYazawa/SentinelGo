// Package repository 提供数据访问层（Data Access Layer），封装 GORM 数据库操作。
//
// 架构设计：
//   - 每个实体对应一个 Repository 结构体（如 UserRepository, RoleRepository）
//   - 所有 Repository 通过 Google Wire 在编译期注入到 Service 层
//   - 使用 Go 泛型 BaseRepository[T] 提供通用的 CRUD 基类
//   - 特定业务查询方法在各子 Repository 中扩展
package repository

import (
	"gorm.io/gorm"
)

// Repository 泛型接口定义了标准 CRUD 操作契约。
// 所有实体 Repository 应实现此接口，方便 Mock 测试和依赖注入。
type Repository[T any] interface {
	Create(entity *T) error         // 创建记录
	Update(entity *T) error         // 更新记录（全字段 Save）
	Delete(id uint64) error         // 按主键删除
	FindByID(id uint64) (*T, error) // 按主键查询
	List() ([]T, error)             // 查询全部记录
}

// BaseRepository 泛型 CRUD 基类，提供通用数据库操作实现。
// 子 Repository 通过嵌入此结构体，自动获得完整的 CRUD 能力。
type BaseRepository[T any] struct {
	db *gorm.DB // GORM 数据库实例
}

// NewBaseRepository 创建泛型基类实例。
func NewBaseRepository[T any](db *gorm.DB) *BaseRepository[T] {
	return &BaseRepository[T]{db: db}
}

// Create 插入新记录。
func (r *BaseRepository[T]) Create(entity *T) error {
	return r.db.Create(entity).Error
}

// Update 全字段更新（基于主键）。
// 注意：Save 会更新所有字段，零值字段也会被写入。
func (r *BaseRepository[T]) Update(entity *T) error {
	return r.db.Save(entity).Error
}

// Delete 按主键删除记录（软删除需实体实现 gorm.DeletedAt）。
func (r *BaseRepository[T]) Delete(id uint64) error {
	var entity T
	return r.db.Delete(&entity, id).Error
}

// FindByID 按主键查询单条记录。
func (r *BaseRepository[T]) FindByID(id uint64) (*T, error) {
	var entity T
	err := r.db.First(&entity, id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// List 查询全部记录（慎用，数据量大时应加分页）。
func (r *BaseRepository[T]) List() ([]T, error) {
	var entities []T
	err := r.db.Find(&entities).Error
	return entities, err
}
