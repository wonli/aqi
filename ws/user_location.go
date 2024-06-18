package ws

import "time"

type Location struct {
	Latitude  float64  `json:"latitude" validate:"required"`
	Longitude float64  `json:"longitude" validate:"required"`
	AdInfo    []string `json:"adInfo" validate:"required"`
	RegionId  string   `json:"regionId" validate:"required"`
	CityCode  string   `json:"cityCode" validate:"required"`
	Address   string   `json:"address" validate:"required"`

	SelectCityCode string `json:"selectCityCode"` //选择的城市编码
	SelectCityName string `json:"selectCityName"` //选择的城市名称
	LatestUpdateAt time.Time
}
