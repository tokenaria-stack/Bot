package market

import "testing"

func TestResolveNavigatorPanes_FromMap(t *testing.T) {
	t.Parallel()

	navs := ResolveNavigatorPanes(map[string]NavigatorUISettings{
		"price": {Enabled: true, UseLong: true, LongLen: 60},
	}, NavigatorUISettings{})
	if len(navs) != 1 || !navs["price"].Enabled || navs["price"].LongLen != 60 {
		t.Fatalf("navigators: %+v", navs)
	}
}
