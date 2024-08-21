package ghosttocastopod

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	castopod "git.cmcode.dev/cmcode/go-castopod/pkg/lib"
)

// Determines the behavior of how values are added/changed in the Castopod
// database. By default, accounts in Castopod will either be marked "active"
// or "suspended" if they are not recognized as valid subscribers in Ghost.
type CastopodConfig struct {
	// The castopod database user ID to use for the CreatedBy field. Probably
	// should be 1 - the admin account.
	CreatedBy uint `json:"createdBy"`
	// The castopod database user ID to use for the UpdatedBy field. Probably
	// should be 1 - the admin account.
	UpdatedBy uint `json:"updatedBy"`
	// Connection string for the Castopod mysql database.
	SQLConnectionString string `json:"sqlConnectionString"`
}

type Config struct {
	// Connection string for the Ghost mysql database.
	SQLConnectionString string `json:"sqlConnectionString"`

	// Represents a mapping of plan IDs to Castopod podcast IDs. For example,
	// the plan with ID 66c3f38aedcb1c0101f6ee4d should grant you access to
	// podcast IDs 1,2,4, etc.
	Plans map[string][]uint `json:"plans"`

	// Represents a mapping of emails to Castopod podcast IDs. For example, the
	// account webmaster@example.com should grant you access to podcast IDs
	// 1,2,3,4, etc. These accounts are "blessed" because they will exist in
	// Castopod regardless of their status in Ghost.
	BlessedAccounts map[string][]uint `json:"blessedAccounts"`

	CastopodConfig CastopodConfig `json:"castopodConfig"`
}

const GHOST_MEMBERSHIP_QUERY = `SELECT
  m.email as email,
  mscs.status,
  mscs.plan_id as plan_id
FROM members_stripe_customers as msc
INNER JOIN members_stripe_customers_subscriptions as mscs
INNER JOIN members as m
ON msc.customer_id = mscs.customer_id AND m.id = msc.member_id
`

const CASTOPOD_SUBSCRIPTION_QUERY = "SELECT id, podcast_id, email, token, status, created_by, updated_by, created_at, updated_at FROM cp_subscriptions"

// GhostMembership is a struct built upon [GHOST_MEMBERSHIP_QUERY].
// Currently, none of its fields can be nullable.
type GhostMembership struct {
	Email  string
	Status string
	PlanID string
}

// CastopodSubscription is a struct that (mostly) mirrors the SQL database's
// definition.
type CastopodSubscription struct {
	ID        uint      // auto-increment
	PodcastID uint      // note: composite key is formed with id+podcast_id
	Email     string    // 255 chars max
	Token     string    // note: unique key; 64 characters, 2 uuids without dashes
	Status    string    // can only be active or suspended
	CreatedBy uint      // defined in the user-provided config
	UpdatedBy uint      // defined in the user-provided config
	CreatedAt time.Time // non-null
	UpdatedAt time.Time // non-null

	// This will get set to true if we changed it from its original database
	// state. It is not a part of the database.
	Changed bool
}

func (c *Config) ProcessGhostMembership(m GhostMembership) (GhostMembership, error) {
	if m.Email == "" {
		return m, fmt.Errorf("Email cannot be empty")
	}
	if m.Status == "" {
		return m, fmt.Errorf("Status cannot be empty")
	}
	if m.PlanID == "" {
		return m, fmt.Errorf("PlanID cannot be empty")
	}

	return m, nil
}

func (c *Config) GetGhostMembership(rows *sql.Rows) (GhostMembership, error) {
	var m GhostMembership

	err := rows.Scan(&m.Email, &m.Status, &m.PlanID)
	if err != nil {
		return m, fmt.Errorf("failed to marshal row into interface: %v", err.Error())
	}

	return c.ProcessGhostMembership(m)
}

// ApplyDefaults applies sensible defaults to the config if left unconfigured.
// You shouldn't normally need to execute this, because it's called
// automatically by [LoadConfig].
func (c *Config) ApplyDefaults() {
	// warning: if you change any of these, please update the unit tests!

	if len(c.Plans) == 0 {
		c.Plans = make(map[string][]uint)
	}

	if len(c.BlessedAccounts) == 0 {
		c.BlessedAccounts = make(map[string][]uint)
	}

	if c.CastopodConfig.CreatedBy == 0 {
		c.CastopodConfig.CreatedBy = 1
	}

	if c.CastopodConfig.UpdatedBy == 0 {
		c.CastopodConfig.UpdatedBy = 1
	}

	for i := range c.Plans {
		slices.Sort(c.Plans[i])
	}

	for k := range c.BlessedAccounts {
		slices.Sort(c.BlessedAccounts[k])
	}
}

// LoadConfig reads from file f and applies sensible defaults to values not
// specifically set by the user.
func LoadConfig(f string) (Config, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return Config{}, fmt.Errorf("failed to load config from %v: %v", f, err)
	}

	var c Config
	err = json.Unmarshal(b, &c)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config from %v: %v", f, err)
	}

	c.ApplyDefaults()

	return c, nil
}

const (
	CastopodStatusSuspended = "suspended"
	CastopodStatusActive    = "active"
	GhostStatusActive       = "active"
)

// GetCastopodSubscriptions accepts a list of all Ghost memberships and all
// current Castopod subscriptions, and returns the final list.
func (c *Config) GetCastopodSubscriptions(gms []GhostMembership, cms []CastopodSubscription) []CastopodSubscription {
	// This needs an interesting data structure. There has to be a
	// one-to-many mapping between each Ghost membership and Castopod
	// subscriptions. This is because the user can configure multiple podcast
	// subscriptions for a single Ghost membership.
	//
	// Castopod subscription entries in the DB should never be deleted, only
	// upserted. This means that the return value of this function will be a
	// slice whose length never shrinks over time.
	//
	// example:
	//
	// plan IDs:
	//
	// foo = 1,2,
	// bar = 3,4
	//
	// members:
	//
	// abc@example.com = foo,bar
	// def@example.com = foo
	//
	// castopod subscriptions:
	//
	// abc@example.com = []foo + []bar = 1,2,3,4
	// def@example.com = []foo = 1,2

	// define a mapping between emails and the granted plan ID's
	emails := make(map[string]map[uint]CastopodSubscription)

	// start by iterating through the existing castopod subscriptions. This data
	// structure allows us to quickly identify the subscriptions that already
	// exist for each email address.
	for _, s := range cms {
		if s.Email == "" {
			continue
		}

		_, ok := emails[s.Email]
		if !ok {
			emails[s.Email] = make(map[uint]CastopodSubscription)
		}

		emails[s.Email][s.PodcastID] = s
	}

	// now that we have a list of all the email addresses in castopod and their
	// corresponding subscriptions, we can iterate through the ghost membership
	// listings and determine which gaps need to be filled.
	for _, gm := range gms {
		if gm.Email == "" {
			continue
		}

		// iterate through all of the user-configured plan IDs, and those plans'
		// corresponding podcast IDs
		for _, p := range c.Plans[gm.PlanID] {
			_, ok := emails[gm.Email]
			if !ok {
				emails[gm.Email] = make(map[uint]CastopodSubscription)
			}

			s, ok := emails[gm.Email][p]
			if !ok {
				s = CastopodSubscription{
					PodcastID: p,
					Email:     gm.Email,
					Token:     castopod.NewToken(),
					CreatedBy: c.CastopodConfig.CreatedBy,
					CreatedAt: time.Now(),
					Changed:   true,
				}
			}

			newStatus := ""
			if gm.Status == GhostStatusActive {
				newStatus = CastopodStatusActive
			} else {
				newStatus = CastopodStatusSuspended
			}

			// only make a change if we need to, otherwise the database
			// will auto-increment out of control on each update
			if newStatus != s.Status {
				s.Status = newStatus
				s.UpdatedAt = time.Now()
				s.UpdatedBy = c.CastopodConfig.UpdatedBy
				s.Changed = true
			} else {
				s.Changed = false
			}

			emails[gm.Email][p] = s
		}
	}

	// introduce the blessed accounts
	for email, ids := range c.BlessedAccounts {
		if email == "" {
			continue
		}

		// for each podcast ID this blessed account has been granted access,
		// grant it active status. A blessed account doesn't automatically get
		// access to *every* podcast. It only gets access to the podcast IDs that
		// have been configured by the user's config.
		for _, p := range ids {
			_, ok := emails[email]
			if !ok {
				emails[email] = make(map[uint]CastopodSubscription)
			}

			s, ok := emails[email][p]
			if !ok {
				s = CastopodSubscription{
					PodcastID: p,
					Email:     email,
					Token:     castopod.NewToken(),
					CreatedBy: c.CastopodConfig.CreatedBy,
					CreatedAt: time.Now(),
					Changed:   true,
				}
			}

			// only make a change if we need to, otherwise the database
			// will auto-increment out of control on each update
			if s.Status != CastopodStatusActive {
				s.Changed = true
				s.Status = CastopodStatusActive
				s.UpdatedAt = time.Now()
				s.UpdatedBy = c.CastopodConfig.UpdatedBy
			}

			emails[email][p] = s
		}
	}

	// finally, flatten the map so we can produce a list of castopod
	// subscriptions
	cs := []CastopodSubscription{}
	for _, sub := range emails {
		for i := range sub {
			cs = append(cs, sub[i])
		}
	}

	return cs
}
