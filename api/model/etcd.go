package model

// EtcdCleanReq etcd clean request struct
type EtcdCleanReq struct {
	Keys []string `json:"etcd_keys"`
}
