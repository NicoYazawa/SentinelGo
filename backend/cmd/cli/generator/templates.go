// Package generator 提供代码生成模板定义。
//
// 模板使用 Go text/template 引擎，通过 {{.FieldName}} 语法注入 TemplateData。
// 由于模板中包含 GORM tag 的反引号，需要使用字符串拼接来构造 tag 字符串。

package generator

import "text/template"

// modelTemplateStr 是 GORM 实体模板。
// 生成包含 ID、CreatedAt、UpdatedAt 基础字段的结构体 + TableName 方法。
const modelTemplateStr = `package model

import "time"

// {{.CamelName}} {{.TableName}} 表实体
type {{.CamelName}} struct {
	ID        uint64    ` + "`" + `gorm:"primarykey" json:"id"` + "`" + `
	CreatedAt time.Time ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time ` + "`" + `json:"updated_at"` + "`" + `
}

func ({{.CamelName}}) TableName() string {
	return "{{.TableName}}"
}
`

// repositoryTemplateStr 是数据访问层模板。
// 生成标准 CRUD 方法：Create, GetByID, List（分页）, Update, Delete。
const repositoryTemplateStr = `package repository

import (
	"context"

	"gorm.io/gorm"
	"sentinelgo/internal/model"
)

type {{.CamelName}}Repository struct {
	db *gorm.DB
}

func New{{.CamelName}}Repository(db *gorm.DB) *{{.CamelName}}Repository {
	return &{{.CamelName}}Repository{db: db}
}

func (r *{{.CamelName}}Repository) Create(ctx context.Context, m *model.{{.CamelName}}) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *{{.CamelName}}Repository) GetByID(ctx context.Context, id uint64) (*model.{{.CamelName}}, error) {
	var m model.{{.CamelName}}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *{{.CamelName}}Repository) List(ctx context.Context, page, pageSize int) ([]model.{{.CamelName}}, int64, error) {
	var list []model.{{.CamelName}}
	var total int64
	q := r.db.WithContext(ctx).Model(&model.{{.CamelName}}{})
	q.Count(&total)
	err := q.Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *{{.CamelName}}Repository) Update(ctx context.Context, m *model.{{.CamelName}}) error {
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *{{.CamelName}}Repository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.{{.CamelName}}{}, id).Error
}
`

// serviceTemplateStr 是业务逻辑层模板。
// 生成透传 Repository 的服务层方法。
const serviceTemplateStr = `package service

import (
	"context"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
)

type {{.CamelName}}Service struct {
	repo *repository.{{.CamelName}}Repository
}

func New{{.CamelName}}Service(repo *repository.{{.CamelName}}Repository) *{{.CamelName}}Service {
	return &{{.CamelName}}Service{repo: repo}
}

func (s *{{.CamelName}}Service) Create(ctx context.Context, m *model.{{.CamelName}}) error {
	return s.repo.Create(ctx, m)
}

func (s *{{.CamelName}}Service) GetByID(ctx context.Context, id uint64) (*model.{{.CamelName}}, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *{{.CamelName}}Service) List(ctx context.Context, page, pageSize int) ([]model.{{.CamelName}}, int64, error) {
	return s.repo.List(ctx, page, pageSize)
}

func (s *{{.CamelName}}Service) Update(ctx context.Context, m *model.{{.CamelName}}) error {
	return s.repo.Update(ctx, m)
}

func (s *{{.CamelName}}Service) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}
`

// handlerTemplateStr 是 HTTP 控制器模板。
// 生成 Gin Handler 方法：Create（POST）、Get（GET /:id）、List（GET）、Update（PUT /:id）、Delete（DELETE /:id）。
const handlerTemplateStr = `package handler

import (
	"strconv"

	"sentinelgo/internal/model"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
)

type {{.CamelName}}Handler struct {
	svc *service.{{.CamelName}}Service
}

func New{{.CamelName}}Handler(svc *service.{{.CamelName}}Service) *{{.CamelName}}Handler {
	return &{{.CamelName}}Handler{svc: svc}
}

// Create POST
func (h *{{.CamelName}}Handler) Create(c *gin.Context) {
	var m model.{{.CamelName}}
	if !response.ShouldBindJSON(c, &m) {
		return
	}
	if err := h.svc.Create(c.Request.Context(), &m); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, m)
}

// Get GET /:id
func (h *{{.CamelName}}Handler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, response.ErrParamInvalid, "ID 无效")
		return
	}
	m, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, m)
}

// List GET
func (h *{{.CamelName}}Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	list, total, err := h.svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{"list": list, "total": total})
}

// Update PUT /:id
func (h *{{.CamelName}}Handler) Update(c *gin.Context) {
	var m model.{{.CamelName}}
	if !response.ShouldBindJSON(c, &m) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, response.ErrParamInvalid, "ID 无效")
		return
	}
	m.ID = id
	if err := h.svc.Update(c.Request.Context(), &m); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, nil)
}

// Delete DELETE /:id
func (h *{{.CamelName}}Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, response.ErrParamInvalid, "ID 无效")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, nil)
}
`

// modelTemplate 返回预编译的 model 模板。
func modelTemplate() *template.Template {
	return template.Must(template.New("model").Parse(modelTemplateStr))
}

// repositoryTemplate 返回预编译的 repository 模板。
func repositoryTemplate() *template.Template {
	return template.Must(template.New("repository").Parse(repositoryTemplateStr))
}

// serviceTemplate 返回预编译的 service 模板。
func serviceTemplate() *template.Template {
	return template.Must(template.New("service").Parse(serviceTemplateStr))
}

// handlerTemplate 返回预编译的 handler 模板。
func handlerTemplate() *template.Template {
	return template.Must(template.New("handler").Parse(handlerTemplateStr))
}

// templateParse 解析模板字符串为 template.Template 对象。
func templateParse(name, tmplStr string) (*template.Template, error) {
	return template.New(name).Parse(tmplStr)
}
