package ghosttocastopod_test

import (
	"testing"

	ghosttocastopod "github.com/charles-m-knox/ghost-to-castopod/pkg/lib"
)

const (
	// Shorthand for ghosttocastopod.GhostStatusActive
	gActive = ghosttocastopod.GhostStatusActive
	// Shorthand for ghosttocastopod.CastopodStatusActive
	cActive = ghosttocastopod.CastopodStatusActive
	// Shorthand for ghosttocastopod.CastopodStatusSuspended
	cSusp = ghosttocastopod.CastopodStatusSuspended
)

func TestProcessGhostMembership(t *testing.T) {
	t.Parallel()

	tc := ghosttocastopod.Config{}

	tests := []struct {
		c   ghosttocastopod.Config
		gm  ghosttocastopod.GhostMembership
		err bool
	}{
		{tc, ghosttocastopod.GhostMembership{Email: ""}, true},
		{tc, ghosttocastopod.GhostMembership{Email: "baz", Status: ""}, true},
		{tc, ghosttocastopod.GhostMembership{Email: "baz", Status: "abc", PlanID: ""}, true},
		{tc, ghosttocastopod.GhostMembership{Email: "baz", Status: "abc", PlanID: "def"}, false},
	}

	for i, test := range tests {
		_, err := test.c.ProcessGhostMembership(test.gm)
		if err != nil && !test.err {
			t.Logf("test %v failed: received unexpected err: %v", i, err.Error())
			t.Fail()
		} else if err == nil && test.err {
			t.Logf("test %v failed: did not receive error but wanted one", i)
			t.Fail()
		}
	}
}

func TestApplyDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		c    ghosttocastopod.Config
		want ghosttocastopod.Config
	}{
		{ghosttocastopod.Config{}, ghosttocastopod.Config{CastopodConfig: ghosttocastopod.CastopodConfig{CreatedBy: 1, UpdatedBy: 1}}},
		{ghosttocastopod.Config{CastopodConfig: ghosttocastopod.CastopodConfig{CreatedBy: 21, UpdatedBy: 21}}, ghosttocastopod.Config{CastopodConfig: ghosttocastopod.CastopodConfig{CreatedBy: 21, UpdatedBy: 21}}},
		{ghosttocastopod.Config{Plans: map[string][]uint{"foo": {2, 3, 1}}}, ghosttocastopod.Config{Plans: map[string][]uint{"foo": {1, 2, 3}}, CastopodConfig: ghosttocastopod.CastopodConfig{CreatedBy: 1, UpdatedBy: 1}}},
		{ghosttocastopod.Config{BlessedAccounts: map[string][]uint{"bar": {2, 3, 1}}}, ghosttocastopod.Config{BlessedAccounts: map[string][]uint{"bar": {1, 2, 3}}, CastopodConfig: ghosttocastopod.CastopodConfig{CreatedBy: 1, UpdatedBy: 1}}},
	}

	for i, test := range tests {
		test.c.ApplyDefaults()

		if test.c.CastopodConfig.CreatedBy != test.want.CastopodConfig.CreatedBy {
			t.Logf("test %v failed: CastopodConfig.CreatedBy mismatch, got %v, want %v", i, test.c.CastopodConfig.CreatedBy, test.want.CastopodConfig.CreatedBy)
			t.Fail()
		}

		if test.c.CastopodConfig.UpdatedBy != test.want.CastopodConfig.UpdatedBy {
			t.Logf("test %v failed: CastopodConfig.UpdatedBy mismatch, got %v, want %v", i, test.c.CastopodConfig.UpdatedBy, test.want.CastopodConfig.UpdatedBy)
			t.Fail()
		}

		for k, v := range test.c.Plans {
			l := len(v)
			wl := len(test.want.Plans[k])
			if l != wl {
				t.Logf("test %v failed: configured plans length mismatch, got %v, want %v (got len=%v, want len=%v)", i, test.c.Plans, test.want.Plans, l, wl)
				t.FailNow()
			}
			for i := range v {
				if v[i] != test.want.Plans[k][i] {
					t.Logf("test %v failed: configured plans value mismatch, got %v, want %v", i, v[i], test.want.Plans[k][i])
					t.Fail()
				}
			}
		}
	}
}

func TestGetCastopodSubscriptions(t *testing.T) {
	t.Parallel()

	const createdBy = 20
	const updatedBy = 21

	const admin1 = "foo@example.com"
	const admin2 = "bar@example.com"
	const email1 = "baz@example.com"
	const email2 = "abc@example.com"
	const email3 = "def@example.com"
	const plan1 = "foo"
	const plan2 = "bar"
	const plan3 = "baz"

	const token1 = "ffbbd29ddf9046a7912320864d1dfcd79d76e50d69ae485dbad895299d11b040"
	const token2 = "4412af7606b74e26ad5dff2ea05ba460b3df02401b98499c94c73f0042617a89"
	const token3 = "664ac581259146888ce85fec8bdfc1d3bbb0a425e1874f4eac2ea929cb3883cd"
	// token4 := ghosttocastopod.NewUUID()
	// token5 := ghosttocastopod.NewUUID()
	// token6 := ghosttocastopod.NewUUID()

	tc := ghosttocastopod.Config{
		Plans: map[string][]uint{
			plan1: {1, 2, 3, 4},
			plan2: {5, 6},
			plan3: {7},
		},
		BlessedAccounts: map[string][]uint{
			admin1: {1, 2, 3, 4, 5, 6},
			admin2: {1, 2, 3, 4, 5},
			"":     {}, // empty test case will get ignored
		},
		CastopodConfig: ghosttocastopod.CastopodConfig{
			CreatedBy: createdBy,
			UpdatedBy: updatedBy,
		},
	}

	tgm := []ghosttocastopod.GhostMembership{
		// first user
		{Email: email1, Status: gActive, PlanID: plan1},
		{Email: email1, Status: gActive, PlanID: plan2},
		// second user
		{Email: email2, Status: gActive, PlanID: plan2},
		// third user
		{Email: email3, PlanID: plan1},
		{Email: email3, Status: gActive, PlanID: plan2},
		{Email: email3, PlanID: plan3},

		{}, // empty test case will get ignored
	}

	tcs := []ghosttocastopod.CastopodSubscription{
		{Email: email1, Token: token1, PodcastID: 1, Status: cActive},
		{Email: email1, Token: token2, PodcastID: 2, Status: cActive},
		{Email: email1, Token: token3, PodcastID: 3, Status: cSusp},

		{}, // empty test case will get ignored
	}

	// note: leaving the token empty will trigger the test to expect a non-empty
	// value. The test needs to test the preservation of existing tokens as well
	// as the generation of new ones.
	tw := []ghosttocastopod.CastopodSubscription{
		{Email: email1, Token: token1, PodcastID: 1, Status: cActive, Changed: false},
		{Email: email1, Token: token2, PodcastID: 2, Status: cActive, Changed: false},
		{Email: email1, Token: token3, PodcastID: 3, Status: cActive, Changed: true},
		{Email: email1, Token: "", PodcastID: 4, Status: cActive, Changed: true},
		{Email: email1, Token: "", PodcastID: 5, Status: cActive, Changed: true},
		{Email: email1, Token: "", PodcastID: 6, Status: cActive, Changed: true},

		// second user
		{Email: email2, Token: "", PodcastID: 5, Status: cActive, Changed: true},
		{Email: email2, Token: "", PodcastID: 6, Status: cActive, Changed: true},

		// third user
		{Email: email3, Token: "", PodcastID: 1, Status: cSusp, Changed: true},
		{Email: email3, Token: "", PodcastID: 2, Status: cSusp, Changed: true},
		{Email: email3, Token: "", PodcastID: 3, Status: cSusp, Changed: true},
		{Email: email3, Token: "", PodcastID: 4, Status: cSusp, Changed: true},
		{Email: email3, Token: "", PodcastID: 5, Status: cActive, Changed: true},
		{Email: email3, Token: "", PodcastID: 6, Status: cActive, Changed: true},
		{Email: email3, Token: "", PodcastID: 7, Status: cSusp, Changed: true},

		// blessed accounts
		{Email: admin1, Token: "", PodcastID: 1, Status: cActive, Changed: true},
		{Email: admin1, Token: "", PodcastID: 2, Status: cActive, Changed: true},
		{Email: admin1, Token: "", PodcastID: 3, Status: cActive, Changed: true},
		{Email: admin1, Token: "", PodcastID: 4, Status: cActive, Changed: true},
		{Email: admin1, Token: "", PodcastID: 5, Status: cActive, Changed: true},
		{Email: admin1, Token: "", PodcastID: 6, Status: cActive, Changed: true},
		// {Email: admin1, Token: "", PodcastID: 7, Status: cActive, Changed: true}, // admin1 hasn't been blessed with access to podcast 7!
		{Email: admin2, Token: "", PodcastID: 1, Status: cActive, Changed: true},
		{Email: admin2, Token: "", PodcastID: 2, Status: cActive, Changed: true},
		{Email: admin2, Token: "", PodcastID: 3, Status: cActive, Changed: true},
		{Email: admin2, Token: "", PodcastID: 4, Status: cActive, Changed: true},
		{Email: admin2, Token: "", PodcastID: 5, Status: cActive, Changed: true},
		// {Email: admin2, Token: "", PodcastID: 6, Status: cActive, Changed: true}, // admin2 hasn't been blessed with access to podcast 6!
		// {Email: admin2, Token: "", PodcastID: 7, Status: cActive, Changed: true}, // admin2 hasn't been blessed with access to podcast 7!
	}

	tests := []struct {
		c    ghosttocastopod.Config
		gm   []ghosttocastopod.GhostMembership
		cs   []ghosttocastopod.CastopodSubscription
		want []ghosttocastopod.CastopodSubscription
	}{
		{tc, tgm, tcs, tw},
	}

	for i, test := range tests {
		got := test.c.GetCastopodSubscriptions(test.gm, test.cs)

		lg := len(got)
		lw := len(test.want)

		if lg != lw {
			t.Logf("test %v failed: result length mismatch, got %v, want %v", i, lg, lw)
			t.Fail()
		}

		successes := 0

		// identifies any missing entries betwen "got" and "test.want"
		found := make(map[int]bool)

		for j, g := range got {
			for k, w := range test.want {
				if g.Email != w.Email || g.PodcastID != w.PodcastID {
					continue
				}

				found[k] = true

				failed := false

				if g.Changed != w.Changed {
					t.Logf("test %v failed: Changed mismatch, got %v, want %v (j=%v, k=%v)", i, g.Changed, w.Changed, j, k)
					t.Fail()
					failed = true
				}

				if g.Status != w.Status {
					t.Logf("test %v failed: Status mismatch, got %v, want %v (j=%v, k=%v)", i, g.Status, w.Status, j, k)
					t.Fail()
					failed = true
				}

				if g.Token != w.Token && w.Token != "" {
					t.Logf("test %v failed: Token mismatch, got %v, want %v (j=%v, k=%v)", i, g.Token, w.Token, j, k)
					t.Fail()
					failed = true
				}

				if len(g.Token) != 64 {
					t.Logf("test %v failed: Token mismatch, got %v, want len %v (j=%v, k=%v)", i, g.Token, 64, j, k)
					t.Fail()
					failed = true
				}

				// only enforce UpdatedBy if the membership was changed
				if g.UpdatedBy != test.c.CastopodConfig.UpdatedBy && g.Changed {
					t.Logf("test %v failed: UpdatedBy mismatch, got %v, want %v (j=%v, k=%v)", i, g.UpdatedBy, test.c.CastopodConfig.UpdatedBy, j, k)
					t.Fail()
					failed = true
				}

				if failed {
					t.Logf("got: %v\n\nwant: %v", g, w)
				} else {
					successes++
				}
			}
		}

		if successes != lw {
			t.Logf("test %v failed: evaluated number of successes mismatched, got %v, want %v", i, successes, lw)
			t.Fail()
		}

		for j := range test.want {
			_, ok := found[j]
			if ok {
				continue
			}

			t.Logf("test %v failed: a wanted entry was missing: %v", i, test.want[j])
			t.Fail()
		}

	}
}
