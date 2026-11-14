package model

// IPInfo IP地理位置信息
type IPInfo struct {
	// 基础信息 (兼容多平台)
	IP             string `json:"ip"`    // ip.sb
	Query          string `json:"query"` // ip-api.com
	Status         string `json:"status"`
	Country        string `json:"country"`
	CountryCode    string `json:"country_code"` // ip.sb
	CountryCodeApi string `json:"countryCode"`  // ip-api.com
	RegionName     string `json:"regionName"`
	City           string `json:"city"`

	// 网络资产信息 (ip.sb 使用 ASN int, ip-api 使用 AS string)
	ASN             int    `json:"asn"`              // ip.sb
	ASNOrganization string `json:"asn_organization"` // ip.sb
	AS              string `json:"as"`               // ip-api.com
	Organization    string `json:"organization"`     // ip.sb
	Org             string `json:"org"`              // ip-api.com
	ISP             string `json:"isp"`

	// 地理坐标与其它
	Latitude      float64 `json:"latitude"`  // ip.sb
	Lat           float64 `json:"lat"`       // ip-api.com
	Longitude     float64 `json:"longitude"` // ip.sb
	Lon           float64 `json:"lon"`       // ip-api.com
	Timezone      string  `json:"timezone"`
	ContinentCode string  `json:"continent_code"`
	Offset        int     `json:"offset"`
}
