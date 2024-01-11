package model

// SearchByDomainRequest 根据地址账号密码查询所有的仓库信息
type SearchByDomainRequest struct {
	Domain   string `json:"domain" validate:"domain|required"`
	UserName string `json:"username"`
	Password string `json:"password"`
}
