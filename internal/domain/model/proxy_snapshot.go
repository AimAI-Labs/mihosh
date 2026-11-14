package model

// ProxySnapshot derives topology from one response without further network reads.
// Callers treat its proxy map as read-only for the duration of a projection.
type ProxySnapshot map[string]Proxy

func (proxies ProxySnapshot) Groups() (map[string]Group, []string) {
	groups := make(map[string]Group)
	for name, proxy := range proxies {
		if len(proxy.All) > 0 {
			groups[name] = Group{
				Name:    name,
				Type:    proxy.Type,
				Now:     proxy.Now,
				All:     proxy.All,
				History: proxy.History,
			}
		}
	}

	var orderedNames []string
	if global, ok := proxies["GLOBAL"]; ok {
		for _, name := range global.All {
			if _, isGroup := groups[name]; isGroup {
				orderedNames = append(orderedNames, name)
			}
		}
	}
	if len(orderedNames) != len(groups) {
		existing := make(map[string]bool, len(orderedNames))
		for _, name := range orderedNames {
			existing[name] = true
		}
		for name := range groups {
			if !existing[name] {
				orderedNames = append(orderedNames, name)
			}
		}
	}

	return groups, orderedNames
}
