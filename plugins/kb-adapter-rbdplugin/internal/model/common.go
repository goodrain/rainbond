package model

type Pagination struct {
	Page     int `query:"page" default:"1"`
	PageSize int `query:"page_size" default:"6"`
}

func (p *Pagination) Validate() {
	if p.Page <= 0 {
		p.Page = 1
	}

	if p.PageSize <= 0 {
		p.PageSize = 6
	}

	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

type Search struct {
	Keyword string `query:"keyword"`
}

// PaginatedResult 分页查询结果
type PaginatedResult[T any] struct {
	Items []T `json:"items"` // 当前页数据
	Total int `json:"total"` // 总数据量
}
