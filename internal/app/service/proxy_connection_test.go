package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/domain/port"
)

type fakeProxyAPI struct {
	groups       map[string]model.Group
	orderedNames []string
	proxies      map[string]model.Proxy
	selectedG    string
	selectedP    string
	delayName    string
	delayURL     string
	delayTimeout int
	delayResult  int
	groupTested  string
	err          error
}

var _ port.ProxyAPI = (*fakeProxyAPI)(nil)

func (f *fakeProxyAPI) GetGroups() (map[string]model.Group, []string, error) {
	return f.groups, f.orderedNames, f.err
}
func (f *fakeProxyAPI) GetProxies() (map[string]model.Proxy, error) { return f.proxies, f.err }
func (f *fakeProxyAPI) GetProxy(name string) (*model.Proxy, error) {
	p, ok := f.proxies[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return &p, f.err
}
func (f *fakeProxyAPI) SelectProxy(group, proxy string) error {
	f.selectedG, f.selectedP = group, proxy
	return f.err
}
func (f *fakeProxyAPI) TestProxyDelay(name, testURL string, timeout int) (int, error) {
	f.delayName, f.delayURL, f.delayTimeout = name, testURL, timeout
	return f.delayResult, f.err
}
func (f *fakeProxyAPI) TestGroupDelay(group, testURL string, timeout int) error {
	f.groupTested, f.delayURL, f.delayTimeout = group, testURL, timeout
	return f.err
}
func (f *fakeProxyAPI) NewHTTPClientWithProxy(string) (*http.Client, error) {
	return &http.Client{}, f.err
}

type fakeConnectionAPI struct {
	resp      *model.ConnectionsResponse
	closedID  string
	closedAll bool
	err       error
}

var _ port.ConnectionAPI = (*fakeConnectionAPI)(nil)

func (f *fakeConnectionAPI) GetConnections() (*model.ConnectionsResponse, error) {
	return f.resp, f.err
}
func (f *fakeConnectionAPI) CloseConnection(id string) error {
	f.closedID = id
	return f.err
}
func (f *fakeConnectionAPI) CloseAllConnections() error {
	f.closedAll = true
	return f.err
}

func TestProxyServiceUsesPortFake(t *testing.T) {
	fake := &fakeProxyAPI{
		groups:       map[string]model.Group{"Proxy": {Name: "Proxy", Now: "HK-1"}},
		orderedNames: []string{"Proxy"},
		proxies:      map[string]model.Proxy{"HK-1": {Name: "HK-1"}},
		delayResult:  42,
	}
	svc := NewProxyService(fake, "https://example.com/generate_204", 2500)

	groups, order, err := svc.GetGroups()
	if err != nil || len(groups) != 1 || len(order) != 1 || order[0] != "Proxy" {
		t.Fatalf("GetGroups() = (%v, %v, %v)", groups, order, err)
	}
	if err := svc.SelectProxy("Proxy", "HK-1"); err != nil || fake.selectedG != "Proxy" || fake.selectedP != "HK-1" {
		t.Fatalf("SelectProxy failed: %v (%s -> %s)", err, fake.selectedG, fake.selectedP)
	}
	delay, err := svc.TestProxyDelay("HK-1")
	if err != nil || delay != 42 || fake.delayURL != "https://example.com/generate_204" || fake.delayTimeout != 2500 {
		t.Fatalf("TestProxyDelay() = (%d, %v), url=%s timeout=%d", delay, err, fake.delayURL, fake.delayTimeout)
	}
	if err := svc.TestGroupDelay("Proxy"); err != nil || fake.groupTested != "Proxy" {
		t.Fatalf("TestGroupDelay() = %v, group=%s", err, fake.groupTested)
	}
}

func TestConnectionServiceUsesPortFake(t *testing.T) {
	fake := &fakeConnectionAPI{
		resp: &model.ConnectionsResponse{
			Connections: []model.Connection{{ID: "conn-1"}},
		},
	}
	svc := NewConnectionService(fake)

	resp, err := svc.GetConnections()
	if err != nil || resp == nil || len(resp.Connections) != 1 {
		t.Fatalf("GetConnections() = (%v, %v)", resp, err)
	}
	if err := svc.CloseConnection("conn-1"); err != nil || fake.closedID != "conn-1" {
		t.Fatalf("CloseConnection() = %v, closedID=%q", err, fake.closedID)
	}
	if err := svc.CloseAllConnections(); err != nil || !fake.closedAll {
		t.Fatalf("CloseAllConnections() = %v, closedAll=%v", err, fake.closedAll)
	}
}
