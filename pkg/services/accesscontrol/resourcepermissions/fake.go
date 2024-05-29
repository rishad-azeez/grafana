package resourcepermissions

import "github.com/grafana/grafana/pkg/services/accesscontrol"

type FakeActionSetSvc struct {
	ExpectedErr         error
	ExpectedActionSets  []string
	ExpectedActions     []string
	ExpectedPermissions []accesscontrol.Permission
}

func (f *FakeActionSetSvc) ResolveAction(action string) []string {
	return f.ExpectedActionSets
}

func (f *FakeActionSetSvc) ResolveActionSet(actionSet string) []string {
	return f.ExpectedActions
}

func (f *FakeActionSetSvc) ExpandActionSets(permissions []accesscontrol.Permission) []accesscontrol.Permission {
	return f.ExpectedPermissions
}

func (f *FakeActionSetSvc) StoreActionSet(resource, permission string, actions []string) {}
