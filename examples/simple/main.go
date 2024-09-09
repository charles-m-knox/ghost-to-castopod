package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	g2c "github.com/charles-m-knox/ghost-to-castopod/pkg/lib"
	_ "github.com/go-sql-driver/mysql"
)

var (
	flagConfig  string
	flagTest    bool
	flagOutFile string
)

func parseFlags() {
	flag.StringVar(&flagConfig, "f", "config.json", "json file to use for loading configuration")
	flag.BoolVar(&flagTest, "test", false, "connect read-only and perform a dry run")
	flag.StringVar(&flagOutFile, "o", "", "a file to write the database query to (can combine with -test to allow manual editing of the query)")
	flag.Parse()
}

func getDB(constr string, readonly bool) *sql.DB {
	db, err := sql.Open("mysql", constr)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err.Error())
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	if readonly {
		_, err = db.Exec("SET SESSION TRANSACTION READ ONLY")
		if err != nil {
			log.Fatalf("failed to set read-only session: %v", err.Error())
		}
	}

	return db
}

func main() {
	parseFlags()

	c, err := g2c.LoadConfig(flagConfig)
	if err != nil {
		log.Fatalf("failed to load config: %v", err.Error())
	}

	ghost := getDB(c.SQLConnectionString, true)
	castopod := getDB(c.CastopodConfig.SQLConnectionString, true)

	var castopodWrite *sql.DB
	if !flagTest {
		castopodWrite = getDB(c.CastopodConfig.SQLConnectionString, false)
	}

	rows, err := ghost.Query(g2c.GHOST_MEMBERSHIP_QUERY)
	if err != nil {
		log.Fatalf("failed to query posts from db: %v", err.Error())
	}

	defer rows.Close()

	gms := []g2c.GhostMembership{}

	for rows.Next() {
		membership, err := c.GetGhostMembership(rows)
		if err != nil {
			log.Fatalf("failed to get ghost post from row: %v", err.Error())
		}

		gms = append(gms, membership)

		log.Println(membership)
	}

	cs := []g2c.CastopodSubscription{}

	{
		rows, err := castopod.Query(g2c.CASTOPOD_SUBSCRIPTION_QUERY)
		if err != nil {
			log.Fatalf("failed to query posts from db: %v", err.Error())
		}

		for rows.Next() {
			var sub g2c.CastopodSubscription
			var createdAt, updatedAt sql.NullString
			err := rows.Scan(&sub.ID, &sub.PodcastID, &sub.Email, &sub.Token, &sub.Status, &sub.CreatedBy, &sub.UpdatedBy, &createdAt, &updatedAt)
			if err != nil {
				log.Fatalf("failed to scan row: %v", err.Error())
			}

			if createdAt.Valid {
				sub.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt.String)
				if err != nil {
					log.Fatalf("failed to parse CreatedAt datetime: %v", err.Error())
				}
			}

			if updatedAt.Valid {
				sub.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAt.String)
				if err != nil {
					log.Fatalf("failed to parse UpdatedAt datetime: %v", err.Error())
				}
			}

			cs = append(cs, sub)

			log.Println(sub)
		}
	}

	results := c.GetCastopodSubscriptions(gms, cs)
	var q strings.Builder

	if len(results) == 0 {
		log.Println("There were no results to update. Exiting.")
		return
	}

	q.WriteString("INSERT INTO cp_subscriptions (podcast_id, email, token, status, created_by, updated_by, created_at, updated_at) VALUES \n")

	lr := len(results) - 1

	changed := false

	for i, r := range results {
		if !r.Changed {
			continue
		}
		changed = true

		finalComma := ","
		if i == lr {
			finalComma = ""
		}

		q.WriteString(fmt.Sprintf("(%v, '%v', '%v', '%v', %v, %v, '%v', '%v')%v \n", r.PodcastID, r.Email, r.Token, r.Status, r.CreatedBy, r.UpdatedBy, r.CreatedAt.Format("2006-01-02 15:04:05"), r.UpdatedAt.Format("2006-01-02 15:04:05"), finalComma))
	}

	if !changed {
		log.Println("done processing; no changes are needed since the last run. exiting.")
		return
	}

	q.WriteString("ON DUPLICATE KEY UPDATE podcast_id = VALUES(podcast_id), email = VALUES(email), token = VALUES(token), status = VALUES(status), created_by = VALUES(created_by), updated_by = VALUES(updated_by), created_at = VALUES(created_at), updated_at = VALUES(updated_at);")

	qq := q.String()
	log.Printf("query: %v", qq)

	if flagOutFile != "" {
		err = os.WriteFile("out.txt", []byte(qq), 0o640)
		if err != nil {
			log.Fatalf("failed to write query to out.txt: %v", err.Error())
		}
	}

	if flagTest {
		log.Println("test mode enabled, exiting.")
		return
	}

	_, err = castopodWrite.Query(qq) // doesn't return any rows
	if err != nil {
		log.Fatalf("failed to write to castopod db: %v", err.Error())
	}

	log.Println("done writing to the castopod database.")
	fmt.Println("")
	fmt.Println("Note: If you're running redis, please run:")
	fmt.Println("")
	fmt.Println("redis-cli -a password_goes_here FLUSHALL")
	fmt.Println("")
	fmt.Println("This prevents Castopod from staying out of sync with the database.")
}
