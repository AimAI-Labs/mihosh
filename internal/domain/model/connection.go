package model

import "time"

// Connection 连接信息
type Connection struct {
	ID            string    `json:"id"`
	Metadata      Metadata  `json:"metadata"`
	Upload        int64     `json:"upload"`
	Download      int64     `json:"download"`
	Start         string    `json:"start"`
	Chains        []string  `json:"chains"`
	Rule          string    `json:"rule"`
	RulePayload   string    `json:"rulePayload"`
	DownloadSpeed int64     `json:"downloadSpeed"`
	UploadSpeed   int64     `json:"uploadSpeed"`
	ClosedAt      time.Time `json:"-"`
}

// Metadata 连接元数据
type Metadata struct {
	Network           string      `json:"network"`
	Type              string      `json:"type"`
	SourceIP          string      `json:"sourceIP"`
	DestinationIP     string      `json:"destinationIP"`
	SourceGeoIP       interface{} `json:"sourceGeoIP"`
	DestinationGeoIP  interface{} `json:"destinationGeoIP"`
	SourceIPASN       string      `json:"sourceIPASN"`
	DestinationIPASN  string      `json:"destinationIPASN"`
	SourcePort        string      `json:"sourcePort"`
	DestinationPort   string      `json:"destinationPort"`
	InboundIP         string      `json:"inboundIP"`
	InboundPort       string      `json:"inboundPort"`
	InboundName       string      `json:"inboundName"`
	InboundUser       string      `json:"inboundUser"`
	Host              string      `json:"host"`
	DNSMode           string      `json:"dnsMode"`
	UID               int         `json:"uid"`
	Process           string      `json:"process"`
	ProcessPath       string      `json:"processPath"`
	SpecialProxy      string      `json:"specialProxy"`
	SpecialRules      string      `json:"specialRules"`
	RemoteDestination string      `json:"remoteDestination"`
	DSCP              int         `json:"dscp"`
	SniffHost         string      `json:"sniffHost"`
}

// ConnectionsResponse 连接列表响应
type ConnectionsResponse struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
}

// Traffic 流量信息
type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// MemoryResponse 内存信息响应
type MemoryResponse struct {
	Inuse   int64 `json:"inuse"`   // 当前使用的内存（字节）
	OSLimit int64 `json:"oslimit"` // 系统限制
}
