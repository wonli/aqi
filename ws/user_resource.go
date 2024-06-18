package ws

import "gorm.io/datatypes"

type Resource struct {
	Path     string         `json:"path,omitempty"`     // 文件路径
	CdnUrl   string         `json:"cdnUrl,omitempty"`   // cdn地址
	Scenes   string         `json:"scenes"`             // 使用场景
	MimeType string         `json:"mimeType,omitempty"` // 文件类型
	ExtData  datatypes.JSON `json:"extData,omitempty"`
}
