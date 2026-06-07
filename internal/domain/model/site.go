package model

// SiteTest 网站测试数据
type SiteTest struct {
	Name    string // 网站名称
	URL     string // 测试URL
	Icon    string // 显示图标（emoji或符号）
	Delay   int    // 延迟（毫秒），0表示未测试或失败
	Testing bool   // 是否正在测试
	Error   string // 错误信息（如"timeout"）
}

// DefaultSiteTests 返回预设的网站测试列表
func DefaultSiteTests() []SiteTest {
	return []SiteTest{
		{
			Name: "Apple",
			URL:  "http://www.apple.com/library/test/success.html",
			Icon: "\ue711",
		},
		{
			Name: "GitHub",
			URL:  "http://github.com",
			Icon: "\uea84",
		},
		{
			Name: "Google",
			URL:  "http://www.gstatic.com/generate_204",
			Icon: "\uf1a0",
		},
		{
			Name: "Youtube",
			URL:  "http://www.youtube.com/generate_204",
			Icon: "\uf16a",
		},
		{
			Name: "Cloudflare",
			URL:  "http://www.cloudflare.com",
			Icon: "\uf0c2",
		},
		{
			Name: "Baidu",
			URL:  "http://www.baidu.com",
			Icon: "\uf1b0",
		},
		{
			Name: "X",
			URL:  "https://x.com",
			Icon: "\uf099",
		},
		{
			Name: "ChatGPT",
			URL:  "https://api.openai.com",
			Icon: "\uf544",
		},
		{
			Name: "Claude",
			URL:  "https://api.anthropic.com",
			Icon: "\uf544",
		},
		{
			Name: "jsDelivr",
			URL:  "https://cdn.jsdelivr.net",
			Icon: "\uf1b2",
		},
		{
			Name: "npmjs",
			URL:  "https://registry.npmjs.org",
			Icon: "\ue71e",
		},
	}
}
