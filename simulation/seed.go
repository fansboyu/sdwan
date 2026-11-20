package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultEmail    = "lab-admin@sdwan.test"
	defaultPassword = "LabNetwork2026!"
	defaultToken    = "sdwan_admin_docker_lab_only"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "http" {
		http.Handle("/", http.FileServer(http.Dir("/srv")))
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
	ctx := context.Background()
	databaseURL := env("DATABASE_URL", "postgres://sdwan:sdwan@postgres:5432/sdwan?sslmode=disable")
	var db *pgx.Conn
	var err error
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		db, err = pgx.Connect(ctx, databaseURL)
		if err == nil {
			var version int
			err = db.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE dirty=false ORDER BY version DESC LIMIT 1`).Scan(&version)
			if err == nil && version >= 10 {
				break
			}
			_ = db.Close(ctx)
			db = nil
		}
		time.Sleep(2 * time.Second)
	}
	if db == nil || err != nil {
		log.Fatalf("controller database migrations did not become ready: %v", err)
	}
	defer db.Close(ctx)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(env("LAB_ADMIN_PASSWORD", defaultPassword)), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	adminToken := env("LAB_ADMIN_TOKEN", defaultToken)
	tokenDigest := sha256.Sum256([]byte(adminToken))
	userID := "usr_sdwan_docker_lab"
	_, err = db.Exec(ctx, `INSERT INTO users (id,email,password_hash,plan_code,overlay_cidr,max_devices,status,path_mode)
VALUES ($1,$2,$3,'relay','100.64.240.0/24',254,'active','direct')
ON CONFLICT (id) DO UPDATE SET email=EXCLUDED.email,password_hash=EXCLUDED.password_hash,
 plan_code=EXCLUDED.plan_code,overlay_cidr=EXCLUDED.overlay_cidr,max_devices=EXCLUDED.max_devices,status='active'`,
		userID, env("LAB_ADMIN_EMAIL", defaultEmail), string(passwordHash))
	if err != nil {
		log.Fatalf("seed lab account: %v", err)
	}
	_, err = db.Exec(ctx, `INSERT INTO admin_sessions (id,user_id,token_hash,expires_at)
VALUES ('ases_sdwan_docker_lab',$1,$2,now()+interval '10 years')
ON CONFLICT (id) DO UPDATE SET user_id=EXCLUDED.user_id,token_hash=EXCLUDED.token_hash,expires_at=EXCLUDED.expires_at`,
		userID, hex.EncodeToString(tokenDigest[:]))
	if err != nil {
		log.Fatalf("seed lab admin session: %v", err)
	}
	fmt.Printf("Docker lab account ready: %s\n", env("LAB_ADMIN_EMAIL", defaultEmail))
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
