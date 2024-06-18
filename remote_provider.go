package aqi

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/wonli/aqi/logger"
)

type Provider string

const ProviderConsul Provider = "consul"
const ProviderEtcd Provider = "etcd"

type RemoteProvider struct {
	Name     Provider //服务商名称
	Path     string   //路径
	Endpoint string   //服务器地址
	Type     string   //json, yaml等
}

// ParseRemoteProvider 格式 provider[s]://endpoint/path.type
// 例：consul://localhost:8500/a.yaml
// 表示远程配置中心为consul，服务器地址为http://localhost:8500, path为a, 配置类型是yaml
// scheme加s表示服务器支持ssl
func ParseRemoteProvider(s string) *RemoteProvider {
	u, err := url.Parse(s)
	if err != nil {
		logger.SugarLog.Errorf("Parse remote provider endpoint error: %s", err.Error())
		return nil
	}

	if u.Scheme == "" {
		return nil
	}

	path := strings.TrimLeft(u.Path, "/")
	fileType := filepath.Ext(path)
	if fileType == "" {
		fileType = "yaml"
	} else {
		path = path[:len(path)-len(fileType)]
		fileType = fileType[1:]
	}

	p := &RemoteProvider{
		Path: path,
		Type: fileType,
	}

	switch u.Scheme {
	case "consul", "consuls":
		p.Name = ProviderConsul
	case "etcd", "etcds":
		p.Name = ProviderEtcd
	}

	if strings.HasSuffix(u.Scheme, "s") {
		p.Endpoint = "https://" + u.Host
	} else {
		p.Endpoint = "http://" + u.Host
	}

	return p
}
