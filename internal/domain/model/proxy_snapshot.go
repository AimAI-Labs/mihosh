package model

import "sort"

// ProxySnapshot derives topology from one response without further network reads.
// Callers treat its proxy map as read-only for the duration of a projection.
type ProxySnapshot map[string]Proxy

// Chain follows current selections to a leaf; incomplete and cyclic chains have no leaf.
func (proxies ProxySnapshot) Chain(root string) ([]string, bool) {
	var chain []string
	visited := make(map[string]struct{})
	for current := root; ; {
		if _, seen := visited[current]; seen {
			return nil, false
		}
		visited[current] = struct{}{}
		proxy, ok := proxies[current]
		if !ok {
			return nil, false
		}
		chain = append(chain, current)
		if proxy.Now == "" {
			if len(proxy.All) > 0 {
				return nil, false
			}
			return chain, true
		}
		current = proxy.Now
	}
}

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
		extra := make([]string, 0, len(groups)-len(orderedNames))
		for name := range groups {
			if !existing[name] {
				extra = append(extra, name)
			}
		}
		sort.Strings(extra)
		orderedNames = append(orderedNames, extra...)
	}

	return groups, orderedNames
}
