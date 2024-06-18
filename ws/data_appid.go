package ws

type Appid int

// 系统保留<200
var sys Appid = 0

var (
	//appid范围200~999
	minAppid = 200
	maxAppid = 999

	//code范围100~999
	minCode = 0
	maxCode = 999
	base    = 1000
)
